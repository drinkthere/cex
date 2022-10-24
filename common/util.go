package common

import (
	"cex/common/logger"
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

func MaxFloat64(list []float64) (max float64) {
	max = list[0]
	for _, v := range list {
		if v > max {
			max = v
		}
	}
	return
}

func MinFloat64(list []float64) (min float64) {
	min = list[0]
	for _, v := range list {
		if v < min {
			min = v
		}
	}
	return
}

// 判断是否是结算时间
// @param timeStamp: 当前时间戳，单位s
func IsSettlement(timeStamp int64) bool {
	// 不用判断时区，因为每8小时结算一次，所以东八区结算时间一样
	// 加60是为了方便后面比较
	tmp := (timeStamp + 60) % (60 * 60 * 24)
	if (tmp >= 0 && tmp <= 3*60) ||
		(tmp >= 8*60*60 && tmp <= 8*60*60+3*60) ||
		(tmp >= 16*60*60 && tmp <= 16*60*60+3*60) {
		return true
	}
	return false
}

// 测速
func TimeCost(start time.Time, remark string) {
	tc := time.Since(start)
	logger.Info("%s|%d", remark, tc.Nanoseconds())
}
