package common

import "cex/common/logger"

// 处理接口的各种错误
func CommonErrorHandler(exchange string, errno int) {
	logger.Error("%s errno: %d", exchange, errno)
}
