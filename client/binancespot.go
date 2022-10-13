package client

import (
	"cex/common"
	"cex/common/logger"
	"context"
	"strconv"
	"sync"
	"time"

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

type BinanceSpotWSClient struct {
	WSClient
	symbols        []string //多币种
	priceWSHandler PriceProcessHandler
	orderWSHandler OrderProcessHandler
	errorHandler   ErrorHandler

	bookTickerStopC           []chan struct{} // bookTicker channel
	bookTickerLastUpdateIDMap sync.Map        // 上一次symbol 更新 bookTicker 价格的 id
}

func (cli *BinanceSpotWSClient) Init(config Config) bool {
	cli.symbols = config.Symbols
	for _, symbol := range cli.symbols {
		cli.bookTickerLastUpdateIDMap.Store(symbol, int64(0))
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

func (cli *BinanceSpotWSClient) StartWS() bool {
	// 启动 bookTicker
	for _, symbol := range cli.symbols {
		cli.bookTickerWSConnect(symbol)
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

func (cli *BinanceSpotWSClient) StopWS() bool {
	// 关闭 bookTicker ws
	if cli.bookTickerStopC != nil {
		for _, stopC := range cli.bookTickerStopC {
			stopC <- struct{}{}
		}
	}
	return true
}
