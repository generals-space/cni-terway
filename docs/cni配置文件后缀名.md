# cni配置文件后缀名

`/etc/cni/net.d`目录下配置文件有两种格式, 以后缀名区分: `*.conflist`和`*.conf`.

`flannel`的配置文件用的是第一种, 命名为`10-flannel.conflist`, 结构如下

```json
{
    "name": "cbr0",
    "cniVersion": "0.3.1",
    "plugins": [
        {
            "type": "flannel",
            "delegate": {
                "hairpinMode": true,
                "isDefaultGateway": true
            }
        },
        {
            "type": "portmap",
            "capabilities": {
                "portMappings": true
            }
        }
    ]
}
```

在使用`bridge+dhcp`原生插件做实验时, 后缀名使用`.conf`, 结构如下

```json
{
    "cniVersion": "0.3.1",
    "name": "mycninet",
    "type": "bridge",
    "bridge": "mybr0",
    "isGateway": false,
    "ipam": {
        "type": "dhcp"
    }
}
```

最初我并不知道这个点, 所以最初编译`cni-terway`并尝试运行时, 配置文件命名为`mybr0.conf`, 内容如下

```json
{
    "name": "mycninet",
    "cniVersion": "0.3.1",
    "plugins": [
        {
            "type": "cni-terway",
            "delegate": {
                "cniVersion": "0.3.1",
                "name": "mycninet",
                "type": "bridge",
                "bridge": "mybr0",
                "isGateway": false,
                "ipam": {
                    "type": "dhcp"
                }
            }
        }
    ]
}
```

重启kubelet, 日志中显示cni插件无法启动, 因为配置文件与内容的格式不匹配.

```
Feb 10 14:30:33 k8s-worker-02 kubelet[97206]: W0210 14:30:33.069410   97206 cni.go:177] Error loading CNI config file /etc/cni/net.d/mybr0.conf: error parsing configuration: missing 'type'
Feb 10 14:30:33 k8s-worker-02 kubelet[97206]: W0210 14:30:33.069463   97206 cni.go:237] Unable to update cni config: no valid networks found in /etc/cni/net.d
```
