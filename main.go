package main

import (
	"cex/common"
	"cex/common/logger"
	"cex/config"
	"fmt"
	"os"
	"time"
)

// 全局变量
var cfg config.Config
var ctxt Context
var orderHandler OrderHandler
var eventHandler EventHandler

func Init(conf *config.Config) {
	// 初始化上下文
	ctxt.Init(conf)

	// 初始化order handlers, 通过HTTPS API 处理订单相关信息
	orderHandler.Init(conf)
	// 初始化 event handlers， 通过WSS event处理价格、订单相关消息
	eventHandler.Init(conf, orderHandler)

	// 初始化 动态配置
	InitDynamicConfig(conf)
}
func Start() {
	// 启动websockets
	eventHandler.Start()

	// 确保 ws 正常启动和监听
	time.Sleep(5 * time.Second)

	// 获取账户初始状态
	UpdateAccount()

	// 每100ms计算一遍波动参数
	go common.Timer(100*time.Millisecond, UpdateDynamicConfigs)

	// 每1s更新一遍订单
	go common.Timer(1*time.Second, UpdateOrders)

	//// 每3秒钟取消距离较远的订单
	//go common.Timer(3*time.Second, CancelFarOrders)
	//
	//// 每3s取消一次间距较近的订单
	//go common.Timer(3*time.Second, CancelCloseDistanceOrders)

	// 每分钟更新一次账户状态
	go common.Timer(60*time.Second, UpdateAccount)

	// 每100ms 检查一下价格，如果指定时间价格没有更新取消挂单或者停掉服务，避免造成亏损
	go common.Timer(100*time.Millisecond, CheckStatus)

	// 每分钟执行一次，统计除了币安下单 ERROR 之外的 ERROR 信息，超过配置次数就报警
	go common.Timer(1*time.Minute, CheckErrors)

}
func ExitProcess() {
	// 取消所有订单, 不判断本地orders
	logger.Info("DelContext cancel all orders")
	for _, symbolContext := range ctxt.symbolContexts {
		symbolContext.Risk = 1
	}
	orderHandler.CancelAllOrdersWithoutCheckOrderBook()

	// 停止webscoket
	eventHandler.Stop()
	os.Exit(1)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("Usage: %s config_file\n", os.Args[0])
		os.Exit(1)
	}

	// 监听退出消息，并调用ExitProcess进行处理
	common.RegisterExitSignal(ExitProcess)

	// 加载配置文件
	cfg = *config.LoadConfig(os.Args[1])

	// 设置日志级别, 并初始化日志
	logger.InitLogger(cfg.LogPath, cfg.LogLevel)

	Init(&cfg)

	Start()

	// 阻塞主进程
	for {
		time.Sleep(24 * time.Hour)
	}
}
