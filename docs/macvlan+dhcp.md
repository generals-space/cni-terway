```json
{
    "cniVersion": "0.3.1",
	"name": "mynet",
	"type": "macvlan",
	"master": "ens160",
	"ipam": {
		"type": "dhcp"
	}
}
```

启动dhcp插件, 然后重启kubelet组件即可生效.

pod创建成功后, 宿主机上看不到任何接口...

macvlan有一个缺点, 就是宿主机和运行在本机上的pod, 双方是无法通信的, 但是其他的可以, 不能不说是一种遗憾.
