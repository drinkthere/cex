package client

import (
	"cex/common"
	"cex/common/logger"
	"strconv"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2/futures"
)

type BinanceFuturesClient struct {
	OrderClient
	Name        string
	orderClient *futures.Client
}

func (cli *BinanceFuturesClient) Init(config Config) bool {
	cli.Name = "BinanceFutures"
	cli.orderClient = futures.NewClient(config.AccessKey, config.SecretKey)
	return true
}

type BinanceFuturesWSClient struct {
	WSClient
	priceWSHandler PriceProcessHandler
	orderWSHandler OrderProcessHandler
	errorHandler   ErrorHandler

	symbols []string //多币种
	//listenKey              string
	bookTickerStopC           []chan struct{} // bookTicker channel
	bookTickerLastUpdateIDMap sync.Map        // 上一次symbol 更新 bookTicker 价格的 id
}

func (cli *BinanceFuturesWSClient) Init(config Config) bool {
	cli.symbols = config.Symbols
	for _, symbol := range cli.symbols {
		cli.bookTickerLastUpdateIDMap.Store(symbol, int64(0))
	}
	return true
}
func (cli *BinanceFuturesWSClient) SetPriceHandler(handler PriceProcessHandler, errHandler ErrorHandler) {
	cli.priceWSHandler = handler
	cli.errorHandler = errHandler
}

func (cli *BinanceFuturesWSClient) SetOrderHandler(handler OrderProcessHandler) {
	cli.orderWSHandler = handler
}

func (cli *BinanceFuturesWSClient) StartWS() bool {
	// 启动 bookTicker
	for _, symbol := range cli.symbols {
		cli.bookTickerWSConnect(symbol)
		time.Sleep(30 * time.Millisecond)
	}
	return true
}

func (cli *BinanceFuturesWSClient) bookTickerWSConnect(symbol string) bool {
	_, stopC, err := futures.WsBookTickerServe(symbol, cli.bookTickerMsgHandler, cli.bookTickerErrorHandler)
	if err != nil {
		logger.Error("failed to establish connection with binance futures bookTicker websocket, message is %s", err.Error())
		return false
	}
	logger.Info("Binance futures bookTicker WS is established, symbol:%s", symbol)
	cli.bookTickerStopC = append(cli.bookTickerStopC, stopC)
	return true
}

func (cli *BinanceFuturesWSClient) bookTickerMsgHandler(event *futures.WsBookTickerEvent) {
	if event.Symbol == "" || !common.InArray(event.Symbol, cli.symbols) {
		return
	}

	bookTickerLastUpdateID, ok := cli.bookTickerLastUpdateIDMap.Load(event.Symbol)
	if !ok || bookTickerLastUpdateID.(int64) >= event.UpdateID {
		return
	}

	var priceResp PriceWSResponse
	bidPrice, err := strconv.ParseFloat(event.BestBidPrice, 64)
	if err != nil {
		logger.Error("Binance futures bookTickerHandler convert bid price=%s to float64 error, message: %s",
			event.BestBidPrice, err.Error())
		return
	}

	bidQty, err := strconv.ParseFloat(event.BestBidQty, 64)
	if err != nil {
		logger.Error("Binance futures bookTickerHandler convert bid quantity=%s to float64 error, message: %s",
			event.BestBidQty, err.Error())
		return
	}
	if bidPrice > 0 && bidQty > 0 {
		item := PriceItem{
			Price:     bidPrice,
			Volume:    bidQty,
			Direction: "buy"}
		priceResp.Items = append(priceResp.Items, item)
	}

	askPrice, err := strconv.ParseFloat(event.BestAskPrice, 64)
	if err != nil {
		logger.Error("Binance bookTickerHandler convert ask price=%s to float64 error, message: %s",
			event.BestAskPrice, err.Error())
		return
	}
	askQty, err := strconv.ParseFloat(event.BestAskQty, 64)
	if err != nil {
		logger.Error("Binance bookTickerHandler convert ask quantity=%s to float64 error, message: %s",
			event.BestAskQty, err.Error())
		return
	}
	if askPrice > 0 && askQty > 0 {
		item := PriceItem{
			Price:     askPrice,
			Volume:    askQty,
			Direction: "sell"}
		priceResp.Items = append(priceResp.Items, item)
	}

	if len(priceResp.Items) > 0 {
		cli.bookTickerLastUpdateIDMap.Store(event.Symbol, event.UpdateID)
		priceResp.Exchange = "Binance"
		priceResp.MsgType = "futuresBookTicker"
		priceResp.TimeStamp = event.Time
		priceResp.Symbol = event.Symbol
		cli.priceWSHandler(&priceResp)
	}
}

func (cli *BinanceFuturesWSClient) bookTickerErrorHandler(err error) {
	logger.Error("Binance futures bookTickerErrorHandler emit, message: %s", err.Error())
	// 重试链接
	logger.Warn("Binance futures  bookTicker reconnect")

	if cli.bookTickerStopC != nil {
		for _, stopC := range cli.bookTickerStopC {
			stopC <- struct{}{}
		}
	}
	time.Sleep(30 * time.Millisecond)
	for _, symbol := range cli.symbols {
		cli.bookTickerWSConnect(symbol)
		time.Sleep(30 * time.Millisecond)
	}
}

func (cli *BinanceFuturesWSClient) StopWS() bool {
	// 关闭 bookTicker ws
	if cli.bookTickerStopC != nil {
		for _, stopC := range cli.bookTickerStopC {
			stopC <- struct{}{}
		}
	}
	return true
}
