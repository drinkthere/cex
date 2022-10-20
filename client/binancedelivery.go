package client

import (
	"cex/common"
	"cex/common/logger"
	"context"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/shopspring/decimal"

	"github.com/adshao/go-binance/v2/delivery"
	"golang.org/x/time/rate"
)

type BinanceDeliveryClient struct {
	OrderClient
	Name         string
	orderClient  *delivery.Client
	limiter      *rate.Limiter
	limitProcess int
	precMap      map[string]int
	qtyMap       map[string]int
}

func (cli *BinanceDeliveryClient) Init(config Config) bool {
	cli.Name = "BinanceDelivery"
	cli.orderClient = delivery.NewClient(config.AccessKey, config.SecretKey)
	limit := rate.Every(1 * time.Second / time.Duration(config.APILimit))
	cli.limiter = rate.NewLimiter(limit, 60)
	cli.limitProcess = config.LimitProcess
	cli.ExchangeInfo()
	return true
}

// 获取下单精度
func (cli *BinanceDeliveryClient) ExchangeInfo() {
	resp, err := cli.orderClient.NewExchangeInfoService().Do(context.Background())
	if err != nil {
		logger.Error("Get ExchangeInfo failed, message is %s", err.Error())
	}
	precMap := map[string]int{}
	qtyMap := map[string]int{}
	if resp.Symbols != nil {
		for i := 0; i < len(resp.Symbols); i++ {
			symbol := resp.Symbols[i].Symbol

			prec := resp.Symbols[i].PricePrecision
			precMap[symbol] = prec

			qty := resp.Symbols[i].QuantityPrecision
			qtyMap[symbol] = qty
		}
	}

	// fix: some symbol is error
	precMap["AAVEUSD_PERP"] = 2
	precMap["AXSUSD_PERP"] = 2
	precMap["APEUSD_PERP"] = 3

	cli.precMap = precMap
	cli.qtyMap = qtyMap
}

// 设置杠杆
func (cli *BinanceDeliveryClient) ChangeLeverage(symbol string, leverage int) {
	logger.Debug("==change symbol=%s leverage to %d", symbol, leverage)
	resp, err := cli.orderClient.NewChangeLeverageService().Symbol(symbol).Leverage(leverage).Do(context.Background())
	if err != nil {
		logger.Error("change leverage failed, message is %s", err.Error())
	}
	logger.Debug("==symbol=%s's leverage has been changed to %+v", symbol, resp)
}

func (cli *BinanceDeliveryClient) GetListenKey() string {
	listenKey, err := cli.orderClient.NewStartUserStreamService().Do(context.Background())
	if err != nil {
		logger.Error(err.Error())
		return ""
	}
	return listenKey
}

func (cli *BinanceDeliveryClient) KeepAliveListenKey(listenKey string) {
	err := cli.orderClient.NewKeepaliveUserStreamService().ListenKey(listenKey).Do(context.Background())
	if err != nil {
		logger.Error(err.Error())
	}
}

func (cli *BinanceDeliveryClient) GetAccount() *delivery.Account {
	account, err := cli.orderClient.NewGetAccountService().Do(context.Background())
	if err != nil {
		logger.Error("get delivery account failed, message is %s", err.Error())
	}
	return account
}

func (cli *BinanceDeliveryClient) GetDepthPriceInfo(symbol string) (*delivery.DepthResponse, error) {
	resp, err := cli.orderClient.NewDepthService().Symbol(symbol).Limit(20).Do(context.Background())
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}
	return resp, nil
}

// 创建订单，如果成功返回orderID，否则返回空
// 限价单，GTC
func (cli *BinanceDeliveryClient) PlaceOrderGTX(order *common.Order) string {
	if !cli.checkLimit(1) {
		return ""
	}
	if order.ClientOrderID == "" {
		order.ClientOrderID = common.GetClientOrderID()
	}

	fPrice := strconv.FormatFloat(order.OrderPrice, 'f', cli.precMap[order.Symbol], 64)
	fQuantity := strconv.FormatFloat(order.OrderVolume, 'f', cli.qtyMap[order.Symbol], 64)

	logger.Info("BinancePlaceOrder: side=%s, price=%s, quantity=%f, clientID=%s", order.OrderType, fPrice, fQuantity, order.ClientOrderID)
	if order.OrderType == "buy" {
		res, err := cli.orderClient.NewCreateOrderService().
			NewClientOrderID(order.ClientOrderID).
			Symbol(order.Symbol).
			Side(delivery.SideTypeBuy).
			Type(delivery.OrderTypeLimit).
			TimeInForce(delivery.TimeInForceTypeGTX).
			Price(fPrice).
			Quantity(fQuantity).
			Do(context.Background())
		if err != nil {
			logger.Error("binance place order error，side=buy, price=%s, amount=%s, symbol=%s, message is %s",
				fPrice, fQuantity, order.Symbol, err.Error())
			return ""
		}
		return strconv.FormatInt(res.OrderID, 10)
	} else if order.OrderType == "sell" {
		res, err := cli.orderClient.NewCreateOrderService().
			NewClientOrderID(order.ClientOrderID).
			Symbol(order.Symbol).
			Side(delivery.SideTypeSell).
			Type(delivery.OrderTypeLimit).
			TimeInForce(delivery.TimeInForceTypeGTX).
			Price(fPrice).
			Quantity(fQuantity).
			Do(context.Background())
		if err != nil {
			logger.Error("binance place order error，side=buy, price=%s, amount=%s, symbol=%s, message is %s",
				fPrice, fQuantity, order.Symbol, err.Error())
			return ""
		}
		return strconv.FormatInt(res.OrderID, 10)
	}
	return ""
}

// 判断API调用频率
// n为api权重
func (cli *BinanceDeliveryClient) checkLimit(n int) bool {
	if cli.limitProcess == 1 {
		err := cli.limiter.WaitN(context.Background(), n)
		if err != nil {
			logger.Error(err.Error())
		}
		return true
	}
	ret := cli.limiter.AllowN(time.Now(), n)
	if !ret {
		logger.Info("BinanceMM API Limit")
	}
	return ret
}

// 取消所有订单
func (cli *BinanceDeliveryClient) CancelAllOrders(symbol string) bool {
	err := cli.orderClient.NewCancelAllOpenOrdersService().Symbol(strings.ToUpper(symbol)).Do(context.Background())
	if err != nil {
		logger.Error(err.Error())
		return false
	}
	return true
}

func (cli *BinanceDeliveryClient) CancelOrdersByClientID(clientOrderIDs *[]string, symbol string) ([]string, error) {
	orderNum := len(*clientOrderIDs)
	canceledIds := make([]string, orderNum)

	resp, err := cli.orderClient.NewCancelMultiplesOrdersService().
		Symbol(symbol).
		OrigClientOrderIDList(*clientOrderIDs).Do(context.Background())
	if err != nil {
		logger.Error(err.Error())
		return canceledIds, err
	}

	for _, order := range resp {
		canceledIds = append(canceledIds, order.ClientOrderID)
	}
	return canceledIds, nil
}

// --------------------------------------以下是websocket

type BinanceDeliveryWSClient struct {
	WSClient
	httpClient     BinanceDeliveryClient
	priceWSHandler PriceProcessHandler
	orderWSHandler OrderProcessHandler
	errorHandler   ErrorHandler

	symbols         []string //多币种
	listenKey       string
	bookTickerStopC []chan struct{} // bookTicker channel
	depthStopC      []chan struct{} // bookTicker channel
	trxOrderStopC   chan struct{}   // transaction order channel

	bookTickerLastUpdateIDMap sync.Map // 上一次symbol 更新 bookTicker 价格的 id
	depthLastUpdateIDMap      sync.Map // 上一次更新depth 价格的 id

	// depth data
	Asks sync.Map // symbol => []DepthPriceItem
	Bids sync.Map
}

func (cli *BinanceDeliveryWSClient) Init(config Config) bool {
	cli.symbols = config.Symbols
	for _, symbol := range cli.symbols {
		cli.bookTickerLastUpdateIDMap.Store(symbol, int64(0))
		cli.depthLastUpdateIDMap.Store(symbol, int64(0))
	}
	return true
}
func (cli *BinanceDeliveryWSClient) SetPriceHandler(handler PriceProcessHandler, errHandler ErrorHandler) {
	cli.priceWSHandler = handler
	cli.errorHandler = errHandler
}

func (cli *BinanceDeliveryWSClient) SetOrderHandler(handler OrderProcessHandler) {
	cli.orderWSHandler = handler
}

func (cli *BinanceDeliveryWSClient) SetHttpClient(client BinanceDeliveryClient) {
	cli.httpClient = client // 获取订单消息时，更新listenKey需要用到
}

func (cli *BinanceDeliveryWSClient) StartWS() bool {
	for _, symbol := range cli.symbols {
		// 启动 bookTicker
		cli.bookTickerWSConnect(symbol)
		time.Sleep(30 * time.Millisecond)

		// 启动 depth
		cli.depthWSConnect(symbol)
		time.Sleep(30 * time.Millisecond)
	}

	// 获取 listenKey，监听transaction 消息时，需要这个 key
	listenKey := cli.httpClient.GetListenKey()
	if listenKey == "" {
		logger.Error("get binance delivery listen key failed, exit the program")
		return false
	}
	cli.listenKey = listenKey
	cli.orderWSConnect()

	// listenKey 每60分钟过期一次，所以需要加个定时器，提前续期
	go common.Timer(30*time.Minute, cli.refreshListenKey)

	return true
}

func (cli *BinanceDeliveryWSClient) bookTickerWSConnect(symbol string) bool {
	_, stopC, err := delivery.WsBookTickerServe(symbol, cli.bookTickerMsgHandler, cli.bookTickerErrorHandler)
	if err != nil {
		logger.Error("Failed to establish connection with delivery BookTicker websocket, message is %s", err.Error())
		return false
	}
	logger.Info("Delivery BookTicker WS is established")
	cli.bookTickerStopC = append(cli.bookTickerStopC, stopC)
	return true
}

func (cli *BinanceDeliveryWSClient) bookTickerMsgHandler(event *delivery.WsBookTickerEvent) {
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
		logger.Error("Binance delivery bookTickerHandler convert bid price=%s to float64 error, message: %s",
			event.BestBidPrice, err.Error())
		return
	}

	bidQty, err := strconv.ParseFloat(event.BestBidQty, 64)
	if err != nil {
		logger.Error("Binance delivery bookTickerHandler convert bid quantity=%s to float64 error, message: %s",
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
		logger.Error("Binance delivery bookTickerHandler convert ask price=%s to float64 error, message: %s",
			event.BestAskPrice, err.Error())
		return
	}
	askQty, err := strconv.ParseFloat(event.BestAskQty, 64)
	if err != nil {
		logger.Error("Binance delivery bookTickerHandler convert ask quantity=%s to float64 error, message: %s",
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
		priceResp.MsgType = "deliveryBookTicker"
		priceResp.TimeStamp = event.Time
		priceResp.Symbol = event.Symbol
		cli.priceWSHandler(&priceResp)
	}
}

func (cli *BinanceDeliveryWSClient) bookTickerErrorHandler(err error) {
	// todo 可以查看下error的消息，如果能只重连对应的链接是最好的
	logger.Error("Binance delivery bookTickerErrorHandler emit, message: %s", err.Error())
	// 重试链接
	logger.Warn("Binance delivery bookTicker reconnect")
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
func (cli *BinanceDeliveryWSClient) depthWSConnect(symbol string) bool {
	// 清空
	var askItems []DepthPriceItem
	var bidItems []DepthPriceItem
	cli.Asks.Store(symbol, askItems)
	cli.Bids.Store(symbol, bidItems)

	rate := 100 * time.Millisecond
	_, stopC, err := delivery.WsDiffDepthServeWithRate(symbol, &rate, cli.depthMsgHandler, cli.depthErrorHandler)
	if err != nil {
		logger.Error("failed to establish connection with delivery depth websocket, message is %s", err.Error())
		return false
	}
	logger.Info("delivery depth WS is established")
	cli.depthStopC = append(cli.depthStopC, stopC)

	// 获取全量数据
	cli.getDeliveryDepthPrice(symbol)
	return true
}

func (cli *BinanceDeliveryWSClient) depthMsgHandler(event *delivery.WsDepthEvent) {
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
		priceResp.MsgType = "deliveryDepth"
		priceResp.TimeStamp = event.Time
		priceResp.Asks = askItems
		priceResp.Bids = bidItems
		priceResp.Symbol = event.Symbol
		cli.priceWSHandler(&priceResp)
	}
}

func (cli *BinanceDeliveryWSClient) parseDepthData(symbol string, bids []futures.Bid, asks []futures.Ask, LastUpdateID int64) {
	for _, bid := range bids {
		bidPrice, err := decimal.NewFromString(bid.Price)
		if err != nil {
			logger.Error("Binance delivery depthMsgHandler convert bid price=%s to float64 error, message: %s",
				bid.Price, err.Error())
			return
		}
		bidQty, err := decimal.NewFromString(bid.Quantity)
		if err != nil {
			logger.Error("Binance delivery depthMsgHandler convert bid quantity=%s to float64 error, message: %s",
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

func (cli *BinanceDeliveryWSClient) depthErrorHandler(err error) {
	logger.Error("Binance delivery depthErrorHandler emit, message: %s", err.Error())
	// 重试链接
	logger.Warn("Binance delivery depthErrorHandler reconnect")
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

func (cli *BinanceDeliveryWSClient) orderWSConnect() bool {
	_, stopC, err := delivery.WsUserDataServe(cli.listenKey, cli.orderHandler, cli.orderErrorHandler)
	if err != nil {
		logger.Error("failed to establish connection with delivery transaction order websocket, message is %s", err.Error())
		return false
	}
	logger.Info("delivery transaction order WS is established")
	cli.trxOrderStopC = stopC
	return true
}

func (cli *BinanceDeliveryWSClient) orderHandler(event *delivery.WsUserDataEvent) {
	topic := string(event.Event)
	if topic == "ORDER_TRADE_UPDATE" {
		logger.Info("delivery order event is: %+v", event)
		var orderResp OrderWSResponse
		orderResp.Exchange = "Binance"
		orderResp.MsgType = topic
		// 订单成交
		orderEvent := event.OrderTradeUpdate
		price, volume, diffVolume := 0.0, 0.0, 0.0
		if orderEvent.Status == "PARTIALLY_FILLED" || orderEvent.Status == "FILLED" {
			volume, _ = strconv.ParseFloat(orderEvent.LastFilledQty, 64)
			price, _ = strconv.ParseFloat(orderEvent.LastFilledPrice, 64)
			if orderEvent.Side == delivery.SideTypeBuy {
				diffVolume = volume
			} else {
				diffVolume = -volume
			}

			// 账户信息变动
			orderResp.Position += diffVolume
			orderResp.PositionAbs = math.Abs(orderResp.Position)
			logger.Info("transaction update, volume=%f, diffVolume=%f positionMargin=%f", volume, diffVolume, orderResp.Position)
		} else if orderEvent.Status == "EXPIRED" {
			price, _ = strconv.ParseFloat(orderEvent.OriginalPrice, 64)
			volume, _ = strconv.ParseFloat(orderEvent.OriginalQty, 64)
			logger.Info("order expired, orderEvent=%+v", orderEvent)
		} else if orderEvent.Status == "NEW" {
			price, _ = strconv.ParseFloat(orderEvent.OriginalPrice, 64)
			volume, _ = strconv.ParseFloat(orderEvent.OriginalQty, 64)
		}
		orderResp.Order.Exchange = "Binance"
		orderResp.Order.Symbol = orderEvent.Symbol
		orderResp.Order.OrderType = strings.ToLower(string(orderEvent.Side))
		orderResp.Order.ClientOrderID = orderEvent.ClientOrderID
		orderResp.Order.OrderID = strconv.FormatInt(event.OrderTradeUpdate.ID, 10)
		orderResp.Order.OrderPrice = price
		orderResp.Order.OrderVolume = volume
		orderResp.Status = string(orderEvent.Status)
		cli.orderWSHandler(&orderResp)
	} else if topic == "ACCOUNT_UPDATE" {
		var orderRespArr []OrderWSResponse
		updateEvent := event.AccountUpdate
		logger.Info("ACCOUNT_UPDATE: updateEvent=%+v", updateEvent)

		if updateEvent.Reason == "ORDER" {
			if updateEvent.Positions != nil {
				for _, item := range updateEvent.Positions {
					var orderResp OrderWSResponse
					orderResp.Exchange = "Binance"
					orderResp.MsgType = topic

					if common.InArray(item.Symbol, cli.symbols) {
						logger.Info("ACCOUNT_UPDATE: ORDER=%+v", item)

						positionAmount, err := strconv.ParseFloat(item.Amount, 64)
						if err != nil {
							logger.Error("Binance TrxHandler convert positionAmount=%s to float64 error, message: %s",
								item.Amount, err.Error())
							return
						}

						orderResp.Status = "ORDER_UPDATE"
						orderResp.Symbol = item.Symbol
						orderResp.Position = positionAmount
						orderResp.PositionAbs = math.Abs(orderResp.Position)

					}
					orderRespArr = append(orderRespArr, orderResp)
				}
			}
		}
		size := len(orderRespArr)
		for i := 0; i < size; i++ {
			cli.orderWSHandler(&orderRespArr[i])
		}
	}

}

func (cli *BinanceDeliveryWSClient) orderErrorHandler(err error) {
	logger.Error("Binance delivery trxErrorHandler emit, message: %s", err.Error())
	// 重试链接
	logger.Warn("Binance delivery trx reconnect")
	cli.orderWSConnect()
}

func (cli *BinanceDeliveryWSClient) refreshListenKey() {
	cli.httpClient.KeepAliveListenKey(cli.listenKey)
}

func (cli *BinanceDeliveryWSClient) getDeliveryDepthPrice(symbol string) {
	resp, err := cli.httpClient.GetDepthPriceInfo(symbol)
	if err != nil {
		logger.Error(err.Error())
		return
	}
	cli.parseDepthData(symbol, resp.Bids, resp.Asks, resp.LastUpdateID)
}

func (cli *BinanceDeliveryWSClient) StopWS() bool {
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

	// 关闭 transaction order ws
	if cli.trxOrderStopC != nil {
		cli.trxOrderStopC <- struct{}{}
	}

	return true
}
