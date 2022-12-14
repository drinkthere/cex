package main

import (
	"cex/client"
	"cex/common"
	"cex/common/logger"
	"cex/config"
)

type EventHandler struct {
	wsClient []client.WSClient
}

func (handler *EventHandler) Init(cfg *config.Config, orderHandler OrderHandler) {
	context := &ctxt
	binanceConfig := client.Config{
		AccessKey: cfg.BinanceAPIKey,
		SecretKey: cfg.BinanceSecretKey,
		Symbols:   cfg.Symbols,
	}

	// 初始化币安的币本位 WS client
	binanceDeliveryWSClient := new(client.BinanceDeliveryWSClient)
	binanceDeliveryWSClient.Init(binanceConfig)
	binanceDeliveryWSClient.SetHttpClient(orderHandler.BinanceDeliveryOrderClient)
	binanceDeliveryWSClient.SetPriceHandler(DeliveryPriceWSHandler, common.CommonErrorHandler)
	binanceDeliveryWSClient.SetOrderHandler(DeliveryOrderWSHandler)
	handler.wsClient = append(handler.wsClient, binanceDeliveryWSClient)

	// 初始化币安的U本位 WS client
	var futuresSymbols []string
	for futuresSymbol := range context.SymbolMap {
		futuresSymbols = append(futuresSymbols, futuresSymbol)
	}
	binanceConfig.Symbols = futuresSymbols
	binanceFuturesWSClient := new(client.BinanceFuturesWSClient)
	binanceFuturesWSClient.Init(binanceConfig)
	binanceFuturesWSClient.SetPriceHandler(FuturesPriceHandler, common.CommonErrorHandler)
	binanceFuturesWSClient.SetHttpClient(orderHandler.BinanceFuturesOrderClient)
	handler.wsClient = append(handler.wsClient, binanceFuturesWSClient)

	// 初始化币安现货的 WS client
	binanceSpotWSClient := new(client.BinanceSpotWSClient)
	binanceSpotWSClient.Init(binanceConfig)
	binanceSpotWSClient.SetPriceHandler(SpotPriceHandler, common.CommonErrorHandler)
	binanceSpotWSClient.SetHttpClient(orderHandler.BinanceSpotOrderClient)
	handler.wsClient = append(handler.wsClient, binanceSpotWSClient)
}

func (handler *EventHandler) Start() {
	for _, wsClient := range handler.wsClient {
		wsClient.StartWS()
	}
}

func (handler *EventHandler) Stop() {
	for _, wsClient := range handler.wsClient {
		wsClient.StopWS()
	}
}

func DeliveryPriceWSHandler(resp *client.PriceWSResponse) {
	context := &ctxt
	config := &cfg
	symbol := resp.Symbol
	symbolCfg := config.SymbolConfigs[symbol]
	symbolContext := context.GetSymbolContext(symbol)

	timeStamp := common.GetTimestampInMS()
	if resp.MsgType == "deliveryBookTicker" {
		// 处理期货bookTicker
		bidPrice, askPrice := 0.0, 0.0
		bidVolume, askVolume := 0.0, 0.0
		for _, item := range resp.Items {
			if item.Volume < symbolCfg.EffectiveNum {
				continue
			}
			if item.Direction == "buy" {
				bidPrice = item.Price
				bidVolume = item.Volume
			} else if item.Direction == "sell" {
				askPrice = item.Price
				askVolume = item.Volume
			}
		}
		if bidPrice > config.MinAccuracy {
			symbolContext.BidPrice = bidPrice
			symbolContext.BidVolume = bidVolume
			logger.Debug("binance delivery %s buy price is %f, quantity is %f at %d",
				symbol, bidPrice, bidVolume, resp.TimeStamp)
			logger.Debug("deliveryBookTicker|%d|%d|%d",
				timeStamp, resp.TimeStamp, timeStamp-resp.TimeStamp)
		}

		if askPrice > config.MinAccuracy {
			symbolContext.AskPrice = askPrice
			symbolContext.AskVolume = askVolume
			logger.Debug("binance delivery %s sell price is %f, quantity is %f at %d",
				symbol, askPrice, askVolume, resp.TimeStamp)
			logger.Debug("deliveryBookTicker|%d|%d|%d",
				timeStamp, resp.TimeStamp, timeStamp-resp.TimeStamp)
		}
		if bidPrice > config.MinAccuracy || askPrice > config.MinAccuracy {
			symbolContext.LastUpdateTime = timeStamp
		}

		logger.Debug("binance delivery bookTicker symbol: %s, bidPrice:%f, bidVolume:%f, askPrice:%f, askVolume:%f, updateTime:%d", resp.Symbol, bidPrice, bidVolume, askPrice, askVolume, resp.TimeStamp)
	} else if resp.MsgType == "deliveryDepth" {
		// 暂时只统计了时间，没有用到depth的Price
		bidPrice, askPrice := 0.0, 0.0
		for _, item := range resp.Bids {
			bidPrice, _ = item.Price.Float64()
		}
		for _, item := range resp.Asks {
			askPrice, _ = item.Price.Float64()
		}

		if bidPrice > config.MinAccuracy {
			logger.Debug("deliveryDepth|%d|%d|%d",
				timeStamp, resp.TimeStamp, timeStamp-resp.TimeStamp)
		}

		if askPrice > config.MinAccuracy {
			logger.Debug("deliveryDepth|%d|%d|%d",
				timeStamp, resp.TimeStamp, timeStamp-resp.TimeStamp)
		}
	}
}

func DeliveryOrderWSHandler(resp *client.OrderWSResponse) {
	context := &ctxt
	config := &cfg
	symbol := resp.Order.Symbol
	// symbolCfg := config.SymbolConfigs[symbol]

	if resp.MsgType == "ORDER_TRADE_UPDATE" {
		orderType := resp.Order.OrderType
		clientOrderID := resp.Order.ClientOrderID
		// 订单成交
		if resp.Status == "PARTIALLY_FILLED" || resp.Status == "FILLED" {
			deliveryContext := context.GetSymbolContext(resp.Order.Symbol)
			spotPriceItem := ctxt.GetPriceItem(config.Exchange, symbol, "spot")
			logger.Info("Op=Fill, Exchange=Binance, Direction=%s, filled price=%f, amount=%f, OrderID=%d, ClientOrderID=%s, BuyPrice=%.2f, SellPrice=%.2f, Symbol=%s, sBidPrice=%.4f, sAskPrice=%.4f",
				resp.Order.OrderType, resp.Order.OrderPrice, resp.Order.OrderVolume, resp.Order.OrderID,
				resp.Order.ClientOrderID, deliveryContext.BidPrice, deliveryContext.AskPrice,
				symbol, spotPriceItem.BidPrice, spotPriceItem.AskPrice)
			//
			//// 下单对冲
			//if config.FunctionHedge == 1 {
			//	hedgeOrderType := common.GetHedgeOrderType(orderType)
			//	// 合约张数
			//	volume := resp.Order.OrderVolume
			//	cont := float64(symbolCfg.Cont)
			//	price := 0.0
			//	if hedgeOrderType == "buy" {
			//		price = spotPriceItem.BidPrice
			//	} else {
			//		price = spotPriceItem.AskPrice
			//	}
			//	amount := volume * cont / price
			//	logger.Info("===volume:%s, price:%s, amount:%s, minHedgeSize:%s, hedgeOrderType: %s", volume, price, amount, symbolCfg.MinHedgeSize, hedgeOrderType)
			//
			//	if volume >= symbolCfg.MinHedgeSize {
			//		var order common.Order
			//		order.Exchange = "Binance"
			//		order.OrderType = hedgeOrderType
			//		order.OrderPrice = price
			//		order.OrderVolume = amount
			//		order.Symbol = resp.Order.Symbol
			//		order.ClientOrderID = resp.Order.ClientOrderID
			//		order.BaseAsset = symbolCfg.BaseAsset
			//		order.QuoteAsset = config.QuoteAsset
			//		order.Precision = symbolCfg.Precision
			//		orderHandler.PlaceHedgeOrder(&order)
			//	}
			//}

			if resp.Status == "FILLED" {
				orderHandler.DeleteByClientOrderID(symbol, orderType, clientOrderID)
			}
		} else if resp.Status == "EXPIRED" {
			logger.Info("EXPIRED, order=%s", resp.Order.FormatString())
			orderHandler.DeleteByClientOrderID(symbol, orderType, clientOrderID)
		} else if resp.Status == "CANCELED" {
			logger.Info("CANCELED, order=%s", resp.Order.FormatString())
			orderHandler.DeleteByClientOrderID(symbol, orderType, clientOrderID)
		} else if resp.Status == "NEW" {
			logger.Info("NEW, Exchange=Binance, Direction=%s, original price=%f, original amount=%f, OrderID=%s, ClientOrderID=%s",
				orderType, resp.Order.OrderPrice, resp.Order.OrderVolume, resp.Order.OrderID, clientOrderID)
			orderHandler.UpdateStatus(symbol, orderType, resp.Order.ClientOrderID, common.CREATED)
		}
	} else if resp.MsgType == "ACCOUNT_UPDATE" {
		// ACCOUNT_UPDATE 返回的仓位是全量信息
		if resp.Status == "ORDER_UPDATE" {
			symbol := resp.Symbol
			account := ctxt.Accounts.GetAccount(resp.Exchange, "swap_cross")

			account.UpdatePosition(symbol, resp.Position)
			logger.Warn("Binance position update, Symbol: %s, PositionMargin=%f",
				symbol, resp.Position)
		}
	}
}

func FuturesPriceHandler(resp *client.PriceWSResponse) {
	timeStamp := common.GetTimestampInMS()
	if resp.MsgType == "futuresBookTicker" {
		// 处理期货bookTicker
		bidPrice, askPrice := 0.0, 0.0
		bidVolume, askVolume := 0.0, 0.0
		for _, item := range resp.Items {
			if item.Direction == "buy" {
				bidPrice = item.Price
				bidVolume = item.Volume
			} else if item.Direction == "sell" {
				askPrice = item.Price
				askVolume = item.Volume
			}
		}
		logger.Debug("binance futures bookTicker symbol: %s, bidPrice:%f, bidVolume:%f, askPrice:%f, askVolume:%f, updateTime:%d", resp.Symbol, bidPrice, bidVolume, askPrice, askVolume, resp.TimeStamp)
		go UpdatePrice(resp.Symbol, bidPrice, bidVolume, askPrice, askVolume, resp.TimeStamp, timeStamp, "futures")

	} else if resp.MsgType == "futuresDepth" {
		config := &cfg
		bidPrice, askPrice := 0.0, 0.0
		for _, item := range resp.Bids {
			bidPrice, _ = item.Price.Float64()
		}
		for _, item := range resp.Asks {
			askPrice, _ = item.Price.Float64()
		}

		if bidPrice > config.MinAccuracy {
			logger.Debug("futuresDepth|%d|%d|%d",
				timeStamp, resp.TimeStamp, timeStamp-resp.TimeStamp)
		}

		if askPrice > config.MinAccuracy {
			logger.Debug("futuresDepth|%d|%d|%d",
				timeStamp, resp.TimeStamp, timeStamp-resp.TimeStamp)
		}
	}
}

// 更新参照组最新买卖价格
func UpdatePrice(symbol string, bidPrice float64, bidVolume float64,
	askPrice float64, askVolume float64, respTs int64, timestamp int64, ptype string) {
	context := &ctxt
	config := &cfg

	deliverySymbols := context.GetDeliverySymbol(symbol)
	for _, deliverySymbol := range deliverySymbols {
		name := common.FormatPriceName(cfg.Exchange, deliverySymbol, ptype)
		priceDataItem, ok := ctxt.Prices.Items[name]
		if !ok {
			continue
		}

		if bidPrice > config.MinAccuracy || askPrice > config.MinAccuracy {
			priceDataItem.LastUpdateTime = timestamp
		}

		if bidPrice > config.MinAccuracy {
			priceDataItem.BidPrice = bidPrice
			priceDataItem.BidVolume = bidVolume
			logger.Debug("%sBookTicker|%d|%d|%d",
				ptype, timestamp, respTs, timestamp-respTs)
		}

		if askPrice > config.MinAccuracy {
			priceDataItem.AskPrice = askPrice
			priceDataItem.AskVolume = askVolume
			logger.Debug("%sBookTicker|%d|%d|%d",
				ptype, timestamp, respTs, timestamp-respTs)
		}
	}
}

func SpotPriceHandler(resp *client.PriceWSResponse) {
	timeStamp := common.GetTimestampInMS()
	if resp.MsgType == "spotBookTicker" {
		// 处理期货bookTicker
		bidPrice, askPrice := 0.0, 0.0
		bidVolume, askVolume := 0.0, 0.0
		for _, item := range resp.Items {
			if item.Direction == "buy" {
				bidPrice = item.Price
				bidVolume = item.Volume
			} else if item.Direction == "sell" {
				askPrice = item.Price
				askVolume = item.Volume
			}
		}

		go UpdatePrice(resp.Symbol, bidPrice, bidVolume, askPrice, askVolume, resp.TimeStamp, timeStamp, "spot")
		logger.Debug("binance spot bookTicker symbol: %s, bidPrice:%f, bidVolume:%f, askPrice:%f, askVolume:%f, updateTime:%d", resp.Symbol, bidPrice, bidVolume, askPrice, askVolume, resp.TimeStamp)
	} else if resp.MsgType == "spotDepth" {
		config := &cfg
		bidPrice, askPrice := 0.0, 0.0
		for _, item := range resp.Bids {
			bidPrice, _ = item.Price.Float64()
		}
		for _, item := range resp.Asks {
			askPrice, _ = item.Price.Float64()
		}

		if bidPrice > config.MinAccuracy {
			logger.Debug("spotDepth|%d|%d|%d",
				timeStamp, resp.TimeStamp, timeStamp-resp.TimeStamp)
		}

		if askPrice > config.MinAccuracy {
			logger.Debug("spotDepth|%d|%d|%d",
				timeStamp, resp.TimeStamp, timeStamp-resp.TimeStamp)
		}
	}

}
