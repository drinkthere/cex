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
	timestamp := common.GetTimestampInMS()
	symbolContext := ctxt.GetSymbolContext(symbol)
	// 每200ms取消2次
	if symbolContext == nil || timestamp-symbolContext.LastCancelTime < 200 {
		return
	}

	cancelOrders := []*common.Order{}

	dynamicConfig := GetDynamicConfig(symbol)
	account := ctxt.Accounts.GetAccount(cfg.Exchange, cfg.SwapType)
	position := account.GetPositionsInfo(symbol)
	// buy orders
	orderBook := handler.BuyOrders[symbol]
	orderBook.Mutex.RLock()
	for i := 0; i < len(orderBook.Data); i++ {
		order := orderBook.Data[i]
		if order.Status != common.NEW && order.Status != common.CREATE && order.Status != common.CREATED {
			continue
		}
		// 如果订单价格离盘口的距离比较远，暂时不考虑取消
		gapSize := symbolContext.BidPrice - order.OrderPrice
		logger.Info("===orderPrice:%.2f, deliveryBidPrice:%.2f, gap: %.6f, gapSize>AdjustedGapSize: %b ",
			order.OrderPrice, symbolContext.BidPrice, gapSize, gapSize > dynamicConfig.AdjustedGapSize)
		if gapSize > dynamicConfig.AdjustedGapSize {
			continue
		}

		// 判断如果当前币本位bid价格和现货的ask价格的价差，如果手续费返点cover不住，就取消。
		// 加一个系数K，当仓位过高时，可以接受亏一些出货
		spotPriceItem := ctxt.GetPriceItem(cfg.Exchange, symbol, "spot")
		profitRatio := (spotPriceItem.BidPrice - symbolContext.AskPrice) / symbolContext.AskPrice
		positionRatio := position.PositionAbs / float64(cfg.SymbolConfigs[symbol].MaxContractNum)
		threashodl := -(cfg.Commission + cfg.CancelShift*positionRatio - cfg.Loss)
		// 最多能接受亏掉补偿手续费在家个让利回吐仓位
		if profitRatio < threashodl {
			cancelOrders = append(cancelOrders, order)
			logger.Info("===CancelOrder: index: %d, askPrice: %.2f, orderPrice: %.2f, spotBidPrice: %.2f, profitRatio: %.6f, threashold: %.6f, positionRatio: %.2f",
				i, symbolContext.AskPrice, order.OrderPrice, spotPriceItem.BidPrice, profitRatio, threashodl, positionRatio)
		}
	}
	orderBook.Mutex.RUnlock()

	// sell orders
	orderBook = handler.SellOrders[symbol]
	orderBook.Mutex.RLock()
	for i := 0; i < len(orderBook.Data); i++ {
		order := orderBook.Data[i]
		if order.Status != common.NEW && order.Status != common.CREATE && order.Status != common.CREATED {
			continue
		}

		// 如果订单价格离盘口的距离比较远，暂时不考虑取消
		gapSize := order.OrderPrice - symbolContext.AskPrice
		logger.Debug("===orderPrice:%.2f, deliveryAskPrice:%.2f, gap: %.6f, gapSize>AdjustedGapSize: %b ",
			order.OrderPrice, symbolContext.BidPrice, gapSize, gapSize > dynamicConfig.AdjustedGapSize)
		if gapSize > dynamicConfig.AdjustedGapSize {
			continue
		}

		// 判断如果当前币本位ask价格和现货的bid价格的价差，如果手续费返点cover不住，就取消。
		// 加一个系数K，当仓位过高时，可以接受亏一些出货
		spotPriceItem := ctxt.GetPriceItem(cfg.Exchange, symbol, "spot")
		profitRatio := (symbolContext.BidPrice - spotPriceItem.AskPrice) / symbolContext.BidPrice
		positionRatio := position.PositionAbs / float64(cfg.SymbolConfigs[symbol].MaxContractNum)
		threashodl := -(cfg.Commission + cfg.CancelShift*positionRatio - cfg.Loss)
		// 最多能接受亏掉补偿手续费在家个让利回吐仓位
		if profitRatio < threashodl {
			cancelOrders = append(cancelOrders, order)
			logger.Info("===CancelOrder: index: %d, bidPrice: %.2f, orderPrice: %.2f, spotAskPrice: %.2f, lossRatio: %.6f, threashold: %.6f, positionRatio: %.2f",
				i, symbolContext.BidPrice, order.OrderPrice, spotPriceItem.AskPrice, profitRatio, threashodl, positionRatio)
		}
	}
	orderBook.Mutex.RUnlock()

	logger.Debug("CancelOrders: %d", len(cancelOrders))
	//handler.CancelOrdersByClientID(cancelOrders)
	// 改成cancelAll
	if len(cancelOrders) > 0 {
		handler.CancelAllOrdersWithSymbol(symbol)
		symbolContext.LastCancelTime = timestamp
	}
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
	logger.Debug("CancelAllOrders %s buy order size: %d, sell order size: %d", symbol,
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

func (handler *OrderHandler) CancelAllOrdersWithoutCheckOrderBook() bool {
	logger.Info("CancelAllOrdersWithoutCheckOrderBook order size: %d", handler.Size())
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
	buyOrderBookSize, sellOrderBookSize := 0, 0

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
		buyOrderBookSize = orderBook.Size()
		for i := 1; i <= tempOrderNum; i++ {
			buyPrice := symbolContext.BidPrice - float64(i)*cfg.GapSizeK*dynamicConfig.AdjustedGapSize
			inRange := handler.IsInRange(i, buyPrice, "buy", orderBook, dynamicConfig)

			// 根据持仓获得修正后的buyPrice, 根据近期的波动，获得修正好的现货和U本位合约的buyPrice

			adjustedDeliveryBuyPrice := getAdjustedPrice(buyPrice, ratio, position.Position)
			adjustedSpotBuyPrice := spotPriceItem.BidPrice * dynamicConfig.AdjustedForgivePercent
			adjustedFuturesBuyPrice := futuresPriceItem.BidPrice * dynamicConfig.AdjustedForgivePercent
			logger.Debug("index: %d, buyPrice: %.2f, ratio: %f, position: %f, condition: %s|%s|%s|%s|%s",
				i, buyPrice, ratio, position.Position,
				!inRange,
				adjustedDeliveryBuyPrice < adjustedSpotBuyPrice,
				adjustedDeliveryBuyPrice < adjustedFuturesBuyPrice,
				position.Position < float64(symbolCfg.MaxContractNum),
				tmpCreateOrderNum <= cfg.MaxOrderOneStep)
			if !inRange && adjustedDeliveryBuyPrice < adjustedSpotBuyPrice &&
				adjustedDeliveryBuyPrice < adjustedFuturesBuyPrice &&
				position.Position < float64(symbolCfg.MaxContractNum) &&
				tmpCreateOrderNum <= cfg.MaxOrderOneStep {

				logger.Info("===position: %.2f, maxPosition: %.2f", position.Position, float64(symbolCfg.MaxContractNum))
				logger.Info("===CreateOrder: index: %d, num: %d, bidPrice: %.2f, adjustedDeliveryBuyPrice: %.2f, adjustedSpotBuyPrice: %.2f, adjustedFuturesBuyPrice: %.2f",
					i, tempOrderNum, symbolContext.BidPrice, adjustedDeliveryBuyPrice, adjustedSpotBuyPrice, adjustedFuturesBuyPrice)
				order := common.Order{Symbol: symbol, OrderType: "buy", OrderVolume: contractNum,
					OrderPrice: buyPrice}
				orders = append(orders, &order)
				tmpCreateOrderNum++

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
		sellOrderBookSize = orderBook.Size()
		for i := 1; i <= tempOrderNum; i++ {
			sellPrice := symbolContext.AskPrice + float64(i)*cfg.GapSizeK*dynamicConfig.AdjustedGapSize
			inRange := handler.IsInRange(i, sellPrice, "sell", orderBook, dynamicConfig)

			// 根据持仓获得修正后的sellPrice
			adjustedDeliverySellPrice := getAdjustedPrice(sellPrice, ratio, position.Position)
			adjustedSpotSellPrice := spotPriceItem.BidPrice / dynamicConfig.AdjustedForgivePercent
			adjustedFuturesSellPrice := futuresPriceItem.BidPrice / dynamicConfig.AdjustedForgivePercent
			logger.Debug("index: %d, buyPrice: %.2f, ratio: %f, position: %f, condition: %s|%s|%s|%s|%s",
				i, sellPrice, ratio, position.Position,
				!inRange,
				adjustedDeliverySellPrice > adjustedSpotSellPrice,
				adjustedDeliverySellPrice > adjustedFuturesSellPrice,
				position.Position > -float64(symbolCfg.MaxContractNum),
				tmpCreateOrderNum <= cfg.MaxOrderOneStep)
			if !inRange && adjustedDeliverySellPrice > adjustedSpotSellPrice &&
				adjustedDeliverySellPrice > adjustedFuturesSellPrice &&
				position.Position > -float64(symbolCfg.MaxContractNum) &&
				tmpCreateOrderNum <= cfg.MaxOrderOneStep {
				logger.Info("===position: %.2f, maxPosition: %.2f", position.Position, float64(symbolCfg.MaxContractNum))
				logger.Info("===CreateOrder: index: %d, num: %d, askPrice: %.2f, adjustedDeliverySellPrice: %.2f, adjustedSpotSellPrice: %.2f, adjustedFuturesSellPrice: %.2f",
					i, tempOrderNum, symbolContext.AskPrice, adjustedDeliverySellPrice, adjustedSpotSellPrice, adjustedFuturesSellPrice)
				order := common.Order{Symbol: symbol, OrderType: "sell", OrderVolume: contractNum,
					OrderPrice: sellPrice}
				orders = append(orders, &order)
				tmpCreateOrderNum++
			}

			if !inRange && (adjustedDeliverySellPrice < adjustedSpotSellPrice || adjustedDeliverySellPrice < adjustedFuturesSellPrice) {
				tempOrderNum++
			}
		}
		orderBook.Mutex.RUnlock()
	}

	logger.Info("CreateOrders: %d, buyOrderBookSize: %d, sellOrderBookSize: %d", len(orders), buyOrderBookSize, sellOrderBookSize)
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

// 取消间距较劲的订单
func (handler *OrderHandler) CancelCloseDistanceOrders(symbol string) {
	cancelOrders := []*common.Order{}

	// buy orders， 第一个和最后一个订单不做处理
	orderBook := handler.BuyOrders[symbol]
	dynamicConfigs := GetDynamicConfig(symbol)
	size := orderBook.Size()
	if size > 2 {
		orderBook.Sort()
		orderBook.Mutex.RLock()

		cursor := size - 2
		for i := size - 3; i > 0; i-- {
			currOrder := orderBook.Data[i]
			prevOrder := orderBook.Data[cursor]
			if prevOrder.OrderPrice-currOrder.OrderPrice < dynamicConfigs.AdjustedGapSize {
				logger.Info("===buy, curOrderPrice: %.2f, prevOrderPrice: %.2f, gapSize: %.2f, adjustedGapSize: %.2f",
					currOrder.OrderPrice, prevOrder.OrderPrice, prevOrder.OrderPrice-currOrder.OrderPrice, dynamicConfigs.AdjustedGapSize)
				cancelOrders = append(cancelOrders, currOrder)
			} else {
				cursor--
			}
		}
		orderBook.Mutex.RUnlock()
	}
	// sell orders， 第一个和最后一个订单不做处理
	orderBook = handler.SellOrders[symbol]
	size = orderBook.Size()
	if size > 2 {
		orderBook.Sort()
		orderBook.Mutex.RLock()
		cursor := 1
		for i := 2; i < size-1; i++ {
			currOrder := orderBook.Data[i]
			prevOrder := orderBook.Data[cursor]
			if currOrder.OrderPrice-prevOrder.OrderPrice < dynamicConfigs.AdjustedGapSize {
				logger.Info("===sell, curOrderPrice: %.2f, prevOrderPrice: %.2f, gapSize: %.2f, adjustedGapSize: %.2f",
					currOrder.OrderPrice, prevOrder.OrderPrice, currOrder.OrderPrice-prevOrder.OrderPrice, dynamicConfigs.AdjustedGapSize)
				cancelOrders = append(cancelOrders, currOrder)
			} else {
				cursor++
			}
		}
		orderBook.Mutex.RUnlock()
	}

	logger.Debug("CancelCloseDistanceOrders: %+v", cancelOrders)
	handler.CancelOrdersByClientID(cancelOrders)
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

// 取消距离较远的订单
func CancelFarOrders() {
	for _, symbol := range ctxt.Symbols {
		orderHandler.CancelFarOrders(symbol)
	}
}

func CancelCloseDistanceOrders() {
	for _, symbol := range ctxt.Symbols {
		orderHandler.CancelCloseDistanceOrders(symbol)
	}
}
