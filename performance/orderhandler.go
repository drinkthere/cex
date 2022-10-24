package main

import (
	"cex/client"
	"cex/common"
	"cex/common/logger"
	"cex/config"
	"time"
)

type OrderHandler struct {
	BuyOrders  map[string]*common.OrderBook
	SellOrders map[string]*common.OrderBook

	BinanceDeliveryOrderClient client.BinanceDeliveryClient
	BinanceFuturesOrderClient  client.BinanceFuturesClient
	BinanceSpotOrderClient     client.BinanceSpotClient
	MinAccuracy                float64
}

func (handler *OrderHandler) Init(cfg *config.Config) {
	binanceConfig := client.Config{
		AccessKey:    cfg.BinanceAPIKey,
		SecretKey:    cfg.BinanceSecretKey,
		APILimit:     cfg.APILimit,
		LimitProcess: cfg.LimitProcess,
	}
	handler.BinanceDeliveryOrderClient.Init(binanceConfig)
	handler.BinanceFuturesOrderClient.Init(binanceConfig)
	handler.BinanceSpotOrderClient.Init(binanceConfig)

	handler.BuyOrders = map[string]*common.OrderBook{}
	handler.SellOrders = map[string]*common.OrderBook{}
	for _, symbol := range ctxt.Symbols {
		handler.BuyOrders[symbol] = &common.OrderBook{}
		handler.BuyOrders[symbol].Init()
		handler.SellOrders[symbol] = &common.OrderBook{}
		handler.SellOrders[symbol].Init()
		// 设置交易对的杠杆
		symbolCfg := cfg.SymbolConfigs[symbol]
		handler.BinanceDeliveryOrderClient.ChangeLeverage(symbol, symbolCfg.Leverage)
	}

	handler.MinAccuracy = cfg.MinAccuracy
}

// 取消订单（必须相同交易对）
func (handler *OrderHandler) CancelOrdersByClientID(orders []*common.Order) {
	defer common.TimeCost(time.Now(), "cancelByClientId")
	clientOrderIDs := []string{}
	clientOrderIDMap := make(map[string]*common.Order)
	for _, order := range orders {
		clientOrderIDs = append(clientOrderIDs, order.ClientOrderID)
		clientOrderIDMap[order.ClientOrderID] = order
	}

	size := len(clientOrderIDs)
	if size <= 0 {
		return
	}
	symbol := orders[0].Symbol

	// 每次最多取消10个订单
	for i := 0; i < size; i += 10 {
		end := i + 10
		if end > size {
			end = size
		}
		lst := clientOrderIDs[i:end]
		successIDs, _ := handler.BinanceDeliveryOrderClient.CancelOrdersByClientID(&lst, symbol)
		for _, id := range successIDs {
			_, ok := clientOrderIDMap[id]
			if ok {
				clientOrderIDMap[id].Status = common.CANCEL
			}
		}
	}
}

func (handler *OrderHandler) CancelAllOrdersWithSymbol(symbol string) bool {
	buyOrderBook := handler.BuyOrders[symbol]
	sellOrderBook := handler.SellOrders[symbol]

	if len(buyOrderBook.Data) > 0 || len(sellOrderBook.Data) > 0 {
		handler.BinanceDeliveryOrderClient.CancelAllOrders(symbol)
	}
	return true
}

func (handler *OrderHandler) CancelAllOrders() bool {
	for _, symbol := range ctxt.Symbols {
		buyOrderBook := handler.BuyOrders[symbol]
		sellOrderBook := handler.SellOrders[symbol]
		if len(buyOrderBook.Data) > 0 || len(sellOrderBook.Data) > 0 {
			handler.BinanceDeliveryOrderClient.CancelAllOrders(symbol)
		}
	}
	return true
}

func (handler *OrderHandler) CancelAllOrdersWithoutCheckOrderBook() bool {
	for _, symbol := range ctxt.Symbols {
		handler.BinanceDeliveryOrderClient.CancelAllOrders(symbol)
	}
	return true
}

func (handler *OrderHandler) Size() int {
	size := 0
	for _, orderbook := range handler.BuyOrders {
		size += orderbook.Size()
	}
	for _, orderbook := range handler.SellOrders {
		size += orderbook.Size()
	}
	return size
}

// 对冲订单
func (handler *OrderHandler) PlaceHedgeOrder(order *common.Order) {
	// 这里的逻辑是用现货市价来对冲订单
	handler.BinanceSpotOrderClient.PlaceMarketOrder(order)
}

// 从orderbook中删除订单
func (handler *OrderHandler) DeleteByClientOrderID(symbol string, orderType string, clientOrderID string) {
	if orderType == "buy" {
		orderBook, ok := handler.BuyOrders[symbol]
		if ok {
			orderBook.DeleteByClientOrderID(clientOrderID)
		}
	} else if orderType == "sell" {
		orderBook, ok := handler.SellOrders[symbol]
		if ok {
			orderBook.DeleteByClientOrderID(clientOrderID)
		}
	}
}

// 更新orderBook中订单的状态
func (handler *OrderHandler) UpdateStatus(symbol string, orderType string, clientOrderID string, status int) {
	if orderType == "buy" {
		orderbook, ok := handler.BuyOrders[symbol]
		if ok {
			orderbook.UpdateStatus(clientOrderID, status)
		}
	} else if orderType == "sell" {
		orderbook, ok := handler.SellOrders[symbol]
		if ok {
			orderbook.UpdateStatus(clientOrderID, status)
		}
	}
}

// 调用API下单
func (handler *OrderHandler) PlaceOrders(orders []*common.Order) {
	orderSize := len(orders)
	for i := 0; i < orderSize; i++ {
		go handler.PlaceOrder(orders[i])
	}
}

// 调用API下单
func (handler *OrderHandler) PlaceOrder(order *common.Order) {
	symbol := order.Symbol

	order.CreateAt = time.Now().Unix()
	order.ClientOrderID = common.GetClientOrderID()
	orderBook := handler.BuyOrders[symbol]
	if order.OrderType == "sell" {
		orderBook = handler.SellOrders[symbol]
	}
	orderBook.Add(order)

	// parse symbol

	orderID := handler.BinanceDeliveryOrderClient.PlaceOrderGTX(order)
	if orderID != "" {
		order.OrderID = orderID
		if order.Status == common.NEW {
			order.Status = common.CREATE
		}
	} else {
		order.Status = common.FAILED
		// 从队列删除
		orderBook.DeleteByClientOrderID(order.ClientOrderID)
	}
}

func (handler *OrderHandler) UpdateOrders() {
	orders := []*common.Order{}
	symbol := "BTCUSD_PERP"
	symbolCfg := cfg.SymbolConfigs[symbol]
	contractNum := float64(symbolCfg.ContractNum)
	symbolContext := ctxt.GetSymbolContext(symbol)
	spotPriceItem := ctxt.GetPriceItem(cfg.Exchange, symbol, "spot")
	futuresPriceItem := ctxt.GetPriceItem(cfg.Exchange, symbol, "futures")
	logger.Info("%v|%v", symbolContext.BidPrice, cfg.MinAccuracy)
	if symbolContext.Risk != 0 || symbol == "BNBUSD_PERP" {
		return
	}

	if spotPriceItem == nil || futuresPriceItem == nil || symbolContext.BidPrice < cfg.MinAccuracy {
		return
	}

	orderBook := handler.BuyOrders[symbol]
	orderBook.Mutex.RLock()
	buyOrderBookSize := orderBook.Size()
	if buyOrderBookSize < 1 {
		buyPrice := symbolContext.BidPrice - 100
		order := common.Order{Symbol: symbol, OrderType: "buy", OrderVolume: contractNum,
			OrderPrice: buyPrice}
		orders = append(orders, &order)
	}
	orderBook.Mutex.RUnlock()
	handler.PlaceOrders(orders)
}

// 取消距离较远的订单
func (handler *OrderHandler) CancelOrders(symbol string) {
	timestamp := common.GetTimestampInMS()
	// 每5s取消2次
	symbolContext := ctxt.GetSymbolContext(symbol)
	if symbolContext == nil || timestamp-symbolContext.LastCancelFarTime < 5000 {
		return
	}

	cancelOrders := []*common.Order{}

	// buy orders
	orderBook := handler.BuyOrders[symbol]
	orderBook.Mutex.RLock()
	size := orderBook.Size()
	for i := 0; i < size; i++ {
		cancelOrders = append(cancelOrders, orderBook.Data[i])
	}
	orderBook.Mutex.RUnlock()
	if len(cancelOrders) > 0 {
		symbolContext.LastCancelFarTime = timestamp
		handler.CancelOrdersByClientID(cancelOrders)
	}
}

// 更新订单
func UpdateOrders() {
	orderHandler.UpdateOrders()
}

// 取消距离较远的订单
func CancelOrders() {
	for _, symbol := range ctxt.Symbols {
		orderHandler.CancelOrders(symbol)
	}
}
