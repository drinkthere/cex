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
	config.InitDynamicConfig(conf)

	// 获取账户初始状态
	// ShowBalance()

	// 初始化统计信息
}
func Start() {
	// 启动websockets
	eventHandler.Start()

}
func ExitProcess() {
	// 停止webscoket
	eventHandler.Stop()
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
