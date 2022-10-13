package common

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

type SimpleFunc func()

func Timer(interval time.Duration, RunFunc SimpleFunc) {
	for {
		RunFunc()
		time.Sleep(interval)
	}
}

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
// 格式：exchange_symbol_product
// product取值：spot/futures/delivery
func FormatPriceName(exchange string, symbol string, product string) string {
	return exchange + "_" + symbol + "_" + product
}

// 将币本位的symbol格式化成U本位的symbol, quoteAsset: BUSD | USDT
func FormatFuturesSymbol(deliverySymbol string, quoteAsset string) string {
	deliverySymbol = strings.ToUpper(deliverySymbol)
	strs := strings.Split(deliverySymbol, "_")
	deliverySymbol = strings.Replace(strs[0], "USD", quoteAsset, -1)
	return deliverySymbol
}

// 将币本位的symbol格式化成现货的symbol, quoteAsset: BUSD | USDT
func FormatSpotSymbol(deliverySymbol string, quoteAsset string) string {
	deliverySymbol = strings.ToUpper(deliverySymbol)
	strs := strings.Split(deliverySymbol, "_")
	deliverySymbol = strings.Replace(strs[0], "USD", quoteAsset, -1)
	return deliverySymbol
}

// 获取ms格式的时间戳
func GetTimestampInMS() int64 {
	return time.Now().UnixNano() / 1e6
}

// 获取对冲OrderType
func GetHedgeOrderType(orderType string) (result string) {
	if strings.ToLower(orderType) == "sell" {
		result = "buy"
	} else if strings.ToLower(orderType) == "buy" {
		result = "sell"
	}
	return result
}

// 生成Client Order ID
var gClientOrderID = GetTimestampInMS()

func GetClientOrderID() string {
	atomic.AddInt64(&gClientOrderID, 1)
	return strconv.FormatInt(atomic.LoadInt64(&gClientOrderID), 10)
}

func InArray(target string, strArray []string) bool {
	for _, element := range strArray {
		if target == element {
			return true
		}
	}
	return false
}
