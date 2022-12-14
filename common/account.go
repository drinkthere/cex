// 账户信息
package common

import (
	"math"
	"math/big"
)

// 持仓信息
type DeliveryPosition struct {
	Symbol string

	PositionAbs float64 // 持仓量（绝对值，不管方向）
	Position    float64 // 持仓量（多正，空负）

}

// 账户中的币和数量
type TokenInfo struct {
	Symbol string  // token对应的Symbol
	Amount big.Int // token数量
}

// 账户信息
type AccountInfo struct {
	Exchange          string                       // 交易所
	SwapType          string                       // swap 逐仓，swap_cross 全仓
	Margin            float64                      // 总的金额
	DeliveryPositions map[string]*DeliveryPosition // 持仓合约的具体数量
	Tokens            []TokenInfo
}

func (account *AccountInfo) Init(exchange string, swapType string) {
	account.Exchange = exchange
	account.SwapType = swapType
	account.Margin = 0
	account.DeliveryPositions = map[string]*DeliveryPosition{}
}

func (account *AccountInfo) GetPositionsInfo(symbol string) *DeliveryPosition {
	position, ok := account.DeliveryPositions[symbol]
	if !ok {
		account.DeliveryPositions[symbol] = &DeliveryPosition{Symbol: symbol, PositionAbs: 0, Position: 0}
		position = account.DeliveryPositions[symbol]
	}

	return position
}

func (account *AccountInfo) UpdatePosition(symbol string, positionMargin float64) {
	positionInfo := account.GetPositionsInfo(symbol)
	positionInfo.Position = positionMargin
	positionInfo.PositionAbs = math.Abs(positionMargin)
}

// 账户信息，有可能会有多个账号，如：用不同的账号进行对冲
type Accounts struct {
	Data []AccountInfo
}

// add one account
func (accounts *Accounts) AddAccount(exchange string, swapType string) {
	account := AccountInfo{}
	account.Init(exchange, swapType)
	accounts.Data = append(accounts.Data, account)
}

// get account by exchange
func (accounts *Accounts) GetAccount(exchange string, swapType string) *AccountInfo {
	size := len(accounts.Data)
	for i := 0; i < size; i++ {
		if exchange == accounts.Data[i].Exchange && swapType == accounts.Data[i].SwapType {
			return &accounts.Data[i]
		}
	}
	return nil
}
