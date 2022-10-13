package common

import (
	"cex/common/logger"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var lastSendTimeStamp = GetTimestampInMS()

func SendMessge(bot *tgbotapi.BotAPI, chantID int64, message string) {
	SendMessgeWithInterval(bot, chantID, message, 0)
}

// 发送消息，发送间隔不小于interval
// @param interval: 最小发送间隔，单位ms
func SendMessgeWithInterval(bot *tgbotapi.BotAPI, chantID int64, message string, interval int64) {
	if bot == nil {
		return
	}
	timeStamp := GetTimestampInMS()
	// 最多interval发一次消息
	if timeStamp-lastSendTimeStamp < interval {
		return
	}
	lastSendTimeStamp = timeStamp
	msg := tgbotapi.NewMessage(chantID, message)
	_, err := bot.Send(msg)
	if err != nil {
		logger.Error("send Alarm via telegram failed. message is %s, error is %s", message, err.Error())
	}
}
