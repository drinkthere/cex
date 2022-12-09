package main

import (
	"cex/common"
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/adshao/go-binance/v2/futures"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

var (
	client      *futures.Client
	telegramBot *tgbotapi.BotAPI // 电报机器人
	chatID      int64
	threshold   float64
)

func processing() {
	fmt.Printf("processiong at %s", time.Now().Format("2006-01-02 15:04:05"))
	infos, err := client.NewPremiumIndexService().Do(context.Background())
	if err != nil {
		fmt.Printf("get binance exchange info failed, error is %s", err.Error())
		return
	}
	text := ""
	for _, info := range infos {
		// 过滤掉交割合约
		if strings.Contains(info.Symbol, "_") {
			continue
		}

		fundingRate, _ := strconv.ParseFloat(info.LastFundingRate, 64)
		// 如果资金费率的绝对值大于threshold 电报报警
		if math.Abs(fundingRate) >= threshold {
			tm := time.Unix(0, info.NextFundingTime*int64(time.Millisecond))
			nextFundingTime := tm.Format("2006-01-02 15:04:05")

			msg := fmt.Sprintf("%s的资金费率%s超过了%.2f%%， 下次结算时间是：%s\n", info.Symbol, info.LastFundingRate, 100*threshold, nextFundingTime)
			text += msg
		}
	}
	if len(text) > 0 {
		// 发送电报

		msg := tgbotapi.NewMessage(chatID, text)
		_, err = telegramBot.Send(msg)
		if err != nil {
			fmt.Printf("send stat message failed, error is %s", err.Error())
		}
	}
}

func main() {
	err := godotenv.Load()
	if err != nil {
		panic(err)
	}

	chatIDStr := os.Getenv("TELEGRAM_CHAT_ID")
	chatID, _ = strconv.ParseInt(chatIDStr, 10, 64)

	thresholdStr := os.Getenv("THRESHOLD")
	threshold, _ = strconv.ParseFloat(thresholdStr, 64)

	client = futures.NewClient(os.Getenv("BN_API_KEY"), "BN_SECRET_KEY")

	// 初始化 telegramBot
	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_API_TOKEN"))
	if err != nil {
		fmt.Printf("init telegram bot failed, error is %s", err.Error())
		os.Exit(1)
	}
	telegramBot = bot

	// 每小时处理一次
	// processing()
	go common.Timer(60*time.Minute, processing)

	// 阻塞主进程
	for {
		time.Sleep(24 * time.Hour)
	}
}
