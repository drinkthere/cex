package config

import (
	"encoding/json"
	"os"

	"go.uber.org/zap/zapcore"
)

type SymbolConfig struct {
	ContractNum    int     // 每单委托数量（单位：张）
	BaseAsset      string  // eg. Base Asset: BTC
	InitValue      float64 // Base Asset初始数量，计算利润时会用到
	InitHedgeValue float64 // Base Asset 在对冲账号中的初始数量，计算利润时会用到
	Cont           int     // 每张多少 u，因为币本位是按照张算的，BTC一张100u，其他一张10u
	Leverage       int     // 杠杆倍数，初始化时给交易对设置好
	MaxContractNum int     // 最大可以开的张数， 也是用来限制单方向最大持仓数量的
	MinHedgeSize   float64 // 最小对冲数量, 需要对冲时，如果不够这个量就不对冲。e.g. 币安限制BTC最小交易额度是0.001
	Precision      [2]int  // BTCBUSD => [4, 2] BTC的精度是4，USD的精度是2
	EffectiveNum   float64 // 获取交易对报价时，quantity 需要大于这个值才认为有效（特别是从depth消息中获取价格时）
}

type Config struct {
	// 日志配置
	LogLevel zapcore.Level
	LogPath  string

	// 电报配置
	TgBotToken string
	TgChatID   int64

	// 币安配置
	BinanceAPIKey    string
	BinanceSecretKey string

	// 频率控制
	APILimit     int // API次数限制（1s）
	LimitProcess int // 超过限制请求的处理方法，1等待，0是丢弃该次请求

	// 套利配置
	Exchange      string                  // 交易所，在哪个交易所挂单， e.g. Binance
	SwapType      string                  // 全仓 swap_cross, 逐仓swap e.g. swap_cross
	Symbols       []string                // 要套利的交易对
	SymbolConfigs map[string]SymbolConfig // 交易对的详细配置

	MaxOrderNum     int     // 每个方向上最多挂单的数量
	MaxOrderOneStep int     // 一次挂单，每个方向最多挂几单
	GapSizePercent  float64 // 每单之间的默认间隔，e.g. 0.0002 就是间隔万分之二
	GapSizeK        float64 // 订单之间的滑动系数，确保间隔越来越大
	SpreadTimes     float64 // AdjustedGapSize = GapSizePercent * (1 + spread * SpreadTimes), 其中 spread = (max - min)/bidPrice

	// 通过一段时间价格的波动，使用以下公式来对forgive进行调整，因为 套利价差比 > forgive 才会下单，所以可以间接调整下单的难以程度
	ForgivePercent          float64 // 套利价差比 > forgive才下单，forgive会随着spread的变化而变动
	ExponentBaseDenominator float64 // (math.Pow((spread/100), 0.75))/4 公式中的100
	ExponentPower           float64 // (math.Pow((spread/100), 0.75))/4 公式中的0.75
	Denominator             float64 // (math.Pow((spread/100), 0.75))/4 公式中的中的4

	TickerShift         float64 // 根据仓位修正现货和U本位合约买卖价格时的系数
	CancelShift         float64 // 根据仓位调整撤销订单的距离， 撤销订单的条件要比挂单的条件严格一些，不然挂单之后马上撤单浪费API限额
	QuoteAsset          string  // Quote Asset: BUSD
	InitQuoteAssetValue float64 // 初始 BUSD/USDT 数量， 统计利润时会用到

	FunctionHedge      int     // 是否启动对冲功能
	MaxErrorsPerMinute int64   // 每分钟允许出现的 Error 日志数量（超出数量之后退出程序）
	MinDeltaRate       float64 // 最小差比例， 价格变动超过这个才进行处理
	MinAccuracy        float64 // 价格最小精度
	Commission         float64 // 手续费返点
	Loss               float64 // 让利亏损
}

func LoadConfig(filename string) *Config {
	config := new(Config)
	reader, err := os.Open(filename)
	if err != nil {
		panic(err)
	}
	defer reader.Close()

	// 加载配置
	decoder := json.NewDecoder(reader)
	err = decoder.Decode(&config)
	if err != nil {
		panic(err)
	}

	return config
}
