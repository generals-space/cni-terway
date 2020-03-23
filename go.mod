module github.com/generals-space/cni-terway

go 1.12

require (
	github.com/containernetworking/cni v0.7.1
	github.com/containernetworking/plugins v0.8.5
	github.com/generals-space/crd-ipkeeper v0.0.0-20200322200009-2a1a971a9091
	github.com/vishvananda/netlink v1.1.0
	golang.org/x/sys v0.0.0-20191120155948-bd437916bb0e // indirect
	k8s.io/apimachinery v0.17.4
	k8s.io/client-go v0.17.4
	k8s.io/klog v1.0.0
)
