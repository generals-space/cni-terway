# 拥有宿主机IP的Pod如何访问ServiceIP(未完美解决)

参考文章

1. [利用TPROXY在网桥上实现透明代理(Fully Transparent Proxy)功能 (CentOS 7)](https://www.lostend.com/index.php/archives/19/)
    - `TPROXY`工具(没用过)部署的步骤, 包括了两条`ebtables -t broute -A BROUTING`相关的命令.
2. [ebtables和iptables与linux bridge的交互](https://article.itxueyuan.com/wyEG1j)
3. [ebtables和iptables与linux bridge的交互](https://www.cnblogs.com/balance/p/8711264.html)
    - `iptables`只对`ip`数据包进行过滤, 与`ip`协议同层的包括`arp`，`802.1q`等协议(`/etc/ethertypes`)的数据包是不会被`iptables`过滤的.
3. [ebtables中的broute表介绍](https://blog.csdn.net/zxygww/article/details/48056601)
4. [ebtables之BROUTING和PREROUTING的redirect的区别](https://blog.csdn.net/dog250/article/details/7269212)
    - 内核源码级别的分析
5. [ebtables中的broute表介绍](https://blog.csdn.net/zxygww/article/details/48056601)
    - ebtables中的broute表功能: 用于控制进来的数据包是需要进行bridge转发还是进行route转发, 即2层转发和3层转发.
    - 完美配图
4. [CentOS 8 都上生产了，你还在用 iptables 吗，是时候拥抱下一代防火墙 nftables 了！...](https://blog.csdn.net/easylife206/article/details/103142273)


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

------

为了解决这个问题我花费了一整天的时间, 从路由`ip route`到`iptables`, 再到`ebtables`, 再到策略路由`ip rule`, 各种方法都用尽了都没有找到更好的方法.

## 2. 初步思考

```console
$ tcpdump -nnve -i mybr0 icmp and host 192.168.0.31
09:19:13.460894 1e:cf:b9:95:f8:d7 > a8:6b:7c:9b:10:f6, ethertype IPv4 (0x0800), length 98: (tos 0x0, ttl 64, id 59455, offset 0, flags [DF], proto ICMP (1), length 84)
    192.168.0.31 > 10.96.0.1: ICMP echo request, id 29712, seq 1, length 64
```


```console
$ ip r
default via 192.168.0.1 dev eth0
192.168.0.0/24 dev eth0 proto kernel scope link src 192.168.0.31
```

> PodIP为`192.168.0.31`.


```
11:57:41.124407 1e:cf:b9:95:f8:d7 > 00:0c:29:4b:22:09, ethertype IPv4 (0x0800), length 98: (tos 0x0, ttl 64, id 57840, offset 0, flags [DF], proto ICMP (1), length 84)
    192.168.0.31 > 10.96.0.1: ICMP echo request, id 45647, seq 1, length 64
11:57:41.124481 00:0c:29:4b:22:09 > 1e:cf:b9:95:f8:d7, ethertype IPv4 (0x0800), length 98: (tos 0x0, ttl 64, id 31448, offset 0, flags [none], proto ICMP (1), length 84)
    10.96.0.1 > 192.168.0.31: ICMP echo reply, id 45647, seq 1, length 64
```

```
$ ip r
default via 192.168.0.1 dev eth0 
10.96.0.0/12 dev eth0 scope link 
192.168.0.0/24 dev eth0 proto kernel scope link src 192.168.0.31 

```

> 不指定`scope`时貌似就是默认的`link`.

下面是在容器内部抓的包.

```console
$ tcpdump -nnve -i eth0 arp
12:14:17.301807 1e:cf:b9:95:f8:d7 > 00:0c:29:4b:22:09, ethertype ARP (0x0806), length 42: Ethernet (len 6), IPv4 (len 4), Request who-has 10.96.0.1 tell 192.168.0.31, length 28
12:14:17.301996 00:0c:29:4b:22:09 > 1e:cf:b9:95:f8:d7, ethertype ARP (0x0806), length 42: Ethernet (len 6), IPv4 (len 4), Reply 10.96.0.1 is-at 00:0c:29:4b:22:09, length 28
```

添加上路由后, Pod会通过eth0向其直连网络发送arp广播请求, 回应的是`mybr0`接口.

我对比了宿主机上`mybr0`接口上的抓包, 与上面的相同. 看来是`mybr0`直接接管了对`10.96.0.1`的请求.



```console
$ ipvsadm -ln
IP Virtual Server version 1.2.1 (size=4096)
Prot LocalAddress:Port Scheduler Flags
  -> RemoteAddress:Port           Forward Weight ActiveConn InActConn
TCP  10.96.0.1:443 rr
  -> 192.168.0.101:6443           Masq    1      0          0
TCP  10.96.0.10:53 rr
  -> 192.168.0.29:53              Masq    1      0          0
  -> 192.168.0.31:53              Masq    1      0          0
TCP  10.96.0.10:9153 rr
  -> 192.168.0.29:9153            Masq    1      0          0
  -> 192.168.0.31:9153            Masq    1      0          0
UDP  10.96.0.10:53 rr
  -> 192.168.0.29:53              Masq    1      0          0
  -> 192.168.0.31:53              Masq    1      0          0
```

1. 为所有Pod创建到ServiceIP的路由, `ip route add 10.96.0.0/12 dev eth0`(我尝试了在宿主机为`mybr0`接口添加上到`ServiceIP`的路由, 但是之后创建的Pod中, 不会继承该接口上的路由, 所以无效).
2. 在宿主机用iptables做NAT转发.

```
iptables -t nat -A POSTROUTING -s 192.168.0.0/24 -o mybr0 -j MASQUERADE
iptables -t filter -A FORWARD -d 10.96.0.0/12 -j ACCEPT
```

本来以为应该是用`MASQUERADE`操作, 想了想, `MASQUERADE`其实是特殊的`SNAT`, 应该用`DNAT`才对. 但是命令写出来发现也不对, `DNAT`就是要改写目标地址, 但是看上面的`tcpdump`结果, 访问的目标IP是确定的, 不需要改写...






iptables -t nat -A PREROUTING -d 10.96.0.0/12 -j DNAT --to-destination 192.168.0.125
iptables -t nat -A POSTROUTING -d 10.96.0.0/12 -j SNAT --to-source 192.168.0.125

ebtables -t broute -A BROUTING -i mybr0 -p ip --ip-destination 10.96.0.0/12 -j redirect --redirect-target DROP

