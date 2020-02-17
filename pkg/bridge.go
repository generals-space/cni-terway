package pkg

import (
	"github.com/vishvananda/netlink"
	"k8s.io/klog"

)

// getBridgeDevice 获取目标网桥设备, 如不存在则创建然后返回.
func getBridgeDevice(name string) (link netlink.Link, err error) {
	link, err = netlink.LinkByName(name)
	if err == nil {
		return
	}
	// 如果获取出错, 先判断是否为设备不存在
	if err.Error() != "Link not found" {
		klog.Errorf("failed to get bridge %s: %s.", name, err)
		return
	}
	klog.Warningf("bridge: %s doesn't exist, try to create it manually.", name)

	// 尝试手动创建然后启动
	bridge := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name: name,
		},
	}
	err = netlink.LinkAdd(bridge)
	if err != nil {
		klog.Warningf("failed to craete bridge %s: %s", name, err)
		return
	}
	err = netlink.LinkSetUp(bridge)
	if err != nil {
		klog.Warningf("failed to setup bridge %s: %s", name, err)
		return
	}

	// 再次尝试获取
	link, err = netlink.LinkByName(name)
	if err != nil {
		klog.Warningf("failed to get bridge %s: %s", name, err)
		return
	}
	return
}

// migrateIPAddrs 在桥接网络中, 需要bridge网桥设备接管物理网卡的IP, 而网卡本身则作为网线存在.
// 在这个函数中会将源设备上的IP移除, 然后添加到目标设备上.
// 在部署网络与卸载网络时都会被调用(源设备和目标设备调换即可).
func migrateIPAddrs(src, dst netlink.Link)(err error) {
	srcName := src.Attrs().Name
	dstName := dst.Attrs().Name

	// 获取指定设备相关的路由
	addrs, err := netlink.AddrList(src, netlink.FAMILY_V4)
	if err != nil {
		klog.Errorf("failed to get address list on %s: %s", srcName, err)
		return
	}
	klog.V(3).Infof("get addresses of device %s, len: %d: %+v", srcName, len(addrs), addrs)

	// 在从 src 移除IP时, 相关路由就会同时被移除, 所以这里先获取.
	// 之后要由 dst 设备接管 src 设备的所有路由.
	routes, err := netlink.RouteList(src, netlink.FAMILY_V4)
	if err != nil {
		klog.Errorf("failed to get address list on %s: %s", srcName, err)
		return
	}
	klog.V(3).Infof("get route items of device %s, len: %d: %+v", srcName, len(routes), routes)

	for _, addr := range addrs {
		// 从一个接口上移除IP地址, 对应的路由应该也会被移除.
		err = netlink.AddrDel(src, &addr)
		if err != nil {
			klog.Errorf("failed to del addr on %s: %s", srcName, err)
			continue
		}
		// 不知道Label是什么意思, 但是如果不修改这个字段, 会出现如下问题.
		// label must begin with interface name
		// 在AddrAdd()源码中有检验Label是否以接口名称为前缀的判断.
		addr.Label = dstName
		err = netlink.AddrAdd(dst, &addr)
		if err != nil {
			klog.Errorf("failed to add addr to %s: %s", dstName, err)
			continue
		}
	}
	klog.V(3).Infof("move address from %s to %s success", srcName, dstName)

	/*
		// 修改原网卡的IP地址为0.0.0.0(没有掩码), 不过在运行到这里的时候会panic, 应该怎么做还没想好. ???
		newAddr := &netlink.Addr{
			IPNet: &net.IPNet{IP: net.ParseIP("0.0.0.0")},
		}
		err = netlink.AddrAdd(src, newAddr)
		if err != nil {
			return
		}
	*/

	return modifyRoutes(routes, dst.Attrs().Index)
}

// modifyRoutes IP地址从物理网卡迁移到bridge设备后还需要修改相关路由的`dev`字段.
// caller: migrateIPAddrs()
func modifyRoutes(routes []netlink.Route, devIndex int)(err error){
	// 修改路由, 这里是逆向遍历, 因为routes[0]为默认路由, 在目标路由不指定的情况下添加default会失败.
	// Error: Nexthop has invalid gateway.
	// 所以我们先修改指定目标网络的路由, 如
	// 192.168.0.0/24 dev eth0 proto kernel scope link src 192.168.0.105 metric 100
	length := len(routes)
	for i := 0; i < length; i++ {
		route := routes[length-i-1]
		err = netlink.RouteDel(&route)
		if err != nil {
			// 有可能在移除eth0上的IP时, 对应的路由就自动被移除了, 所以这里出错不return
			klog.Errorf("failed to del route %+v: %s", route, err)
		}
		// 我们变更路由主要是将路由条目的`dev`bridgeName接口.
		// 而在route对象中, LinkIndex才是表示
		route.LinkIndex = devIndex
		err = netlink.RouteAdd(&route)
		if err != nil {
			klog.Errorf("failed to add route %+v: %s", route, err)
			return
		}
	}

	return
}

func getBridgeAndEth0(bridgeName, eth0Name string) (bridge netlink.Link, eth0 netlink.Link, err error) {
	bridge, err = getBridgeDevice(bridgeName)
	if err != nil {
		// no need to log
		return
	}

	eth0, err = netlink.LinkByName(eth0Name)
	if err != nil {
		klog.Errorf("failed to get target device %s: %s", eth0Name, err)
		return
	}

	return
}

// InstallBridgeNetwork 部署桥接网络.
// 手动创建 mybr0 接口, 然后将宿主机主网卡 eth0 接入.
// 因为如果不完成接入, invoke 调用 bridge+dhcp 插件时请求会失败, 出现如下报错
// error calling DHCP.Allocate: no more tries
func InstallBridgeNetwork(bridgeName, eth0Name string) (err error) {
	linkBridge, linkEth0, err := getBridgeAndEth0(bridgeName, eth0Name)
	if err != nil {
		// no need to log
		return
	}
	// SetMaster 并不会影响 eht0 上的IP及相关路由, 需要手动进行操作.
	err = netlink.LinkSetMaster(linkEth0, linkBridge)
	if err != nil {
		klog.Errorf("failed to set %s master to %s: %s", eth0Name, err)
		return err
	}
	klog.V(3).Infof("set %s master %s success", eth0Name, bridgeName)

	err = migrateIPAddrs(linkEth0, linkBridge)
	if err != nil {
		// no need to log
		return err
	}

	return
}

// UninstallBridgeNetwork 移除桥接网络, 在此插件被移除时被调用.
// 将物理网卡 eth0 从 mybr0 网桥设备中拔出, 并且恢复其路由配置.
// 最终移除 mybr0.
func UninstallBridgeNetwork(bridgeName, eth0Name string) (err error){
	linkBridge, linkEth0, err := getBridgeAndEth0(bridgeName, eth0Name)
	if err != nil {
		// no need to log
		return
	}

	err = netlink.LinkSetNoMaster(linkEth0)
	if err != nil {
		klog.Errorf("failed to set nomaster for %s: %s", eth0Name, err)
		return err
	}
	klog.V(3).Infof("set nomaster for %s success", eth0Name)

	err = migrateIPAddrs(linkBridge, linkEth0)
	if err != nil {
		// no need to log
		return err
	}

	err = netlink.LinkDel(linkBridge)
	if err != nil {
		klog.Errorf("failed to remove bridge device %s: %s", bridgeName, err)
		return
	}
	return
}
