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
	//binanceDeliveryWSClient.SetOrderHandler(DeliveryOrderWSHandler)
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
	handler.wsClient = append(handler.wsClient, binanceFuturesWSClient)

	// 初始化币安现货的 WS client
	binanceSpotWSClient := new(client.BinanceSpotWSClient)
	binanceSpotWSClient.Init(binanceConfig)
	binanceSpotWSClient.SetPriceHandler(SpotPriceHandler, common.CommonErrorHandler)
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
	config := &cfg

	timeStamp := common.GetTimestampInMS()
	if resp.MsgType == "deliveryBookTicker" {
		// 处理期货bookTicker
		bidPrice, askPrice := 0.0, 0.0
		for _, item := range resp.Items {
			if item.Direction == "buy" {
				bidPrice = item.Price
			} else if item.Direction == "sell" {
				askPrice = item.Price
			}
		}
		if bidPrice > config.MinAccuracy {
			logger.Info("deliveryBookTicker|%d|%d|d",
				timeStamp, resp.TimeStamp, timeStamp-resp.TimeStamp)
		}

		if askPrice > config.MinAccuracy {
			logger.Info("deliveryBookTicker|%d|%d|d",
				timeStamp, resp.TimeStamp, timeStamp-resp.TimeStamp)
		}
	}
}

//func DeliveryOrderWSHandler(resp *client.OrderWSResponse) {
//	context := &ctxt
//	config := &cfg
//	symbol := resp.Order.Symbol
//	symbolCfg := config.SymbolConfigs[symbol]
//
//	logger.Info("binance delivery order resp is: %+v", resp)
//	if resp.MsgType == "ORDER_TRADE_UPDATE" {
//		orderType := resp.Order.OrderType
//		clientOrderID := resp.Order.ClientOrderID
//		// 订单成交
//		if resp.Status == "PARTIALLY_FILLED" || resp.Status == "FILLED" {
//			deliveryContext := context.GetSymbolContext(resp.Order.Symbol)
//			spotPriceItem := ctxt.GetPriceItem(config.Exchange, symbol, "spot")
//			logger.Info("Op=Fill, Exchange=Binance, Direction=%s, filled price=%f, amount=%f, OrderID=%d, ClientOrderID=%s, BuyPrice=%.2f, SellPrice=%.2f, Symbol=%s, sBidPrice=%.4f, sAskPrice=%.4f",
//				resp.Order.OrderType, resp.Order.OrderPrice, resp.Order.OrderVolume, resp.Order.OrderID,
//				resp.Order.ClientOrderID, deliveryContext.BidPrice, deliveryContext.AskPrice,
//				symbol, spotPriceItem.BidPrice, spotPriceItem.AskPrice)
//
//			// 下单对冲
//			if config.FunctionHedge == 1 {
//				hedgeOrderType := common.GetHedgeOrderType(orderType)
//				// 合约张数
//				volume := resp.Order.OrderVolume
//				cont := float64(symbolCfg.Cont)
//				price := 0.0
//				if hedgeOrderType == "buy" {
//					price = spotPriceItem.BidPrice
//				} else {
//					price = spotPriceItem.AskPrice
//				}
//				amount := volume * cont / price
//				logger.Info("===volume:%s, price:%s, amount:%s, minHedgeSize:%s, hedgeOrderType: %s", volume, price, amount, symbolCfg.MinHedgeSize, hedgeOrderType)
//
//				if volume >= symbolCfg.MinHedgeSize {
//					var order common.Order
//					order.Exchange = "Binance"
//					order.OrderType = hedgeOrderType
//					order.OrderPrice = price
//					order.OrderVolume = amount
//					order.Symbol = resp.Order.Symbol
//					order.ClientOrderID = resp.Order.ClientOrderID
//					order.BaseAsset = symbolCfg.BaseAsset
//					order.QuoteAsset = config.QuoteAsset
//					order.Precision = symbolCfg.Precision
//					orderHandler.PlaceHedgeOrder(&order)
//				}
//			}
//
//			if resp.Status == "FILLED" {
//				orderHandler.DeleteByClientOrderID(symbol, orderType, clientOrderID)
//			}
//		} else if resp.Status == "EXPIRED" {
//			logger.Info("EXPIRED, order=%s", resp.Order.FormatString())
//			orderHandler.DeleteByClientOrderID(symbol, orderType, clientOrderID)
//		} else if resp.Status == "CANCELED" {
//			logger.Info("CANCELED, order=%s", resp.Order.FormatString())
//			orderHandler.DeleteByClientOrderID(symbol, orderType, clientOrderID)
//		} else if resp.Status == "NEW" {
//			logger.Info("NEW, Exchange=Binance, Direction=%s, original price=%f, original amount=%f, OrderID=%s, ClientOrderID=%s",
//				orderType, resp.Order.OrderPrice, resp.Order.OrderVolume, resp.Order.OrderID, clientOrderID)
//			orderHandler.UpdateStatus(symbol, orderType, resp.Order.ClientOrderID, common.CREATED)
//		}
//	} else if resp.MsgType == "ACCOUNT_UPDATE" {
//		// ACCOUNT_UPDATE 返回的仓位是全量信息
//		if resp.Status == "ORDER_UPDATE" {
//			symbol := resp.Symbol
//			account := ctxt.Accounts.GetAccount(resp.Exchange, "swap_cross")
//
//			account.UpdatePosition(symbol, resp.Position)
//			logger.Warn("Binance position update, Symbol: %s, PositionMargin=%f",
//				symbol, resp.Position)
//		}
//	}
//}

func FuturesPriceHandler(resp *client.PriceWSResponse) {
	config := &cfg

	timeStamp := common.GetTimestampInMS()
	if resp.MsgType == "futuresBookTicker" {
		// 处理期货bookTicker
		bidPrice, askPrice := 0.0, 0.0
		for _, item := range resp.Items {
			if item.Direction == "buy" {
				bidPrice = item.Price
			} else if item.Direction == "sell" {
				askPrice = item.Price
			}
		}
		if bidPrice > config.MinAccuracy {
			logger.Info("futuresBookTicker|%d|%d|d",
				timeStamp, resp.TimeStamp, timeStamp-resp.TimeStamp)
		}

		if askPrice > config.MinAccuracy {
			logger.Info("futuresBookTicker|%d|%d|d",
				timeStamp, resp.TimeStamp, timeStamp-resp.TimeStamp)
		}
	}
}

func SpotPriceHandler(resp *client.PriceWSResponse) {
	config := &cfg

	timeStamp := common.GetTimestampInMS()
	if resp.MsgType == "spotBookTicker" {
		// 处理期货bookTicker
		bidPrice, askPrice := 0.0, 0.0
		for _, item := range resp.Items {
			if item.Direction == "buy" {
				bidPrice = item.Price
			} else if item.Direction == "sell" {
				askPrice = item.Price
			}
		}
		if bidPrice > config.MinAccuracy {
			logger.Info("spotBookTicker|%d|%d|d",
				timeStamp, resp.TimeStamp, timeStamp-resp.TimeStamp)
		}

		if askPrice > config.MinAccuracy {
			logger.Info("spotBookTicker|%d|%d|d",
				timeStamp, resp.TimeStamp, timeStamp-resp.TimeStamp)
		}
	}
}
