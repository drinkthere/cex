package config

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
