package common

import (
	"fmt"
	"sync"
	"time"
)

// 订单状态
const (
	NEW             int = iota // 新创建（还未提交）
	CREATE                     // 已经成功提交
	CREATED                    // 创建成功
	CANCEL                     // 已经提交取消（还未取消成功）
	CANCELED                   // 已取消
	PARTIALLYFILLED            // 部分成交
	FILLED                     // 全部成交
	FAILED                     // 创建失败
)

type Order struct {
	Exchange      string // 交易所
	Symbol        string
	OrderType     string // 订单类型: buy sell
	OrderID       string
	OrderPrice    float64
	OrderVolume   float64 // 用于现货U本位是amount，合约是张数
	CreateAt      int64
	ClientOrderID string
	BaseAsset     string // BTCBUSD 中 BTC是BaseAsset，BUSD是QuoteAsset
	QuoteAsset    string
	Precision     [2]int // //  [4, 2], 以BTCBUSD为例，BTC的精度是4，BUSD的精度是2
	Status        int    // 订单状态
}

func (order *Order) FormatString() string {
	str := fmt.Sprintf("OrderID=%s, ClientOrderID=%s, OrderType=%s, Price=%v, Volume=%v, Symbol=%s",
		order.OrderID, order.ClientOrderID, order.OrderType, order.OrderPrice,
		order.OrderVolume, order.Symbol)
	return str
}

type OrderList []*Order

type OrderBook struct {
	Data           OrderList
	canceledOrders map[string]*Order // 已经取消的订单
	Mutex          sync.RWMutex
}

// 通过ClientOrderID删除对应的Order
func (orderBook *OrderBook) DeleteByClientOrderID(clientOrderID string) *Order {
	orderBook.Mutex.Lock()
	defer orderBook.Mutex.Unlock()
	var order *Order = nil
	for i := 0; i < len(orderBook.Data); {
		if orderBook.Data[i].ClientOrderID == clientOrderID {
			order = orderBook.Data[i]
			orderBook.Data = append(orderBook.Data[:i], orderBook.Data[i+1:]...)
			orderBook.canceledOrders[order.ClientOrderID] = order
			break
		} else {
			i++
		}
	}

	if len(orderBook.canceledOrders) > 50 {
		timeStamp := time.Now().Unix()
		for id, order := range orderBook.canceledOrders {
			if timeStamp-order.CreateAt > 10 {
				delete(orderBook.canceledOrders, id)
			}
		}
	}

	return order
}

// 更新order状态
func (orderBook *OrderBook) UpdateStatus(clientOrderID string, status int) *Order {
	orderBook.Mutex.Lock()
	defer orderBook.Mutex.Unlock()
	for i := 0; i < len(orderBook.Data); i++ {
		if orderBook.Data[i].ClientOrderID == clientOrderID {
			orderBook.Data[i].Status = status
			return orderBook.Data[i]
		}
	}
	return nil
}
