基于 云巴 Go SDK [yunba/mqtt.go](https://github.com/yunba/mqtt.go) 修改的 `publish` 性能测试工具。

## 先进入测试程序所在目录：
```bash
cd samples/yb-perf
```

## 查看帮助
```bash
go run yb-perf.go --help
```

## 注册
```bash
go run yb-perf.go --broker tcp://127.0.0.1:1883 --reg --regcnt 1
```

## 测试 `publish`
```bash
go run yb-perf.go --broker tcp://127.0.0.1:1883 --pubcnt 1 --subcnt 100
```
