# 部署文件中的hostPID字段

参考文章

1. [Pods are not starting. NetworkPlugin cni failed to set up pod](https://stackoverflow.com/questions/44305615/pods-are-not-starting-networkplugin-cni-failed-to-set-up-pod)
    - `kubeadm init --pod-network-cidr=10.244.0.0/16`指定`cidr`
2. ["Error deleting network: failed to Statfs" during pod shutdown](https://github.com/kubernetes/kubernetes/issues/57465)
3. [Add more precise check for netns "no such file or directory" error](https://github.com/kubernetes/kubernetes/pull/84157)
4. ["Error adding network: failed to Statfs" during pod startup](https://github.com/kubernetes/kubernetes/issues/72044)
5. [can't access /proc/[pid]/exe in docker build](https://github.com/moby/moby/issues/7147)
6. [Seccomp security profiles for Docker](https://docs.docker.com/engine/security/seccomp/)
    - `setns`需要`SYS_ADMIN`能力.
7. [京东数科容器实践之contiv支持非docker容器运行时](https://www.kubernetes.org.cn/5345.html)

本来之前作为与bridge, dhcp平级的cni插件时完全没问题, 但是作为Pod部署时就无法生效了. Pod能部署, 但是新建其他Pod时无法通过此插件正确获取到IP, Pod一直处于`ContainerCreating`状态. 查看`/var/log/message`会发现有如下报错.

```
Feb 13 15:11:58 k8s-worker-01 kubelet[111977]: 2020/02/13 15:11:58 error calling DHCP.Allocate: failed to Statfs "/proc/81648/ns/net": no such file or directory.
```

我在`plugin`和`cni`两个工程中追踪了很久, 发现其流程是这样的:

1. `kubelet`接收到调度指令, 开始创建pause容器, 完成且以containerID, 及其所属的netns等参数调用`bridge`插件, 然后`bridge`会发现我们已经创建了`mybr0`接口, 然后继续调用`dhcp`命令.
2. `dhcp`插件分为两个部分, 启动时加`daemon`参数的是作为服务端, 不加的则作为客户端. `bridge`调用时`dhcp`是作为客户端, 此时`dhcp`使用socket连接`/run/cni/dhcp.sock`文件, 主要有两种操作: `Accquire`和`Release`.
3. `dhcp`服务端在处理`Accquire`操作时, 发现`kubelet`传入的`netns`已经不存在了, 于是`cni`插件调用失败, 就出现了上述错误.

在网上搜索到了许多相关的问题(参考文章1到4等), 但是都没有明确的解决方案.

本来以为这是一个无法逾越的问题, 但是后我尝试把通过`cni-terway`启动的`dhcp`守护进程kill掉, 然后在宿主机使用命令行启动`dhcp daemon`, 然后重新创建业务Pod, 发现Pod启动成功了, 而且也正常的获得了IP...

所以现在的问题就是, 宿主机上的dhcp进程可以, Pod里启动就不行, 使用了`hostNetwork`也不行.

于是尝试了在部署yaml文件中添加`hostPID`字段, 就可以了(好吧其实我重新试了很多次).

在添加`hostPID`后, cni-terway的Pod中, `/proc`目录下就与宿主机的相同了, 宿主机上能访问到的`/proc/${pid}/ns/net`, 在Pod内部也能访问.

但是这样还不够, 如果进入cni-terway的Pod, 随便进入一个`/proc/${pid}/ns/net`, 会发现无法读取其下的namespace文件.

```console
/proc/101342/ns # ls -al
total 0
dr-x--x--x    2 root     root             0 Feb 13 17:25 .
dr-xr-xr-x    9 root     root             0 Feb 13 02:28 ..
ls: ./cgroup: cannot read link: Permission denied
lrwxrwxrwx    1 root     root             0 Feb 13 17:25 cgroup
ls: ./ipc: cannot read link: Permission denied
lrwxrwxrwx    1 root     root             0 Feb 13 17:25 ipc
ls: ./mnt: cannot read link: Permission denied
lrwxrwxrwx    1 root     root             0 Feb 13 17:25 mnt
ls: ./net: cannot read link: Permission denied
lrwxrwxrwx    1 root     root             0 Feb 13 17:25 net
ls: ./pid: cannot read link: Permission denied
lrwxrwxrwx    1 root     root             0 Feb 13 17:25 pid
ls: ./pid_for_children: cannot read link: Permission denied
lrwxrwxrwx    1 root     root             0 Feb 13 17:25 pid_for_children
ls: ./user: cannot read link: Permission denied
lrwxrwxrwx    1 root     root             0 Feb 13 17:25 user
ls: ./uts: cannot read link: Permission denied
lrwxrwxrwx    1 root     root             0 Feb 13 17:25 uts
```

按照参考文章5提问者的说法, 要读取`/proc/${pid}/ns/net`, 还需要为Pod添加`SYS_PTRACE`的`capabilities`. 

重新部署后, 再进入这样的目录, 会发现输出结果变成了这样.

```console
/proc/19117/ns # ls -al
total 0
dr-x--x--x    2 root     root             0 Feb 14 07:40 .
dr-xr-xr-x    9 root     root             0 Feb 14 07:36 ..
lrwxrwxrwx    1 root     root             0 Feb 14 07:40 cgroup -> cgroup:[4026531835]
lrwxrwxrwx    1 root     root             0 Feb 14 07:40 ipc -> ipc:[4026531839]
lrwxrwxrwx    1 root     root             0 Feb 14 07:40 mnt -> mnt:[4026531840]
lrwxrwxrwx    1 root     root             0 Feb 14 07:40 net -> net:[4026531992]
lrwxrwxrwx    1 root     root             0 Feb 14 07:40 pid -> pid:[4026531836]
lrwxrwxrwx    1 root     root             0 Feb 14 07:40 pid_for_children -> pid:[4026531836]
lrwxrwxrwx    1 root     root             0 Feb 14 07:40 user -> user:[4026531837]
lrwxrwxrwx    1 root     root             0 Feb 14 07:40 uts -> uts:[4026531838]
```

本来以为这样就可以了, 结果还是不行, 这次`/var/log/message`的日志变成了这种.

```
Feb 14 17:23:17 k8s-worker-02 kubelet[8288]: E0214 network: error calling DHCP.Allocate: error switching to ns /proc/49967/ns/net: Error switching to ns /proc/49967/ns/net: operation not permitted
```

本来觉得实在没有办法了, 我差点就想一个一个试`CAP_XXX`选项了. 后来找到了参考文章7, ta们做的更彻底, 直接写代码操作底层. 这篇文章介绍了一下`kubelet`在创建Pod时建立网络空间时的工作流程, 最重要的一点就是`setns()`的使用. 于是就想到了查一下`setns()`这个系统调用需要什么样的权限, 最终找到了参考文章6.

再添加上`SYS_ADMIN`字段, 终于成功了.

------

其实`dhcp`插件在工程代码中就给出了`.service`服务脚本, 本来建议使用`systemctl`将其作为服务启动的, 但是我更希望在Pod中集成这样的功能, 减少多余的操作.
