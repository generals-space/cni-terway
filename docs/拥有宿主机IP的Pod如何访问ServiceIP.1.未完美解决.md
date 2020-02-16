# 拥有宿主机IP的Pod如何访问ServiceIP.1.未完美解决

参考文章

1. [利用TPROXY在网桥上实现透明代理(Fully Transparent Proxy)功能 (CentOS 7)](https://www.lostend.com/index.php/archives/19/)
    - `TPROXY`工具(没用过)部署的步骤, 包括了两条`ebtables -t broute -A BROUTING`相关的命令.
2. [ebtables和iptables与linux bridge的交互](https://article.itxueyuan.com/wyEG1j)
    - `iptables`只对`ip`数据包进行过滤, 与`ip`协议同层的包括`arp`，`802.1q`等协议(`/etc/ethertypes`)的数据包是不会被`iptables`过滤的.

## 1. 引言

本来以为基本功能算是完成了, 结果发现`coredns`虽然成功获取到了物理网络的IP, 状态也变为了`Running`, 但是并没有`READY`.

```console
$ k get pod -o wide -A
NAMESPACE     NAME                                    READY   STATUS    RESTARTS   AGE   IP              NODE            NOMINATED NODE   READINESS GATES
kube-system   coredns-67c766df46-bm7qn                0/1     Running   0          43m   192.168.0.34    k8s-worker-02   <none>           <none>
kube-system   coredns-67c766df46-vclqc                0/1     Running   0          43m   192.168.0.33    k8s-worker-02   <none>           <none>
```

查看`coredns`的日志, 发现其连接不上`apiserver`.

```
E0215 00:37:48.766271       1 reflector.go:126] pkg/mod/k8s.io/client-go@v11.0.0+incompatible/tools/cache/reflector.go:94: Failed to list *v1.Endpoints: Get https://10.96.0.1:443/api/v1/endpoints?limit=500&resourceVersion=0: dial tcp 10.96.0.1:443: i/o timeout
```

这就指出了一个我之前没有考虑到的问题: **拥有宿主机IP的Pod如何访问ServiceIP?**

## 2. 探索

初始时, 安装完成此网络插件后创建的Pod的内部路由如下

```
default via 192.168.0.1 dev eth0 
192.168.0.0/24 dev eth0 proto kernel scope link src 192.168.0.55 
```

> `192.168.0.55`为Pod获取的IP.

其实只要为所有Pod创建到`ServiceIP`的路由, `ip route add 10.96.0.0/12 dev eth0 via 宿主机节点IP`即可(必须要有`via`字段, 否则在Pod内部只能ping通`ServiceIP`但无法访问对应的端口).

我尝试了在宿主机为`mybr0`接口添加上到`ServiceIP`的路由, 但是之后创建的Pod中, 不会继承该接口上的路由, 所以无效.

但我不希望像flannel那样存在两个可执行文件, 一个用来部署宿主机节点网络, 一个用来部署Pod容器网络. 我想在宿主机上做一些事情来达到这个目的.

为了解决这个问题我花费了一整天的时间, 从路由`ip route`到`iptables(SNAT, DNAT)`, 再到`ebtables(broute)`, 再到策略路由`ip rule`, 各种方法都用尽了都没有找到更好的方法.

最后我妥协了, 又创建了一个标准的cni插件, 只做了一件事, 就是在pause容器启动后, `bridge+dhcp`调用完成后再为其添加上那段路由...

最终Pod内部的的路由表如下

```
default via 192.168.0.1 dev eth0 
10.96.0.0/12 via 192.168.0.125 dev eth0 
192.168.0.0/24 dev eth0 proto kernel scope link src 192.168.0.55 
```