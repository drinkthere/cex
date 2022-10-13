package main

import (
	"cex/client"
	"cex/common"
	"cex/common/logger"
	"cex/config"
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

// 取消价格不合适的订单
func (handler *OrderHandler) CancelOrders(symbol string) {

}

// 对冲订单
func (handler *OrderHandler) PlaceHedgeOrder(order *common.Order) {
	// 这里的逻辑是用现货市价来对冲订单
	logger.Info("OrderDebug: Hedge op=New, %s", order.FormatString())
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
