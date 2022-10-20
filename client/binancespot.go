package client

import (
	"cex/common"
	"cex/common/logger"
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/shopspring/decimal"

	"github.com/adshao/go-binance/v2"
)

type BinanceSpotClient struct {
	OrderClient
	Name        string
	orderClient *binance.Client
}

func (cli *BinanceSpotClient) Init(config Config) bool {
	cli.Name = "BinanceSpot"
	cli.orderClient = binance.NewClient(config.AccessKey, config.SecretKey)
	return true
}

func (cli *BinanceSpotClient) GetAccount() *binance.Account {
	account, err := cli.orderClient.NewGetAccountService().Do(context.Background())
	if err != nil {
		logger.Error("get delivery account failed, message is %s", err.Error())
	}
	return account
}

func (cli *BinanceSpotClient) GetDepthPriceInfo(symbol string) (*binance.DepthResponse, error) {
	resp, err := cli.orderClient.NewDepthService().Symbol(symbol).Limit(20).Do(context.Background())
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}
	return resp, nil
}

func (cli *BinanceSpotClient) PlaceMarketOrder(order *common.Order) string {
	if order.ClientOrderID == "" {
		order.ClientOrderID = common.GetClientOrderID()
	}
	symbol := common.FormatSpotSymbol(order.Symbol, order.QuoteAsset)
	fQuantity := strconv.FormatFloat(order.OrderVolume, 'f', order.Precision[0], 64)

	logger.Info("BinanceSpotPlaceOrder: symbol=%s, side=%s, quantity=%f, clientID=%s", symbol, order.OrderType, fQuantity, order.ClientOrderID)
	if order.OrderType == "buy" {
		res, err := cli.orderClient.NewCreateOrderService().
			NewClientOrderID(order.ClientOrderID).
			Symbol(symbol).
			Side(binance.SideTypeBuy).
			Type(binance.OrderTypeMarket).
			Quantity(fQuantity).
			Do(context.Background())
		if err != nil {
			logger.Error("BinanceSpotPlaceOrder: error，side=buy, amount=%s, symbol=%s, message is %s", fQuantity, symbol, err.Error())
			return ""
		}
		return strconv.FormatInt(res.OrderID, 10)
	} else if order.OrderType == "sell" {
		res, err := cli.orderClient.NewCreateOrderService().
			NewClientOrderID(order.ClientOrderID).
			Symbol(symbol).
			Side(binance.SideTypeSell).
			Type(binance.OrderTypeMarket).
			Quantity(fQuantity).
			Do(context.Background())
		if err != nil {
			logger.Error("BinanceSpotPlaceOrder error: side=sell, amount=%s, symbol=%s, message is %s", fQuantity, symbol, err.Error())
			return ""
		}
		return strconv.FormatInt(res.OrderID, 10)
	}
	return ""
}

// -------------------------以下是websocket相关内容-----------

type BinanceSpotWSClient struct {
	WSClient
	httpClient     BinanceSpotClient
	symbols        []string //多币种
	priceWSHandler PriceProcessHandler
	orderWSHandler OrderProcessHandler
	errorHandler   ErrorHandler

	bookTickerStopC []chan struct{} // bookTicker channel
	depthStopC      []chan struct{} // bookTicker channel

	bookTickerLastUpdateIDMap sync.Map // 上一次symbol 更新 bookTicker 价格的 id
	depthLastUpdateIDMap      sync.Map // 上一次更新depth 价格的 id

	// depth data
	Asks sync.Map // symbol => []DepthPriceItem
	Bids sync.Map
}

func (cli *BinanceSpotWSClient) Init(config Config) bool {
	cli.symbols = config.Symbols
	for _, symbol := range cli.symbols {
		cli.bookTickerLastUpdateIDMap.Store(symbol, int64(0))
		cli.depthLastUpdateIDMap.Store(symbol, int64(0))
	}
	return true
}

func (cli *BinanceSpotWSClient) SetPriceHandler(handler PriceProcessHandler, errHandler ErrorHandler) {
	cli.priceWSHandler = handler
	cli.errorHandler = errHandler
}

func (cli *BinanceSpotWSClient) SetOrderHandler(handler OrderProcessHandler) {
	cli.orderWSHandler = handler
}

func (cli *BinanceSpotWSClient) SetHttpClient(client BinanceSpotClient) {
	cli.httpClient = client
}

func (cli *BinanceSpotWSClient) StartWS() bool {
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

func (cli *BinanceSpotWSClient) bookTickerWSConnect(symbol string) bool {
	_, stopC, err := binance.WsBookTickerServe(symbol, cli.bookTickerMsgHandler, cli.bookTickerErrorHandler)
	if err != nil {
		logger.Error("failed to establish connection with binance spot bookTicker websocket, message is %s", err.Error())
		return false
	}
	logger.Info("Binance spot bookTicker WS is established, symbol=%s", symbol)
	cli.bookTickerStopC = append(cli.bookTickerStopC, stopC)

	// 获取全量数据
	cli.getSpotDepthPrice(symbol)
	return true
}

func (cli *BinanceSpotWSClient) bookTickerMsgHandler(event *binance.WsBookTickerEvent) {
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
		logger.Error("Binance spot bookTickerHandler convert bid price=%s to float64 error, message: %s",
			event.BestBidPrice, err.Error())
		return
	}

	bidQty, err := strconv.ParseFloat(event.BestBidQty, 64)
	if err != nil {
		logger.Error("Binance spot bookTickerHandler convert bid quantity=%s to float64 error, message: %s",
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
		logger.Error("Binance spot bookTickerHandler convert ask price=%s to float64 error, message: %s",
			event.BestAskPrice, err.Error())
		return
	}
	askQty, err := strconv.ParseFloat(event.BestAskQty, 64)
	if err != nil {
		logger.Error("Binance spot bookTickerHandler convert ask quantity=%s to float64 error, message: %s",
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
		priceResp.MsgType = "spotBookTicker"
		priceResp.TimeStamp = time.Now().UnixNano() / 1e6
		priceResp.Symbol = event.Symbol
		cli.priceWSHandler(&priceResp)
	}
}

func (cli *BinanceSpotWSClient) bookTickerErrorHandler(err error) {
	logger.Error("Binance spot bookTickerErrorHandler emit, message: %s", err.Error())
	// 重试链接
	logger.Warn("Binance spot bookTicker reconnect")
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
func (cli *BinanceSpotWSClient) depthWSConnect(symbol string) bool {
	// 清空
	var askItems []DepthPriceItem
	var bidItems []DepthPriceItem
	cli.Asks.Store(symbol, askItems)
	cli.Bids.Store(symbol, bidItems)

	_, stopC, err := binance.WsDepthServe100Ms(symbol, cli.depthMsgHandler, cli.depthErrorHandler)
	if err != nil {
		logger.Error("failed to establish connection with spot depth websocket, message is %s", err.Error())
		return false
	}
	logger.Info("spot depth WS is established")
	cli.depthStopC = append(cli.depthStopC, stopC)

	// 获取全量数据
	cli.getSpotDepthPrice(symbol)
	return true
}

func (cli *BinanceSpotWSClient) depthMsgHandler(event *binance.WsDepthEvent) {
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
		priceResp.MsgType = "spotDepth"
		priceResp.TimeStamp = event.Time
		priceResp.Asks = askItems
		priceResp.Bids = bidItems
		priceResp.Symbol = event.Symbol
		cli.priceWSHandler(&priceResp)
	}
}

func (cli *BinanceSpotWSClient) parseDepthData(symbol string, bids []futures.Bid, asks []futures.Ask, LastUpdateID int64) {
	for _, bid := range bids {
		bidPrice, err := decimal.NewFromString(bid.Price)
		if err != nil {
			logger.Error("Binance spot depthMsgHandler convert bid price=%s to float64 error, message: %s",
				bid.Price, err.Error())
			return
		}
		bidQty, err := decimal.NewFromString(bid.Quantity)
		if err != nil {
			logger.Error("Binance spot depthMsgHandler convert bid quantity=%s to float64 error, message: %s",
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

func (cli *BinanceSpotWSClient) depthErrorHandler(err error) {
	logger.Error("Binance spot depthErrorHandler emit, message: %s", err.Error())
	// 重试链接
	logger.Warn("Binance spot depthErrorHandler reconnect")
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

func (cli *BinanceSpotWSClient) getSpotDepthPrice(symbol string) {
	resp, err := cli.httpClient.GetDepthPriceInfo(symbol)
	if err != nil {
		logger.Error(err.Error())
		return
	}
	cli.parseDepthData(symbol, resp.Bids, resp.Asks, resp.LastUpdateID)
}

func (cli *BinanceSpotWSClient) StopWS() bool {
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
