package main

import (
	"cex/common/logger"
	"fmt"
	"math"
	"strconv"

	"github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/delivery"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type AccountStatInfo struct {
	symbol          string
	balance         float64
	averageLeverage float64
	initValue       float64
	positionAmt     float64
}

// 获取并更新账户信息
func UpdateAccount() {
	account := orderHandler.BinanceDeliveryOrderClient.GetAccount()
	hedgeAccount := orderHandler.BinanceSpotOrderClient.GetAccount()
	message := ""

	// update to accountInfo
	accountInfo := ctxt.Accounts.GetAccount(cfg.Exchange, cfg.SwapType)
	for _, position := range account.Positions {
		symbol := position.Symbol
		accountInfo.GetPositionsInfo(symbol).Position, _ = strconv.ParseFloat(position.PositionAmt, 64)
		accountInfo.GetPositionsInfo(symbol).PositionAbs = math.Abs(accountInfo.GetPositionsInfo(symbol).Position)
	}

	accountStatInfo := map[string]*AccountStatInfo{}
	hedgeStatInfo := map[string]*AccountStatInfo{}

	for symbol, symbolConfig := range cfg.SymbolConfigs {
		accountStatInfo[symbolConfig.BaseAsset] = &AccountStatInfo{symbol: symbol, initValue: symbolConfig.InitValue, positionAmt: 0}
		hedgeStatInfo[symbolConfig.BaseAsset] = &AccountStatInfo{symbol: symbol, initValue: symbolConfig.InitHedgeValue, positionAmt: 0}
	}
	hedgeStatInfo[cfg.QuoteAsset] = &AccountStatInfo{symbol: "", initValue: cfg.InitQuoteAssetValue, positionAmt: 0}

	getProfit(account, accountStatInfo)
	getHedgeProfit(hedgeAccount, hedgeStatInfo)
	accountTotalProfitInUSD := 0.0
	for asset, item := range accountStatInfo {
		dBalance := item.balance
		sBalance := hedgeStatInfo[asset].balance
		profit := dBalance + sBalance - item.initValue - hedgeStatInfo[asset].initValue
		spotPriceItem := ctxt.GetPriceItem(cfg.Exchange, item.symbol, "spot")
		if profit > 0 {
			accountTotalProfitInUSD += profit * spotPriceItem.BidPrice
		} else {
			accountTotalProfitInUSD += profit * spotPriceItem.AskPrice
		}
		message += fmt.Sprintf("profit%s=%.4f, ", asset, profit)
	}
	quoteAssetBalance := hedgeStatInfo[cfg.QuoteAsset].balance
	logger.Warn("%sBalance=%s, initValue=%s", cfg.QuoteAsset, quoteAssetBalance, hedgeStatInfo[cfg.QuoteAsset].initValue)
	quoteAssetProfit := quoteAssetBalance - hedgeStatInfo[cfg.QuoteAsset].initValue
	logger.Warn("%sProfit=%s", cfg.QuoteAsset, quoteAssetProfit)
	accountTotalProfitInUSD += quoteAssetProfit
	message += fmt.Sprintf("profit in %s=%.4f, ", cfg.QuoteAsset, quoteAssetProfit)

	getAverageLeverage(account, accountStatInfo)
	for symbol, item := range accountStatInfo {
		// BNB 用来抵扣手续费，不算抵押资产
		if symbol == "BNB" {
			continue
		}
		message += fmt.Sprintf("AverageLeverage%s=%.2f, ", symbol, item.averageLeverage)
	}

	message += fmt.Sprintf("TotalProfitInUSD=%.2f, ", accountTotalProfitInUSD)
	if ctxt.Risk == 4 {
		isSmall := true
		for _, item := range accountStatInfo {
			symbolCfg := cfg.SymbolConfigs[item.symbol]
			if item.averageLeverage > float64(symbolCfg.Leverage) {
				isSmall = false
				break
			}
		}
		if isSmall {
			ctxt.Risk = 0
			logger.Error("Leverage is smaller then max leverage, resume place order!")
		}
	}
	if ctxt.Risk == 0 {
		for _, item := range accountStatInfo {
			symbolCfg := cfg.SymbolConfigs[item.symbol]
			if item.averageLeverage > float64(symbolCfg.Leverage) {
				ctxt.Risk = 4
				logger.Error("Leverage is bigger then max leverage, stop place order!")
			}
		}
	}

	if message != "" {
		logger.Warn(message)
		msg := tgbotapi.NewMessage(cfg.TgChatID, message)
		_, err := ctxt.TelegramBot.Send(msg)
		if err != nil {
			logger.Warn("send stat message failed, error: %+v", err)
		}
	}
}

func getProfit(account *delivery.Account, statInfo map[string]*AccountStatInfo) {
	if account == nil {
		return
	}
	if len(account.Assets) > 0 {
		for _, asset := range account.Assets {
			item, ok := statInfo[asset.Asset]
			if !ok {
				continue
			}
			item.balance, _ = strconv.ParseFloat(asset.MarginBalance, 64)
		}
	}
}

// spot用来对冲
func getHedgeProfit(account *binance.Account, hedgeStatInfo map[string]*AccountStatInfo) {
	if account == nil {
		return
	}
	if len(account.Balances) > 0 {
		for _, asset := range account.Balances {
			item, ok := hedgeStatInfo[asset.Asset]
			if !ok {
				continue
			}
			item.balance, _ = strconv.ParseFloat(asset.Free, 64)
		}
	}
}

func getAverageLeverage(account *delivery.Account, statInfo map[string]*AccountStatInfo) {
	if account == nil {
		return
	}
	if len(account.Positions) > 0 {
		for _, position := range account.Positions {
			asset := cfg.SymbolConfigs[position.Symbol].BaseAsset
			item, ok := statInfo[asset]
			if !ok {
				continue
			}

			tmp, _ := strconv.ParseFloat(position.PositionAmt, 64)
			if tmp == 0 {
				continue
			}
			item.positionAmt += tmp
			spotPriceItem := ctxt.GetPriceItem(cfg.Exchange, position.Symbol, "spot")
			item.averageLeverage = item.positionAmt * float64(cfg.SymbolConfigs[position.Symbol].Cont) / spotPriceItem.BidPrice / item.balance
		}
	}
}
