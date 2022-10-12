package common

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type SimpleFunc func()

func RegisterExitSignal(exitFunc SimpleFunc) {
	c := make(chan os.Signal, 5)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		for s := range c {
			switch s {
			case syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
				exitFunc()
			default:
				fmt.Println("其他信号:", s)
			}
		}
	}()
}

// 用来唯一标识一个价格
// 格式：exchange_futureSymbol_product
// product取值：spot/futures/delivery
func FormatPriceName(exchange string, symbol string, product string) string {
	return exchange + "_" + symbol + "_" + product
}

// 将币本位的symbol格式化成U本位的symbol
func FormatFuturesSymbol(deliverySymbol string, quoteAsset string) string {
	deliverySymbol = strings.ToUpper(deliverySymbol)
	strs := strings.Split(deliverySymbol, "_")
	deliverySymbol = strings.Replace(strs[0], "USD", quoteAsset, -1)
	return deliverySymbol
}

// 获取ms格式的时间戳
func GetTimestampInMS() int64 {
	return time.Now().UnixNano() / 1e6
}
