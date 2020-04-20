package cninet

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

// GetDefRoute 获取默认路由.
// 判断依据是 route 对象是否拥有 gw 成员, 因为一般的路由只有 dst, 没有gw.
// 如果没有默认路由, 则返回 nil.
func GetDefRoute() (route *netlink.Route, err error) {
	routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		return nil, fmt.Errorf("failed to get default route: %s", err)
	}
	for _, route := range routes {
		if route.Gw != nil {
			return &route, nil
		}
	}
	return nil, fmt.Errorf("default route doesn't exist")
}

// MakeDefRoute 生成用于默认路由对象, 需要指定网关.
func MakeDefRoute(gw net.IP) *netlink.Route {
	_, defnet, _ := net.ParseCIDR("0.0.0.0/0")
	return &netlink.Route{
		Scope: netlink.SCOPE_UNIVERSE,
		Dst:   defnet,
		Gw:    gw,
	}
}
