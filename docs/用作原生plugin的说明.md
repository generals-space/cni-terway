阅读flannel源码可知, flannel插件正常运行分为两部分: 

1. 部署flannel Pod, 为[coreos/flannel]工程, 用于创建`flannel.1`接口, 配置iptables, 路由等;
2. kubelet调用flannel cni插件, 位于[plugin/flannel]工程, 这个可执行文件又最终调用了`bridge`插件;

我们要做的事情很简单, 就是预先创建bridge接口, 将物理网卡接入, 然后将原物理网卡的IP由新的bridge接管, 模拟类似虚拟机的"桥接模式". 

之前的测试是将此工程放在第2步使用的, 即先手动将配置文件拷贝到`/etc/cni/net.d`目录下, 然后重启操作系统(由于此插件会修改网卡设备和路由, 不可逆, 只能重启才能重新部署). 之后再创建Pod, 就可以获得物理网络地址.

但这个步骤放在flannel的第1步已经够用, 所以在本次提交到将做修正.
