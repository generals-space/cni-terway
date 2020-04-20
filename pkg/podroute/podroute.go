package podroute

import (
	"fmt"
	"net"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/generals-space/cni-terway/pkg/cninet"
	"github.com/vishvananda/netlink"
	"k8s.io/klog"
)

// makeServiceCIDRRoute 生成 Pod 到 ServiceIP 的路由.
func makeServiceCIDRRoute(linkBridge netlink.Link, serviceIPCIDR string) (svcRoute *netlink.Route, err error) {
	bridgeAddrs, err := netlink.AddrList(linkBridge, netlink.FAMILY_V4)
	if err != nil {
		return nil, fmt.Errorf("faliled to get bridge address: %s", err)
	}
	klog.V(3).Infof("bridge addrs: %+v, len: %d", bridgeAddrs, len(bridgeAddrs))

	var gw net.IP
	if len(bridgeAddrs) > 0 {
		gw = bridgeAddrs[0].IP
	}

	svcRoute = &netlink.Route{
		Dst: &net.IPNet{
			IP: net.IPv4(10, 96, 0, 0), Mask: net.CIDRMask(12, 32),
		},
		Gw: gw,
	}
	if serviceIPCIDR != "" {
		// ParseCIDR 解析 192.168.0.0/12 网络字符串
		_, svcNet, err := net.ParseCIDR(serviceIPCIDR)
		if err != nil {
			return nil, fmt.Errorf("failed to parse service ip cidr %s: %v", serviceIPCIDR, err)
		}
		svcRoute.Dst = svcNet
	}

	return
}

// SetRouteInPod 在Pod空间里添加路由, 有两种情况:
// 1. 默认路由: 一般 bridge+dhcp 会自动为Pod创建默认路由, 但是Esxi环境下创建的Pod申请到IP后并不会创建, 需要补充上.
// 2. Pod 到 ServiceIP 的路由, 需要设置宿主机为该Pod的网关, 否则拥有宿主机网络IP的 Pod 无法访问 Service.
func SetRouteInPod(bridgeName, netnsPath, serviceIPCIDR string) (svcRoute *netlink.Route, err error) {
	// var hostDefRoute *netlink.Route
	linkBridge, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return nil, fmt.Errorf("faliled to get bridge link: %s", err)
	}

	// 获取宿主机上的默认路由, 之后需要在设置容器中默认路由时使用ta的网关.
	hostDefRoute, err := cninet.GetDefRoute()
	if err != nil {
		hostDefRoute = nil
		klog.Warning(err)
	}

	svcRoute, err = makeServiceCIDRRoute(linkBridge, serviceIPCIDR)
	if err != nil {
		return nil, fmt.Errorf("faliled to generate service route: %s", err)
	}

	netns, err := ns.GetNS(netnsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open netns %q: %v", netnsPath, err)
	}
	defer netns.Close()

	err = netns.Do(func(containerNS ns.NetNS) (err error) {
		link, err := netlink.LinkByName("eth0")
		if err != nil {
			return fmt.Errorf("faliled to get eth0 link: %s", err)
		}
		// 判断容器中是否存在默认路由, 如果不存在则创建(需要使用宿主机的网关).
		_, err = cninet.GetDefRoute()
		if err != nil {
			klog.Warning(err)
			defRoute := cninet.MakeDefRoute(hostDefRoute.Gw)
			defRoute.LinkIndex = link.Attrs().Index
			err = netlink.RouteAdd(defRoute)
			if err != nil {
				return fmt.Errorf("faliled to add default route: %s", err)
			}
		}
		// 添加到service cidr的路由.
		svcRoute.LinkIndex = link.Attrs().Index
		err = netlink.RouteAdd(svcRoute)
		if err != nil {
			return fmt.Errorf("faliled to add service cidr route: %s", err)
		}
		return nil
	})
	return svcRoute, err
}
