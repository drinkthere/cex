package config

import (
	"cex/common/logger"
	"math"
)

type DynamicConfig struct {
	PriceList              []float64
	AdjustedGapSize        float64
	AdjustedForgivePercent float64
}

var dynamicConfigs map[string]*DynamicConfig

func InitDynamicConfig(cfg *Config) {
	dynamicConfigs = map[string]*DynamicConfig{}
	for _, symbol := range cfg.Symbols {
		dynamicConfig := DynamicConfig{AdjustedForgivePercent: cfg.ForgivePercent}
		dynamicConfigs[symbol] = &dynamicConfig
	}
}

func UpdateDynamicConfigs(cfg *Config) {

	for symbol, dynamicConfig := range dynamicConfigs {
		UpdateDynamicConfig(symbol, dynamicConfig)
	}
}

func UpdateDynamicConfig(symbol string, dynamicConfig *DynamicConfig) {
	// TODO: read from config
	gapSizePercent := cfg.GapSizePercent
	spotForgive := cfg.SpotForgive
	spotForgiveParam := cfg.SpotForgiveParam

	priceContext := ctxt.GetPriceContext(symbol)
	if priceContext == nil || priceContext.DeliveryBidPrice < MinAccuracy {
		return
	}
	logger.Debug("DynamicConfig Symbol: %s, PriceContext: %+v", symbol, priceContext)
	if priceContext.DeliveryBidPrice > MinAccuracy {
		dynamicConfig.PriceList = append(dynamicConfig.PriceList, priceContext.DeliveryBidPrice)
	}
	gapSize := gapSizePercent * priceContext.DeliveryBidPrice

	spread := 0.0
	if len(dynamicConfig.PriceList) < 300 {
		dynamicConfig.AdjustedgapSize = gapSize
		dynamicConfig.AdjustedspotForgive = spotForgive
	} else if len(dynamicConfig.PriceList) >= 300 {
		max := common.MaxFloat64(dynamicConfig.PriceList)
		min := common.MinFloat64(dynamicConfig.PriceList)
		spread = (max - min) / priceContext.DeliveryBidPrice
		dynamicConfig.AdjustedgapSize = gapSize + gapSize*spread*cfg.SpreadConfig
		dynamicConfig.AdjustedspotForgive = spotForgive - (math.Pow((spread/100), 0.75))/spotForgiveParam
	}

	if len(dynamicConfig.PriceList) > 40000 {
		dynamicConfig.PriceList = dynamicConfig.PriceList[1:]
	}

	logger.Info("DynamicConfig Symbol: %s, Spread: %f, AdjustedgapSize: %f, AdjustedspotForgive: %f, Length: %d",
		symbol, spread, dynamicConfig.AdjustedgapSize, dynamicConfig.AdjustedspotForgive,
		len(dynamicConfig.PriceList))
}
