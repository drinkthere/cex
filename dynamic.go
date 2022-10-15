package main

import (
	"cex/common"
	"cex/common/logger"
	"cex/config"
	"math"
)

type DynamicConfig struct {
	PriceList              []float64
	AdjustedGapSize        float64
	AdjustedForgivePercent float64
}

var dynamicConfigs map[string]*DynamicConfig

func InitDynamicConfig(cfg *config.Config) {
	dynamicConfigs = map[string]*DynamicConfig{}
	for _, symbol := range cfg.Symbols {
		dynamicConfig := DynamicConfig{AdjustedForgivePercent: cfg.ForgivePercent}
		dynamicConfigs[symbol] = &dynamicConfig
	}
}
func GetDynamicConfig(symbol string) *DynamicConfig {
	dynamicConfig := dynamicConfigs[symbol]
	return dynamicConfig
}

func UpdateDynamicConfigs() {
	for symbol, dynamicConfig := range dynamicConfigs {
		UpdateDynamicConfig(symbol, dynamicConfig)
	}
}

func UpdateDynamicConfig(symbol string, dynamicConfig *DynamicConfig) {
	gapSizePercent := cfg.GapSizePercent
	forgivePercent := cfg.ForgivePercent

	symbolContext := ctxt.GetSymbolContext(symbol)
	if symbolContext == nil || symbolContext.BidPrice < cfg.MinAccuracy {
		return
	}
	logger.Debug("DynamicConfig Symbol: %s, SymbolContext: %+v", symbol, symbolContext)
	if symbolContext.BidPrice > cfg.MinAccuracy {
		dynamicConfig.PriceList = append(dynamicConfig.PriceList, symbolContext.BidPrice)
	}
	gapSize := gapSizePercent * symbolContext.BidPrice

	spread := 0.0
	if len(dynamicConfig.PriceList) < 300 {
		dynamicConfig.AdjustedGapSize = gapSize
		dynamicConfig.AdjustedForgivePercent = forgivePercent
	} else if len(dynamicConfig.PriceList) <= 3000 {
		max := common.MaxFloat64(dynamicConfig.PriceList)
		min := common.MinFloat64(dynamicConfig.PriceList)
		spread = (max - min) / symbolContext.BidPrice
		dynamicConfig.AdjustedGapSize = gapSize + gapSize*spread*cfg.SpreadTimes
		dynamicConfig.AdjustedForgivePercent = forgivePercent - (math.Pow((spread/cfg.ExponentBaseDenominator), cfg.ExponentPower))/cfg.Denominator
	}

	if len(dynamicConfig.PriceList) > 3000 {
		dynamicConfig.PriceList = dynamicConfig.PriceList[1:]
	}

	logger.Info("DynamicConfig Symbol: %s, Spread: %f, AdjustedGapSize: %f, AdjustedForgivePercent: %f, Length: %d",
		symbol, spread, dynamicConfig.AdjustedGapSize, dynamicConfig.AdjustedForgivePercent,
		len(dynamicConfig.PriceList))
}
