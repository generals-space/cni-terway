# client-go编译时出现not enough arguments in call to watch.NewStreamWatcher

引入`client-go`编写kuber客户端后编译代码时出现如下报错.

```
/go/pkg/mod/k8s.io/client-go@v11.0.0+incompatible/rest/request.go:598:31: not enough arguments in call to watch.NewStreamWatcher
	have (*versioned.Decoder)
	want (watch.Decoder, watch.Reporter)
```

这个问题挺普遍的, 搜一下有很多相关的问题.

其实主要在于`client-go`本身与`kubernetes`及其依赖工程(如`api`, `apimachinery`等)用得不是一套版本号, 代码的自动补全补上的两个工程相冲突.

```
require (
	k8s.io/apimachinery v0.17.3
    k8s.io/client-go v11.0.0+incompatible
)
```

这个对应关系在`client-go`的`readme.md`有写明, 以后要机会再研究, 这里将`go.mod`修改如下部分就可以了.

```
require (
	k8s.io/apimachinery v0.17.3
	k8s.io/client-go v0.17.0
)
```

