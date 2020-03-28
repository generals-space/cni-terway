# VMware Esxi无法通过DHCP获取IP

1. [EXSI安装虚机地址不通的问题--番外篇](http://blog.sina.com.cn/s/blog_700d0afe01019hi5.html)
    - `esxi上`安装`xenserver`, 然后`xenserver`再安装`centos`(双层虚拟, 是个人才). 
    - `xenserver`进行dhcp失败, 但手动设置ip能通外网.
    - 场景极度类似, 开启混杂模式可解决.
2. [Esxi虚拟系统中虚拟机docker桥接网络](https://blog.csdn.net/qq_39471962/article/details/80832140)
    - Esxi虚拟机+docker模拟桥接网桥

在本地VMware(NAT模式, 桥接模式)通过测试之后, 又尝试在VMware Esxi虚拟机集群进行测试, 但是出现了问题.

`cni-terway`的`Pod`资源运行正常, 但是之后业务容器一直处于`ContainerCreating`状态. 在`/var/log/message`日志中发现有如下错误

```
Mar 28 15:10:50 master01 kubelet: I0328 15:10:50.957228    2889 main.go:190] start cni-terway plugin...
Mar 28 15:10:50 master01 NetworkManager[985]: <info>  [1585379450.9601] device (veth874e66e6): released from master device mybr0
Mar 28 15:10:50 master01 kubelet: E0328 15:10:50.974397   21732 remote_runtime.go:105] RunPodSandbox from runtime service failed: rpc error: code = Unknown desc = failed to set up sandbox container "ff30af421955231c5051e52cd43f0e26980e865389e241b45791e1de67da905d" network for pod "coredns-67c766df46-pvbxt": networkPlugin cni failed to set up pod "coredns-67c766df46-pvbxt_kube-system" network: error calling DHCP.Allocate: no more tries
Mar 28 15:10:50 master01 kubelet: E0328 15:10:50.974562   21732 kuberuntime_sandbox.go:68] CreatePodSandbox for pod "coredns-67c766df46-pvbxt_kube-system(5ceaf471-745b-4b43-a4a8-6b09d09a43c1)" failed: rpc error: code = Unknown desc = failed to set up sandbox container "ff30af421955231c5051e52cd43f0e26980e865389e241b45791e1de67da905d" network for pod "coredns-67c766df46-pvbxt": networkPlugin cni failed to set up pod "coredns-67c766df46-pvbxt_kube-system" network: error calling DHCP.Allocate: no more tries
Mar 28 15:10:50 master01 kubelet: E0328 15:10:50.974611   21732 kuberuntime_manager.go:710] createPodSandbox for pod "coredns-67c766df46-pvbxt_kube-system(5ceaf471-745b-4b43-a4a8-6b09d09a43c1)" failed: rpc error: code = Unknown desc = failed to set up sandbox container "ff30af421955231c5051e52cd43f0e26980e865389e241b45791e1de67da905d" network for pod "coredns-67c766df46-pvbxt": networkPlugin cni failed to set up pod "coredns-67c766df46-pvbxt_kube-system" network: error calling DHCP.Allocate: no more tries
Mar 28 15:10:50 master01 kubelet: E0328 15:10:50.974769   21732 pod_workers.go:191] Error syncing pod 5ceaf471-745b-4b43-a4a8-6b09d09a43c1 ("coredns-67c766df46-pvbxt_kube-system(5ceaf471-745b-4b43-a4a8-6b09d09a43c1)"), skipping: failed to "CreatePodSandbox" for "coredns-67c766df46-pvbxt_kube-system(5ceaf471-745b-4b43-a4a8-6b09d09a43c1)" with CreatePodSandboxError: "CreatePodSandbox for pod \"coredns-67c766df46-pvbxt_kube-system(5ceaf471-745b-4b43-a4a8-6b09d09a43c1)\" failed: rpc error: code = Unknown desc = failed to set up sandbox container \"ff30af421955231c5051e52cd43f0e26980e865389e241b45791e1de67da905d\" network for pod \"coredns-67c766df46-pvbxt\": networkPlugin cni failed to set up pod \"coredns-67c766df46-pvbxt_kube-system\" network: error calling DHCP.Allocate: no more tries"
Mar 28 15:10:51 master01 kubelet: W0328 15:10:51.563163   21732 docker_sandbox.go:394] failed to read pod IP from plugin/docker: networkPlugin cni failed on the status hook for pod "coredns-67c766df46-pvbxt_kube-system": CNI failed to retrieve network namespace path: cannot find network namespace for the terminated container "ff30af421955231c5051e52cd43f0e26980e865389e241b45791e1de67da905d"
Mar 28 15:10:51 master01 kubelet: W0328 15:10:51.567635   21732 pod_container_deletor.go:75] Container "ff30af421955231c5051e52cd43f0e26980e865389e241b45791e1de67da905d" not found in pod's containers
Mar 28 15:10:51 master01 kubelet: W0328 15:10:51.571958   21732 cni.go:328] CNI failed to retrieve network namespace path: cannot find network namespace for the terminated container "ff30af421955231c5051e52cd43f0e26980e865389e241b45791e1de67da905d"
```

`/run/cni/dhcp.log`日志输出如下

```
2020/03/28 05:46:22 b24b1f9890dce6ee22f03bcabe1a24862c1a91796317af6adb002158bf368bfd/mycninet: acquiring lease
2020/03/28 05:46:31 resource temporarily unavailable
2020/03/28 05:46:31 resource temporarily unavailable
2020/03/28 05:46:42 resource temporarily unavailable
```

`error calling DHCP.Allocate: no more tries`这个错误似曾相识, 明显是之前因为未设置`hostPID: true`而出现的问题, 但是为什么在esxi中又不行了呢?

我尝试杀掉由`cni-terway` Pod启动的`dhcp daemon`子进程, 手动在容器外启动, 但是无效. 

然后使用tcpdump抓包, 发现只有本机发出的`Discover`广播, 但是没有`Offer`回应, 而且`Discover`的请求貌似根本没有广播出去, worker01上无法收到worker02发出的广播包.

```
tcpdump -nve -i ens192 udp and port 67 and port 68
```

> dhcp协议使用的是`udp/67`和`udp/68`端口.

这不由得让我觉得, esxi集群和各种云环境的十分相似了, 毕竟目前云环境是无法任由用户发送广播包的.

但是`vmware workstation`实现的较为底层的虚拟化, 按理说esxi的限制不会像云环境那样严格, 毕竟可以自由添加虚拟交换机, 配置vlan等.

于是尝试以**esxi 虚拟网桥**为关键字进行搜索, 找到了两篇相关文章, 都有提到交换机的**混杂模式**.

查看了一下, 虚拟交换机的配置的确是禁用了混杂模式的.

![](https://gitee.com/generals-space/gitimg/raw/master/466D521FF652FDF17189A54843C1B779.png)

最初本来想只启用kuber节点上网卡的混杂模式的(`ip link set ens192 promisc on`), 但是无效, 所以只能冒险开启虚拟交换机的混杂模式了(开玩笑, 混杂模式应该只会影响性能, 方便嗅探测试而已).

![](https://gitee.com/generals-space/gitimg/raw/master/466D521FF652FDF17189A54843C1B779.png)

然后再次进行尝试, 成功了.

...但是还没完.

虽然之后的业务容器可以通过dhcp获取IP了, 但是进入容器发现无法访问外网, 因为默认路由不见了...

在调用dhcp作ipam的时候是没有管过默认路由的, 因为在本地使用vmware测试时, bridge+dhcp会自动为pause设置上默认路由, 这下需要手动完成了.

