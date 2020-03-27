# networkPlugin cni failed to set up pod xxx network: unexpected end of JSON input

在编写使用cni插件做固定IP的代码时, 发现`cni-terway`和`crd-ipkeeper`能够正常启动, 但是新建Pod时Pod总是处于`ContainerCreating`的状态, `describe`一下有如下输出

```
Events:
  Type     Reason                  Age                From                    Message
  ----     ------                  ----               ----                    -------
  Normal   Scheduled               <unknown>          default-scheduler       Successfully assigned kube-system/coredns-67c766df46-7v4gb to k8s-master-01
  Warning  FailedCreatePodSandBox  91s                kubelet, k8s-master-01  Failed create pod sandbox: rpc error: code = Unknown desc = failed to set up sandbox container "67d0522a49e15f895b21fa6421f60d6a0e8c4cc1337401700ec7a94bb2ac1384" network for pod "coredns-67c766df46-7v4gb": networkPlugin cni failed to set up pod "coredns-67c766df46-7v4gb_kube-system" network: request ip return 500 {}
  Normal   SandboxChanged          5s (x6 over 111s)  kubelet, k8s-master-01  Pod sandbox changed, it will be killed and re-created.
  Warning  FailedCreatePodSandBox  5s                 kubelet, k8s-master-01  Failed create pod sandbox: rpc error: code = Unknown desc = failed to set up sandbox container "0bad31111b2e5cabf72ed24e140a3348dc37953793ba45c84c83dcf20ab6d55e" network for pod "coredns-67c766df46-7v4gb": networkPlugin cni failed to set up pod "coredns-67c766df46-7v4gb_kube-system" network: request ip return 500 {}
```

但是`cni-terway`和`crd-ipkeeper`的Pod中并没有异常日志, 那么错误可能出现在`cni`部分, 由于`cni`插件是由kubelet调用的, 所以ta的日志也在`/var/log/message`中.

```
Mar 22 14:48:27 k8s-master-01 kubelet: I0322 14:48:27.696806   88851 main.go:152] start cni-terway plugin...
Mar 22 14:48:27 k8s-master-01 kubelet: E0322 14:48:27.903124   83710 remote_runtime.go:105] RunPodSandbox from runtime service failed: rpc error: code = Unknown desc = failed to set up sandbox container "fd8d2b9e6ca85bc57382073c2b8eddc38ecf8e6ec2f98eb3c2aaa5585b5bb009" network for pod "devops": networkPlugin cni failed to set up pod "devops_kube-system" network: unexpected end of JSON input
Mar 22 14:48:27 k8s-master-01 kubelet: E0322 14:48:27.903239   83710 kuberuntime_sandbox.go:68] CreatePodSandbox for pod "devops_kube-system(92320e55-bc31-490c-960a-cd4fdb1e3a88)" failed: rpc error: code = Unknown desc = failed to set up sandbox container "fd8d2b9e6ca85bc57382073c2b8eddc38ecf8e6ec2f98eb3c2aaa5585b5bb009" network for pod "devops": networkPlugin cni failed to set up pod "devops_kube-system" network: unexpected end of JSON input
Mar 22 14:48:27 k8s-master-01 kubelet: E0322 14:48:27.903256   83710 kuberuntime_manager.go:710] createPodSandbox for pod "devops_kube-system(92320e55-bc31-490c-960a-cd4fdb1e3a88)" failed: rpc error: code = Unknown desc = failed to set up sandbox container "fd8d2b9e6ca85bc57382073c2b8eddc38ecf8e6ec2f98eb3c2aaa5585b5bb009" network for pod "devops": networkPlugin cni failed to set up pod "devops_kube-system" network: unexpected end of JSON input
Mar 22 14:48:27 k8s-master-01 kubelet: E0322 14:48:27.903302   83710 pod_workers.go:191] Error syncing pod 92320e55-bc31-490c-960a-cd4fdb1e3a88 ("devops_kube-system(92320e55-bc31-490c-960a-cd4fdb1e3a88)"), skipping: failed to "CreatePodSandbox" for "devops_kube-system(92320e55-bc31-490c-960a-cd4fdb1e3a88)" with CreatePodSandboxError: "CreatePodSandbox for pod \"devops_kube-system(92320e55-bc31-490c-960a-cd4fdb1e3a88)\" failed: rpc error: code = Unknown desc = failed to set up sandbox container \"fd8d2b9e6ca85bc57382073c2b8eddc38ecf8e6ec2f98eb3c2aaa5585b5bb009\" network for pod \"devops\": networkPlugin cni failed to set up pod \"devops_kube-system\" network: unexpected end of JSON input"
Mar 22 14:48:28 k8s-master-01 kubelet: W0322 14:48:28.025993   83710 prober.go:108] No ref for container "docker://7cab2903ba345a2ddd21f10f122f0d8ea1cd2c03a230db5a1803a26c52396e08" (coredns-67c766df46-bbdpt_kube-system(d3fe692f-0208-4789-aea6-02cffc6b409e):coredns)
Mar 22 14:48:28 k8s-master-01 kubelet: W0322 14:48:28.272077   83710 docker_sandbox.go:394] failed to read pod IP from plugin/docker: networkPlugin cni failed on the status hook for pod "devops_kube-system": CNI failed to retrieve network namespace path: cannot find network namespace for the terminated container "fd8d2b9e6ca85bc57382073c2b8eddc38ecf8e6ec2f98eb3c2aaa5585b5bb009"
Mar 22 14:48:28 k8s-master-01 kubelet: W0322 14:48:28.308075   83710 pod_container_deletor.go:75] Container "fd8d2b9e6ca85bc57382073c2b8eddc38ecf8e6ec2f98eb3c2aaa5585b5bb009" not found in pod's containers
Mar 22 14:48:28 k8s-master-01 kubelet: W0322 14:48:28.310306   83710 cni.go:328] CNI failed to retrieve network namespace path: cannot find network namespace for the terminated container "fd8d2b9e6ca85bc57382073c2b8eddc38ecf8e6ec2f98eb3c2aaa5585b5bb009"
```

`unexpected end of JSON input`是一个非常奇怪的问题, 官方issue并没有明确的解决方法. 

> 最初我以为是因为`crd-ipkeeper`在设置容器内的静态IP时没有设置默认路由导致的, 但是加了之后仍然不行.

我没有找到如何设置`kubelet`的日志级别(在`/usr/lib/systemd/system/kubelet.service`中添加的`--v=5`参数无效, `kubelet`有固定的启动参数), 只能一点一点找.

按照日志中的关键字, 追踪的线索如下

1. `pkg/kubelet/dockershim/docker_sandbox.go` -> `dockerService.RunPodSandbox()`
2. `pkg/kubelet/dockershim/network/plugins.go` -> `PluginManager.SetUpPod()`
3. `pkg/kubelet/dockershim/network/cni/cni.go` -> `cniNetworkPlugin.SetUpPod()` -> `cniNetworkPlugin.addToNetwork()`
4. `gopath/pkg/mod/github.com/containernetworking/cni@v0.7.1/libcni/api.go` -> `CNIConfig.AddNetworkList()`

到这里, 我突然想到, `containernetworking/plugins`工程下的ipam插件在执行时都会打印出实际的网络配置, 而且在多个cni插件的代码中也的确存在`types.PrintResult()`.

于是尝试在通过`NewCNIServerClient`创建的客户端请求`crd-ipkeeper`中的cniserver设置固定IP后, 打印这样的`result`, 竟然真的可以了.

看样子是kubelet是通过`exec`的形式调用了ipam插件, 然后读取标准输出当作最终的设置结果, 作为`kubectl get pod -o wide`中的`IP`保存下来的.
