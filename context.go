package main

import (
	"cex/common"
	"cex/common/logger"
	"cex/config"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// 币本位上下文
type SymbolContext struct {
	Symbol            string  // 交易对
	BidPrice          float64 // 买价
	BidAmount         float64 // 买量
	AskPrice          float64 // 卖价
	AskAmount         float64 // 卖量
	LastUpdateTime    int64   // 单位：ms，如果更新时间超过阈值，Risk设置成3，取消全部订单，并暂停下单直到恢复
	LastCancelTime    int64   // 单位：ms，用来控制取消订单的频率
	LastCancelFarTime int64   // 单位：ms，用来控制取消远距离订单的频率
	Risk              int     // 风险控制：0可以挂单，1表示出错，2表示处于结算时间，3系统暂停等待价格更新，4超过最大仓位
}

func (context *SymbolContext) Init(deliverySymbol string) {
	// init prices
	context.Symbol = deliverySymbol
	context.BidPrice = 0.0
	context.BidAmount = 0.0
	context.AskPrice = 0.0
	context.AskAmount = 0.0
	context.LastUpdateTime = common.GetTimestampInMS()
	context.LastCancelTime = 0
	context.LastCancelFarTime = 0
	context.Risk = 0
}

type PriceDataItem struct {
	Symbol    string  // 交易对
	BidPrice  float64 // 买价
	BidAmount float64 // 买量
	AskPrice  float64 // 卖价
	AskAmount float64 // 卖量

	LastUpdateTime int64 // 单位：ms
}

type PriceData struct {
	Items map[string]*PriceDataItem
}

// 整体上下文
type Context struct {
	Accounts common.Accounts // 账户信息

	Risk           int                 // 风险控制：0可以挂单，1表示出错，2表示处于结算时间，3系统启动等待价格更新
	Symbols        []string            // 支持多个交易对，如：BTCUSD_PERP, BTCUSD_0930
	symbolContexts []*SymbolContext    // 交易对的上下文
	SymbolMap      map[string][]string // 标准symbol对应到币本位symbol(一对多)

	MinDeltaRate float64          // 最小差比例， 价格变动超过这个才进行处理
	TelegramBot  *tgbotapi.BotAPI // 电报机器人

	// 其它交易模块价格数据，比如当前做的是币本位，则这里放U本位和现货的价格，也可以放其他交易所的价格做辅助
	Prices PriceData
}

func (context *Context) Init(cfg *config.Config) {
	context.Risk = 0

	// SymbolMap 存的是 U本位交易对和1至多个币本位交易对 的映射
	// SymbolMap["AVAXUSDT"] => ["AVAXUSD_PERP", "AVAXUSD_220930"]
	context.SymbolMap = map[string][]string{}
	context.Prices.Items = make(map[string]*PriceDataItem)
	for _, symbol := range cfg.Symbols {
		// symbol 是币本位的symbol 如：BTCUSD_PERP, BTCUSD_0930
		context.Symbols = append(context.Symbols, symbol)
		// 初始化币本位symbol的上下文
		context.GetSymbolContext(symbol)

		symbolCfg := cfg.SymbolConfig[symbol]
		// futuresSymbol 是U本位的symbol，如: BTCBUSD, 这里做个映射，方便后面获取对应值
		futuresSymbol := common.FormatFuturesSymbol(symbol, symbolCfg.QuoteAsset)
		// SymbolMap["BTCBUSD"] => ["BTCUSD_PERP", "BTCUSD_0930"]
		context.SymbolMap[futuresSymbol] = append(context.SymbolMap[futuresSymbol], symbol)

		// 现货
		spotKey := common.FormatPriceName(cfg.Exchange, symbol, "spot")
		ctxt.Prices.Items[spotKey] = &PriceDataItem{Symbol: symbol}

		// U本位永续
		futuresKey := common.FormatPriceName(cfg.Exchange, symbol, "futures")
		ctxt.Prices.Items[futuresKey] = &PriceDataItem{Symbol: symbol}
	}

	context.Accounts.AddAccount(cfg.Exchange, cfg.SwapType)

	context.MinDeltaRate = cfg.MinDeltaRate

	// 初始化 telegramBot
	bot, err := tgbotapi.NewBotAPI(cfg.TgBotToken)
	if err != nil {
		logger.Error("init telegram bot failed")
		ExitProcess()
	}
	context.TelegramBot = bot
}

// 获取币本位交易对的上下文
func (context *Context) GetSymbolContext(deliverySymbol string) *SymbolContext {
	size := len(context.symbolContexts)
	for i := 0; i < size; i++ {
		if deliverySymbol == context.symbolContexts[i].Symbol {
			return context.symbolContexts[i]
		}
	}

	symbolContext := SymbolContext{}
	symbolContext.Init(deliverySymbol)
	context.symbolContexts = append(context.symbolContexts, &symbolContext)

	return context.symbolContexts[len(context.symbolContexts)-1]
}

// 获取对照组价格
func (context *Context) GetPriceItem(exchange string, symbol string, product string) *PriceDataItem {
	name := common.FormatPriceName(exchange, symbol, product)
	item, ok := context.Prices.Items[name]
	if !ok {
		return nil
	}
	return item
}
