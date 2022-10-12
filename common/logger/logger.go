package logger

import (
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var sugaredLogger *zap.SugaredLogger

// 初始化日志
func InitLogger(logPath string, level zapcore.Level) {
	encoderCfg := zapcore.EncoderConfig{
		TimeKey:      "time",
		MessageKey:   "msg",
		LevelKey:     "level",
		CallerKey:    "caller",
		EncodeLevel:  zapcore.CapitalColorLevelEncoder,
		EncodeTime:   zapcore.ISO8601TimeEncoder,
		EncodeCaller: zapcore.ShortCallerEncoder,
	}

	atomicLevel := zap.NewAtomicLevel()
	atomicLevel.SetLevel(level)

	writer := getWriter(logPath)

	core := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderCfg), writer, atomicLevel)
	sugaredLogger = zap.New(core).Sugar()

	if level == zapcore.DebugLevel {
		// 打印行号
		sugaredLogger = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1)).Sugar()
	} else {
		sugaredLogger = zap.New(core).Sugar()
	}
}

func getWriter(filename string) zapcore.WriteSyncer {
	hook, err := rotatelogs.New(
		filename+".%Y%m%d%H",
		rotatelogs.WithLinkName(filename),
		rotatelogs.WithMaxAge(time.Hour*24*7),
		rotatelogs.WithRotationTime(8*time.Hour), // 一天一个日志文件
	)

	if err != nil {
		panic(err)
	}
	return zapcore.AddSync(hook)
}

func Fatal(template string, args ...interface{}) {
	sugaredLogger.Fatalf(template, args...)
}

func Error(template string, args ...interface{}) {
	sugaredLogger.Errorf(template, args...)
}

func Panic(template string, args ...interface{}) {
	sugaredLogger.Panicf(template, args...)
}

func Warn(template string, args ...interface{}) {
	sugaredLogger.Warnf(template, args...)
}

func Info(template string, args ...interface{}) {
	sugaredLogger.Infof(template, args...)
}

func Debug(template string, args ...interface{}) {
	sugaredLogger.Debugf(template, args...)
}
