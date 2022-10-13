package main

import (
	"cex/common"
	"cex/common/logger"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// check 程序运行状态
func CheckStatus() {

	timeStamp := common.GetTimestampInMS()

	// 超过1秒没有更新，停止挂单
	for _, symbol := range ctxt.Symbols {
		symbolContext := ctxt.GetSymbolContext(symbol)
		timeDiff := timeStamp - symbolContext.LastUpdateTime
		logger.Debug("timediff:%d", timeStamp-symbolContext.LastUpdateTime)
		if timeDiff > 1000 {
			logger.Debug("%s Price not update in 1s. TimeStamp: %d", symbol, symbolContext.LastUpdateTime)
			// 超过10s没有更新，取消订单
			if timeDiff > 10000 {
				logger.Warn("%s Price not update in 10s. CancelAllOrders: %d", symbol, symbolContext.LastUpdateTime)
				orderHandler.CancelAllOrdersWithSymbol(symbol)
			}
			if symbolContext.Risk == 3 {
				continue
			}
			symbolContext.Risk = 3
			common.SendMessge(ctxt.TelegramBot, cfg.TgChatID, "停止挂单，原因:价格超过1s没有更新")
		} else {
			if symbolContext.Risk == 3 {
				symbolContext.Risk = 0
				logger.Warn("%s Price updated, set risk to 0.", symbol)
				common.SendMessge(ctxt.TelegramBot, cfg.TgChatID, "继续挂单，价格已回复更新")
			}
		}
	}
}

func CheckErrors() {
	now := time.Now()
	timeStamp := now.Unix()
	if common.IsSettlement(timeStamp) {
		return
	}
	// 获取服务器设置的时区
	local, err := time.LoadLocation("Local")
	if err != nil {
		logger.Error("Load timezone failed, message is %s", err.Error())
		return
	}

	lastMinute := now.In(local).Format("2006-01-02T15:04")
	count := GetErrorCountFromLog(cfg.LogPath, lastMinute)
	logger.Debug("last minute=%s, error numbers=%d", lastMinute, count)
	if count > cfg.MaxErrorsPerMinute {
		logger.Error("ERROR logs nums over max num, stop placing order from Huobi")
		ctxt.Risk = 1
		// 取消所有挂单
		orderHandler.CancelAllOrders()
		common.SendMessge(ctxt.TelegramBot, cfg.TgChatID, "停止挂单，原因:"+lastMinute+"错误次数超过限制")
	}
}

func GetErrorCountFromLog(logFile string, lastMinute string) int64 {
	shell := fmt.Sprintf("/bin/grep '%s' %s | grep ERROR | grep -v \"Price not update\" | wc -l", lastMinute, logFile)

	// 通过 grep 获取 Error 日志出现次数
	cmd := exec.Command("/bin/sh", "-c", shell)
	countRaw, err := cmd.Output()
	if err != nil {
		logger.Error("Failed to stat ERROR logs, message is %s", err.Error())
		return 0
	}

	// 默认执行结果里面有"\n"，需要去掉
	countStr := strings.Trim(string(countRaw), "\n")
	countStr = strings.Trim(countStr, " ")
	count, err := strconv.ParseInt(countStr, 10, 64)
	if err != nil {
		logger.Error("CheckErrors failed to convert count to int, count is %s, message is %s", countStr, err.Error())
		return 0
	}

	return count
}
