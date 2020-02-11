需要借助kubernetescni插件中的bridge和dhcp客户端.

3. 添加路由
1. 启动dhcp服务.
2. 调用bridge插件

编译

```
go build -o cni-terway main.go
```
