基于 [云巴 Go SDK](https://github.com/yunba/mqtt.go) 修改的 `publish` 性能测试工具。

## 安装依赖的 websocket
```bash
go get golang.org/x/net/websocket
```

## 进入测试程序所在目录并 build
```bash
cd samples/yb-perf
go build
```

## 查看帮助
```bash
./yb-perf.go --help
```

## 注册
```bash
./yb-perf.go --broker tcp://127.0.0.1:1883 --reg 2
```

## 测试 publish
```bash
./yb-perf.go --broker tcp://127.0.0.1:1883 --pubcnt 1 --subcnt 1
```

