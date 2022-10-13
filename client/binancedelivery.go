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

	"github.com/adshao/go-binance/v2/delivery"
	"golang.org/x/time/rate"
)

type BinanceDeliveryClient struct {
	OrderClient
	Name         string
	orderClient  *delivery.Client
	limiter      *rate.Limiter
	limitProcess int
}

func (cli *BinanceDeliveryClient) Init(config Config) bool {
	cli.Name = "BinanceDelivery"
	cli.orderClient = delivery.NewClient(config.AccessKey, config.SecretKey)
	limit := rate.Every(1 * time.Second / time.Duration(config.APILimit))
	cli.limiter = rate.NewLimiter(limit, 60)
	cli.limitProcess = config.LimitProcess
	return true
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
	trxOrderStopC   chan struct{}   // transaction order channel

	bookTickerLastUpdateIDMap sync.Map // 上一次symbol 更新 bookTicker 价格的 id
}

func (cli *BinanceDeliveryWSClient) Init(config Config) bool {
	cli.symbols = config.Symbols
	for _, symbol := range cli.symbols {
		cli.bookTickerLastUpdateIDMap.Store(symbol, int64(0))
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
	// 启动 bookTicker
	for _, symbol := range cli.symbols {
		cli.bookTickerWSConnect(symbol)
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

func (cli *BinanceDeliveryWSClient) StopWS() bool {
	// 关闭 bookTicker ws
	if cli.bookTickerStopC != nil {
		for _, stopC := range cli.bookTickerStopC {
			stopC <- struct{}{}
		}
	}

	// 关闭 transaction order ws
	if cli.trxOrderStopC != nil {
		cli.trxOrderStopC <- struct{}{}
	}

	return true
}