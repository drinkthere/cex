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

// 取消价格不合适的订单
func (handler *OrderHandler) CancelOrders(symbol string) {

}
func (handler *OrderHandler) CancelAllOrdersWithSymbol(symbol string) bool {
	buyOrderBook := handler.BuyOrders[symbol]
	sellOrderBook := handler.SellOrders[symbol]
	logger.Info("CancelAllOrders %s buy order size: %d, sell order size: %d", symbol,
		buyOrderBook.Size(), sellOrderBook.Size())

	if len(buyOrderBook.Data) > 0 || len(sellOrderBook.Data) > 0 {
		handler.BinanceDeliveryOrderClient.CancelAllOrders(symbol)
	}
	return true
}

func (handler *OrderHandler) CancelAllOrders() bool {
	logger.Info("CancelAllOrders order size: %d", handler.Size())
	for _, symbol := range ctxt.Symbols {
		buyOrderBook := handler.BuyOrders[symbol]
		sellOrderBook := handler.SellOrders[symbol]
		if len(buyOrderBook.Data) > 0 || len(sellOrderBook.Data) > 0 {
			handler.BinanceDeliveryOrderClient.CancelAllOrders(symbol)
		}
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

func (handler *OrderHandler) UpdateOrders() {
	account := ctxt.Accounts.GetAccount(cfg.Exchange, cfg.SwapType)
	orders := []*common.Order{}

	// buy orders
	for symbol, orderBook := range handler.BuyOrders {
		symbolCfg := cfg.SymbolConfigs[symbol]
		// 每单交易量
		contractNum := float64(symbolCfg.ContractNum)
		// 当前仓位，挂单随持仓量变化，long仓越多，越容易挂ask单，越难挂bid单，反之则反。
		position := account.GetPositionsInfo(symbol)
		ratio := 1 + cfg.TickerShift*position.PositionAbs/contractNum

		symbolContext := ctxt.GetSymbolContext(symbol)
		spotPriceItem := ctxt.GetPriceItem(cfg.Exchange, symbol, "spot")
		futuresPriceItem := ctxt.GetPriceItem(cfg.Exchange, symbol, "futures")

		if symbolContext.Risk != 0 || symbol == "BNBUSD_PERP" {
			continue
		}

		if spotPriceItem == nil || futuresPriceItem == nil || symbolContext.BidPrice < cfg.MinAccuracy {
			continue
		}

		dynamicConfig := GetDynamicConfig(symbol)

		tempOrderNum, tmpCreateOrderNum := cfg.MaxOrderNum, 0
		orderBook.Mutex.RLock()
		for i := 1; i <= tempOrderNum; i++ {
			buyPrice := symbolContext.BidPrice - float64(i)*cfg.GapSizeK*dynamicConfig.AdjustedGapSize
			inRange := handler.IsInRange(i, buyPrice, "buy", orderBook, dynamicConfig)

			// 根据持仓获得修正后的buyPrice
			adjustedDeliveryBuyPrice := getAdjustedPrice(buyPrice, ratio, position.Position)
			adjustedSpotBuyPrice := spotPriceItem.BidPrice * dynamicConfig.AdjustedForgivePercent
			adjustedFuturesBuyPrice := futuresPriceItem.BidPrice * dynamicConfig.AdjustedForgivePercent
			if !inRange && adjustedDeliveryBuyPrice < adjustedSpotBuyPrice &&
				adjustedDeliveryBuyPrice < adjustedFuturesBuyPrice &&
				position.PositionAbs < contractNum*float64(symbolCfg.Leverage) {
				if tmpCreateOrderNum+orderBook.Size() < cfg.MaxOrderNum {
					logger.Info("===position: %.2f, maxPosition: %.2f", position.Position, contractNum*float64(symbolCfg.Leverage))
					order := common.Order{Symbol: symbol, OrderType: "buy", OrderVolume: contractNum,
						OrderPrice: buyPrice}
					orders = append(orders, &order)
					tmpCreateOrderNum++
				}

			}

			if !inRange && (adjustedDeliveryBuyPrice > adjustedSpotBuyPrice || buyPrice > adjustedFuturesBuyPrice) {
				tempOrderNum++
			}
		}
		orderBook.Mutex.RUnlock()
	}

	// sell orders
	for symbol, orderBook := range handler.SellOrders {
		symbolCfg := cfg.SymbolConfigs[symbol]
		// 每单交易量
		contractNum := float64(symbolCfg.ContractNum)
		// 当前仓位，挂单随持仓量变化，long仓越多，越容易挂ask单，越难挂bid单，反之则反。
		position := account.GetPositionsInfo(symbol)
		ratio := 1 + cfg.TickerShift*position.PositionAbs/contractNum

		symbolContext := ctxt.GetSymbolContext(symbol)
		spotPriceItem := ctxt.GetPriceItem(cfg.Exchange, symbol, "spot")
		futuresPriceItem := ctxt.GetPriceItem(cfg.Exchange, symbol, "futures")

		if symbolContext.Risk != 0 || symbol == "BNBUSD_PERP" {
			continue
		}
		if spotPriceItem == nil || futuresPriceItem == nil || symbolContext.BidPrice < cfg.MinAccuracy {
			continue
		}

		dynamicConfig := GetDynamicConfig(symbol)

		tempOrderNum, tmpCreateOrderNum := cfg.MaxOrderNum, 0
		orderBook.Mutex.RLock()
		for i := 1; i <= tempOrderNum; i++ {
			sellPrice := symbolContext.AskPrice + float64(i)*cfg.GapSizeK*dynamicConfig.AdjustedGapSize
			inRange := handler.IsInRange(i, sellPrice, "sell", orderBook, dynamicConfig)

			// 根据持仓获得修正后的sellPrice
			adjustedDeliverySellPrice := getAdjustedPrice(sellPrice, ratio, position.Position)
			adjustedSpotSellPrice := spotPriceItem.BidPrice / dynamicConfig.AdjustedForgivePercent
			adjustedFuturesSellPrice := futuresPriceItem.BidPrice / dynamicConfig.AdjustedForgivePercent

			if !inRange && adjustedDeliverySellPrice > adjustedSpotSellPrice &&
				adjustedDeliverySellPrice > adjustedFuturesSellPrice &&
				position.Position > -contractNum*float64(symbolCfg.Leverage) {

				if tmpCreateOrderNum+orderBook.Size() < cfg.MaxOrderNum {
					logger.Info("===CreateOrder: index: %d, num: %d, askPrice: %.2f, adjustedDeliverySellPrice: %.2f, adjustedSpotSellPrice: %.2f, adjustedFuturesSellPrice: %.2f", i, tempOrderNum, symbolContext.AskPrice, adjustedDeliverySellPrice, adjustedSpotSellPrice, adjustedFuturesSellPrice)
					order := common.Order{Symbol: symbol, OrderType: "sell", OrderVolume: contractNum,
						OrderPrice: sellPrice}
					orders = append(orders, &order)
					tmpCreateOrderNum++
				}
			}

			if !inRange && (adjustedDeliverySellPrice < adjustedSpotSellPrice || adjustedDeliverySellPrice < adjustedFuturesSellPrice) {
				tempOrderNum++
			}
		}
		orderBook.Mutex.RUnlock()
	}

	logger.Info("CreateOrders: %d", len(orders))
	handler.PlaceOrders(orders)
}

func (handler *OrderHandler) IsInRange(loop int, price float64, offset string, orderBook *common.OrderBook, dynamicConfig *DynamicConfig) bool {
	for _, order := range orderBook.Data {
		if loop != 0 {
			if price <= order.OrderPrice+dynamicConfig.AdjustedGapSize*cfg.GapSizeK && price >= order.OrderPrice-dynamicConfig.AdjustedForgivePercent*cfg.GapSizeK {
				return true
			}
		} else {
			if offset == "buy" {
				if order.OrderPrice >= price {
					return true
				}
			} else if offset == "sell" {
				if order.OrderPrice <= price {
					return true
				}
			}

		}
	}
	return false
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
	logger.Info("OrderDebug: op=New, %s", order.FormatString())
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

func getAdjustedPrice(price float64, ratio, position float64) float64 {
	if position > 0 {
		return price * ratio
	} else if position < 0 {
		return price / ratio
	}
	return price
}

// 更新订单
func UpdateOrders() {
	orderHandler.UpdateOrders()
}
