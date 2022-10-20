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

// 取消距离较远的订单
func (handler *OrderHandler) CancelFarOrders(symbol string) {
	timestamp := common.GetTimestampInMS()
	// 每2s取消2次
	symbolContext := ctxt.GetSymbolContext(symbol)
	if symbolContext == nil || timestamp-symbolContext.LastCancelFarTime < 2000 {
		return
	}

	cancelOrders := []*common.Order{}

	// buy orders
	orderBook := handler.BuyOrders[symbol]
	size := orderBook.Size() - cfg.MaxOrderNum
	if size > 0 {
		orderBook.Sort()

		orderBook.Mutex.RLock()
		for i := 0; i < size; i++ {
			cancelOrders = append(cancelOrders, orderBook.Data[i])
		}
		orderBook.Mutex.RUnlock()
	}

	// sell orders
	orderBook = handler.SellOrders[symbol]
	size = orderBook.Size() - cfg.MaxOrderNum
	if size > 0 {
		orderBook.Sort()

		orderBook.Mutex.RLock()
		for i := orderBook.Size() - 1; orderBook.Size()-i <= size; i-- {
			cancelOrders = append(cancelOrders, orderBook.Data[i])
		}
		orderBook.Mutex.RUnlock()
	}

	if len(cancelOrders) > 2 {
		symbolContext.LastCancelFarTime = timestamp
	}
	logger.Debug("CancelFarOrders: %+v", cancelOrders)
	handler.CancelOrdersByClientID(cancelOrders)
}
