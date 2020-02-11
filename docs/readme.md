参考文章

1. [libvirt - VirtualNetworking](https://wiki.libvirt.org/page/VirtualNetworking)
2. [理解桥接bridge和dhcp的原理](https://yuerblog.cc/2017/01/22/understand-bridge-and-dhcp/)
3. [libvirt kvm 虚拟机上网 – Bridge桥接](https://www.chenyudong.com/archives/libvirt-kvm-bridge-network.html)
4. [一个关于Linux Bridge配置的吐嘈](https://blog.51cto.com/dog250/1412926)
    - 废话很多, 但是解释还是可以的
    - 确认了物理网卡在连接到bridge设备时网络中断无法解决的原因, 并且给出了一些建议方案(实现起来有些麻烦).
5. [Proper isolation of a Linux bridge](https://vincent.bernat.ch/en/blog/2017-linux-bridge-isolation)
    - 最有深度的一篇文章, 基本解释了虚拟机bridge桥接模式的实现原理, 以及一些技术细节(bridge设备会获得ip, 如何通过ebtables转发二层包等)
    - 本文中的`ebtables`方法应该是参考文章4中`ebtables`方案的理论来源
    - 场景是不希望绿色部分的虚拟机访问到紫色部分的内网
6. [云计算底层技术-虚拟网络设备(Bridge,VLAN)](https://opengers.github.io/openstack/openstack-base-virtual-network-devices-bridge-and-vlan/)
    - 很深入

我们要做的就类似于vmware/virtualbox中的桥接模型.

我始终想找到一种可以不将eth0连接到mybr0网桥上的方法, 因为eth0会失去IP, 这样太奇怪了.

当然, 使用macvlan+dhcp就简单了, macvlan插件的readme文档中就说了, macvlan就类似于一个已经连接到bridge的虚拟网卡, 且该虚拟网卡就是借助于物理网卡实现的.

那么为什么macvlan就可以, bridge就不可以呢? 为什么dhcp广播包不能通过mybr0->eth0传播到网络中呢?

