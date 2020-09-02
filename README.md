你好！
很冒昧用这样的方式来和你沟通，如有打扰请忽略我的提交哈。我是光年实验室（gnlab.com）的HR，在招Golang开发工程师，我们是一个技术型团队，技术氛围非常好。全职和兼职都可以，不过最好是全职，工作地点杭州。
我们公司是做流量增长的，Golang负责开发SAAS平台的应用，我们做的很多应用是全新的，工作非常有挑战也很有意思，是国内很多大厂的顾问。
如果有兴趣的话加我微信：13515810775  ，也可以访问 https://gnlab.com/，联系客服转发给HR。
# yunba-perf-go

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
