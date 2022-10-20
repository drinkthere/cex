package client

import (
	"cex/common"
	"cex/common/logger"
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/shopspring/decimal"

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

func (cli *BinanceFuturesClient) GetDepthPriceInfo(symbol string) (*futures.DepthResponse, error) {
	resp, err := cli.orderClient.NewDepthService().Symbol(symbol).Limit(20).Do(context.Background())
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}
	return resp, nil
}

type BinanceFuturesWSClient struct {
	WSClient
	httpClient     BinanceFuturesClient
	priceWSHandler PriceProcessHandler
	orderWSHandler OrderProcessHandler
	errorHandler   ErrorHandler

	symbols []string //多币种
	//listenKey              string
	bookTickerStopC           []chan struct{} // bookTicker channel
	depthStopC                []chan struct{} // bookTicker channel
	bookTickerLastUpdateIDMap sync.Map        // 上一次symbol 更新 bookTicker 价格的 id
	depthLastUpdateIDMap      sync.Map        // 上一次更新depth 价格的 id

	// depth data
	Asks sync.Map // symbol => []DepthPriceItem
	Bids sync.Map
}

func (cli *BinanceFuturesWSClient) Init(config Config) bool {
	cli.symbols = config.Symbols
	for _, symbol := range cli.symbols {
		cli.bookTickerLastUpdateIDMap.Store(symbol, int64(0))
		cli.depthLastUpdateIDMap.Store(symbol, int64(0))
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

func (cli *BinanceFuturesWSClient) SetHttpClient(client BinanceFuturesClient) {
	cli.httpClient = client // 获取订单消息时，更新listenKey需要用到
}

func (cli *BinanceFuturesWSClient) StartWS() bool {
	for _, symbol := range cli.symbols {
		// 启动 bookTicker
		cli.bookTickerWSConnect(symbol)
		time.Sleep(30 * time.Millisecond)

		// 启动 depth
		cli.depthWSConnect(symbol)
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

// 增量深度信息
func (cli *BinanceFuturesWSClient) depthWSConnect(symbol string) bool {
	// 清空
	var askItems []DepthPriceItem
	var bidItems []DepthPriceItem
	cli.Asks.Store(symbol, askItems)
	cli.Bids.Store(symbol, bidItems)

	_, stopC, err := futures.WsDiffDepthServeWithRate(symbol, 100*time.Millisecond, cli.depthMsgHandler, cli.depthErrorHandler)
	if err != nil {
		logger.Error("failed to establish connection with depth websocket, message is %s", err.Error())
		return false
	}
	logger.Info("depth WS is established")
	cli.depthStopC = append(cli.depthStopC, stopC)

	// 获取全量数据
	cli.getFuturesDepthPrice(symbol)

	return true
}

func (cli *BinanceFuturesWSClient) depthMsgHandler(event *futures.WsDepthEvent) {
	if event.Symbol == "" || !common.InArray(event.Symbol, cli.symbols) {
		return
	}
	depthLastUpdateID, ok := cli.depthLastUpdateIDMap.Load(event.Symbol)
	if !ok || depthLastUpdateID.(int64) >= event.LastUpdateID {
		return
	}

	var priceResp PriceWSResponse
	symbol := event.Symbol

	cli.parseDepthData(symbol, event.Bids, event.Asks, event.LastUpdateID)

	askPriceItems, _ := cli.Asks.Load(symbol)
	askItems := askPriceItems.([]DepthPriceItem)
	bidPriceItems, _ := cli.Asks.Load(symbol)
	bidItems := bidPriceItems.([]DepthPriceItem)

	if len(askItems) > 0 || len(bidItems) > 0 {
		cli.depthLastUpdateIDMap.Store(event.Symbol, event.LastUpdateID)
		priceResp.Exchange = "Binance"
		priceResp.MsgType = "futuresDepth"
		priceResp.TimeStamp = event.Time
		priceResp.Asks = askItems
		priceResp.Bids = bidItems
		priceResp.Symbol = event.Symbol
		cli.priceWSHandler(&priceResp)
	}
}

func (cli *BinanceFuturesWSClient) parseDepthData(symbol string, bids []futures.Bid, asks []futures.Ask, LastUpdateID int64) {
	for _, bid := range bids {
		bidPrice, err := decimal.NewFromString(bid.Price)
		if err != nil {
			logger.Error("Binance futures depthMsgHandler convert bid price=%s to float64 error, message: %s",
				bid.Price, err.Error())
			return
		}
		bidQty, err := decimal.NewFromString(bid.Quantity)
		if err != nil {
			logger.Error("Binance futures depthMsgHandler convert bid quantity=%s to float64 error, message: %s",
				bid.Quantity, err.Error())
			return
		}
		if bidPrice.IsPositive() && !bidQty.IsNegative() {
			bidPriceItems, _ := cli.Bids.Load(symbol)
			bidItems := bidPriceItems.([]DepthPriceItem)
			item := DepthPriceItem{
				Price:        bidPrice,
				Volume:       bidQty,
				LastUpdateID: LastUpdateID}
			cli.Bids.Store(symbol, processOneStepDepthBinance(bidItems, item, "bid"))
		}
	}
	// logger.Info("process_bid item: %d %+v", len(bids), bids)
	// logger.Info("process_bid item: %d %+v", len(cli.Bids), cli.Bids)

	for _, ask := range asks {
		askPrice, err := decimal.NewFromString(ask.Price)
		if err != nil {
			logger.Error("Binance depthMsgHandler convert ask price=%s to float64 error, message: %s",
				ask.Price, err.Error())
			return
		}
		askQty, err := decimal.NewFromString(ask.Quantity)
		if err != nil {
			logger.Error("Binance depthMsgHandler convert ask quantity=%s to float64 error, message: %s",
				ask.Quantity, err.Error())
			return
		}
		if askPrice.IsPositive() && !askQty.IsNegative() {
			askPriceItems, _ := cli.Asks.Load(symbol)
			askItems := askPriceItems.([]DepthPriceItem)
			item := DepthPriceItem{
				Price:        askPrice,
				Volume:       askQty,
				LastUpdateID: LastUpdateID}
			cli.Asks.Store(symbol, processOneStepDepthBinance(askItems, item, "ask"))
		}
	}
	// logger.Info("process_ask item: %d %+v", len(asks), asks)
	// logger.Info("process_ask item: %d %+v", len(cli.Asks), cli.Asks)
}

func (cli *BinanceFuturesWSClient) depthErrorHandler(err error) {
	logger.Error("Binance futures depthErrorHandler emit, message: %s", err.Error())
	// 重试链接
	logger.Warn("Binance futures depthErrorHandler reconnect")
	if cli.depthStopC != nil {
		for _, stopC := range cli.depthStopC {
			stopC <- struct{}{}
		}
	}
	time.Sleep(30 * time.Millisecond)
	for _, symbol := range cli.symbols {
		cli.depthWSConnect(symbol)
		time.Sleep(30 * time.Millisecond)
	}
}

func (cli *BinanceFuturesWSClient) getFuturesDepthPrice(symbol string) {
	resp, err := cli.httpClient.GetDepthPriceInfo(symbol)
	if err != nil {
		logger.Error(err.Error())
		return
	}

	cli.parseDepthData(symbol, resp.Bids, resp.Asks, resp.LastUpdateID)
}

func (cli *BinanceFuturesWSClient) StopWS() bool {
	// 关闭 bookTicker ws
	if cli.bookTickerStopC != nil {
		for _, stopC := range cli.bookTickerStopC {
			stopC <- struct{}{}
		}
	}

	// 关闭 depth ws
	if cli.depthStopC != nil {
		for _, stopC := range cli.depthStopC {
			stopC <- struct{}{}
		}
	}

	return true
}
