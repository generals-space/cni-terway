# 在程序中获取service-cluster-ip-range值

参考文章

1. [Kubernetes - Find out service ip range CIDR programatically](https://stackoverflow.com/questions/44154425/kubernetes-find-out-service-ip-range-cidr-programatically)
2. [Expose the service-cluster-ip-range CIDR through the API Server](https://github.com/kubernetes/kubernetes/issues/25533)
3. [Would like `ClusterCIDR` to be fetchable by pods.](https://github.com/kubernetes/kubernetes/issues/46508)
    - 如果添加这样的接口, 会影响其他网络插件, 比如Calico???

我希望在`cni-terway`中能够获取`ServiceIP`的网段范围, 以便向Pod中添加路由. 

但是kuber的API中貌似不存在这样的接口, 见参考文章1, 2, 3的issue.

不过`kubectl cluster-info dump`命令可以查到`service-cluster-ip-range`.

```
$ k cluster-info dump | grep service-cluster-ip-range
                            "--service-cluster-ip-range=10.96.0.0/12",
                            "--service-cluster-ip-range=10.96.0.0/12",
```

于是看看ta是怎么做的, 最终的实现在`kubernetes/staging/src/k8s.io/kubectl/pkg/cmd/clusterinfo/clusterinfo_dump.go` -> `ClusterInfoDumpOptions.Run()`函数中.

其实就是获取所有的`PodList`对象, 全部打印出来而已. 上面的结果其实就是`kube-apiserver`的`command`字段的输出, 于是我也这么做了, 不算太复杂.

> 还有`--cluster-cidr`选项, 不过没在`apiserver`的yaml中, 而是在`kube-controller-manager`中.

