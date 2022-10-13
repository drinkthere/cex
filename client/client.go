package client

import (
	"cex/common"

	"github.com/shopspring/decimal"
)

// 配置信息，用于Client初始化
type Config struct {
	Internal  bool // 是否使用内网
	AccessKey string
	SecretKey string
	Symbols   []string // 多币种

	VolumePerOneContract  float64 // 火币需要设置每张对应的数量
	APILimit              int     // API次数限制（3s）
	LimitProcess          int     // 超过限制请求的处理方法，1等待，0是丢弃该次请求
	IsSwapCross           bool    // 全仓或者逐仓，true 全仓， false 逐仓
	ListenFuturesOrderMsg bool    //  是否监听U本位合约订单 order 信息
}

type OrderClient interface {
	Init(config Config) bool
}

// ------------------以下是websocket相关的内容

type WSResponse struct {
	Exchange  string // 消息对应的交易所，Huobi、Binance
	MsgType   string // 消息类型
	TimeStamp int64  // 时间戳，时间单位ms
	UpdateID  int64  // 上次更信的 id
}

type PriceItem struct {
	Price     float64
	Direction string  // 价格方向：buy sell
	Volume    float64 // 交易量
}

type DepthPriceItem struct {
	Price        decimal.Decimal
	Volume       decimal.Decimal // 交易量
	LastUpdateID int64
}

// 价格变化消息
type PriceWSResponse struct {
	WSResponse
	Symbol string
	Items  []PriceItem
	// 增量depth价格
	Bids []DepthPriceItem
	Asks []DepthPriceItem
}

// 订单变化消息
type OrderWSResponse struct {
	WSResponse
	Order       common.Order
	Status      string // 订单状态：NEW新创建，PARTIALLY_FILLED部分成交，FILLED全部成交，CANCEL取消
	Symbol      string
	Position    float64
	PositionAbs float64
}

// 处理ws消息返回的数据
type ProcessHandler func(resp *WSResponse)

// 处理price ws消息返回的数据
type PriceProcessHandler func(resp *PriceWSResponse)

// 处理order ws消息返回的数据
type OrderProcessHandler func(resp *OrderWSResponse)

// 处理ws消息抛出的错误
type ErrorHandler func(exchange string, error int)

type WSClient interface {
	// 初始化
	Init(config Config) bool
	SetPriceHandler(handler PriceProcessHandler, errHandler ErrorHandler)
	SetOrderHandler(handler OrderProcessHandler)
	// 启动webscoket
	StartWS() bool
	// 停止webscoket
	StopWS() bool
}
