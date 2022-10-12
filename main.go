package main

import (
	"cex/common"
	"cex/config"
	"fmt"
	"os"
)

// 全局变量
var cfg config.Config

func ExitProcess() {
	//
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

	fmt.Printf("cfg is %#v", cfg)
}
