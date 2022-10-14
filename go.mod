module cex

go 1.18

require (
	github.com/adshao/go-binance/v2 v2.3.9
	github.com/go-telegram-bot-api/telegram-bot-api/v5 v5.5.1
	github.com/lestrrat-go/file-rotatelogs v2.4.0+incompatible
	github.com/shopspring/decimal v1.3.1
	go.uber.org/zap v1.23.0
	golang.org/x/time v0.0.0-20220922220347-f3bd1da661af
)

require (
	github.com/bitly/go-simplejson v0.5.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/jonboulle/clockwork v0.3.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/lestrrat-go/strftime v1.0.6 // indirect
	github.com/modern-go/concurrent v0.0.0-20180228061459-e0a39a4cb421 // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/multierr v1.6.0 // indirect
)

replace github.com/adshao/go-binance/v2 v2.3.9 => github.com/drinkthere/go-binance/v2 v2.3.5-0.20221014063512-fe67fe05a09b
