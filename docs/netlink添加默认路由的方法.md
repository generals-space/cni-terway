# netlink添加默认路由的方法

记录一下

```go
	_, defaultNet, _ := net.ParseCIDR("0.0.0.0/0")
	err = netlink.RouteAdd(&netlink.Route{
		LinkIndex: containerLink.Attrs().Index,
		Scope:     netlink.SCOPE_UNIVERSE,
		Dst:       defaultNet,
		Gw:        net.ParseIP(gateway),
	})
	if err != nil {
		return fmt.Errorf("config gateway failed %v", err)
	}
```

代码来源: kube-ovn v0.1.0, `pkg/daemon/ovs.go` -> `configureContainerNic()`.
