# 原生bridge+dhcp实践

参考文章

1. [Kubernetes利用CNI-bridge插件打通网络](https://blog.csdn.net/qq_36183935/article/details/90735049)
    - 直接使用`bridge`插件作为kuber集群的网络插件的示例步骤
    - 详细解释了`bridge`插件在创建Pod分配IP时的工作流程
2. [linux brctl](https://blog.csdn.net/hoO_flying/article/details/1553175)
    - 该文的示例与参考文章1中让bridge设备替代eth0本质上一致(不过是`brctl`的示例).
3. [初探 CNI 的 IP 分配問題 (IPAM)](https://www.hwchiu.com/cni-ipam.html)
    - CNI中dhcp插件的使用方法(其实没有明确的使用方法, dhcp的请求广播包要能通过物理网卡出去才行)
    - 本文是对dhcp插件使用的最核心的文章了

**环境**

Mac VMware装CentOS7, NAT网络.

master: 172.16.91.10/24
kuber: 1.16.2(单一master节点)

------

创建`/etc/cni/net.d/mybr.conf`文件

```json
{
    "cniVersion": "0.3.1",
    "name": "mycninet",
    "type": "bridge",
    "bridge": "mybr0",
    "isGateway": false,
    "ipam": {
        "type": "dhcp"
    }
}
```

同时启动dhcp服务

```console
$ /opt/cni/bin/dhcp daemon
```

重启`kubelet`服务使cni配置生效.

```console
$ systemctl restart kubelet
```

在创建新的pod的时候, `bridge`会自动创建名为`mybr0`的网络接口, 但是dhcp进程无法获取IP.

```
2020/01/31 15:22:14 cc4a50f9e4d56811e422296866d4b71aba1645c227d6a2af48be6da4b49986e2/mycninet: acquiring lease
2020/01/31 15:22:19 resource temporarily unavailable
2020/01/31 15:22:28 resource temporarily unavailable
2020/01/31 15:22:40 resource temporarily unavailable
```

使用`tcpdump`抓包发现, `eth0`上根本没有`udp`包流经, 只有`mybr0`接口上有. 

```console
$ tcpdump -i mybr0 -p udp
tcpdump: verbose output suppressed, use -v or -vv for full protocol decode
listening on mybr0, link-type EN10MB (Ethernet), capture size 262144 bytes
14:33:37.399942 IP 0.0.0.0.bootpc > 255.255.255.255.bootps: BOOTP/DHCP, Request from 5e:15:05:0c:13:92 (oui Unknown), length 272
14:34:00.922868 IP 0.0.0.0.bootpc > 255.255.255.255.bootps: BOOTP/DHCP, Request from 0a:8b:45:6d:2c:00 (oui Unknown), length 272
```

看这情况明显是`kubelet`通过`mybr0`接口尝试对外进行dhcp广播, 但`bridge`创建的`mybr0`接口并没有IP地址, 并没有连接到物理机网络, 导致请求没有办法被dhcp服务器接收到.

于是, 按照参考文章1中的做法, 将`mybr.conf`中的`isGateway`改为`true`, 并重启`kubelet`生效. 然后执行如下命令

```bash
ip link set eth0 master mybr0
```

网络会出现中断, 之后`mybr0`上会出现IP地址`172.16.91.2`(呃, VM的NAT网络, 这个地址应该是DNS和DHCP服务的地址, 不知道为什么会放到这里), 而且`eth0`接口上的地址不见了...

就是说, 作为连接实际网络的物理接口`eth0`, 通过`master`操作接入网桥`mybr0`后, 由后者接管了物理网络. 网络中的其他主机只能通过`172.16.91.2`这个地址访问, `eth0`就像变成了一根网线, 只作数据传输使用.

但是这样肯定是不行的, 宿主机对外的地址变了, 所有网络功能都会发生变动, `kubectl`也需要重新指定apiserver地址.

```
$ k get pod
Unable to connect to the server: dial tcp 172.16.91.10:8443: connect: no route to host
```

参考文章1中还有后续步骤, 就是为失去IP地址的`eth0`重新添加上地址, 并且为`mybr0`也添加上地址. 

```bash
ip addr add 172.16.91.10/24 dev ens33
ip addr add 172.16.91.3/24 dev mybr0
```

由于参考文章1中并未创建Pod, 所以`mybr0`上还没有地址, 但我们这里由于上面示例的原因, 已经分配了一个`172.16.91.2`, 考虑到这个IP的特殊性, 这里还需要将其删除.

```bash
ip addr del 172.16.91.2/24 dev mybr0
```

但此时网络中还是无法访问`172.16.91.10`, 而且最终发现原因为路由的混乱.

```console
$ ip r
10.254.0.0/24 dev cni0 proto kernel scope link src 10.254.0.1
172.16.91.0/24 dev ens33 proto kernel scope link src 172.16.91.10
172.16.91.0/24 dev mybr0 proto kernel scope link src 172.16.91.3
172.17.0.0/16 dev docker0 proto kernel scope link src 172.17.0.1
```

第2条是多出来的, 虽然ens33才是物理网卡, 但是实际接入网络的其实是`mybr0`了, 所以移除这条路由.

```bash
ip r del 172.16.91.0/24 src 172.16.91.10
```

我也不知道是不是因为和参考文章1中步骤不同的原因, 总之按照ta文章中给出的操作, master的网络就断了, 肯定还需要其他的工作.
