package log

import (
	"os"
	"tiny_cni/internal/constexpr"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func getLogEncoder() zapcore.EncoderConfig {

	encoderCfg := zap.NewDevelopmentEncoderConfig()
	encoderCfg.EncodeTime = zapcore.RFC3339NanoTimeEncoder
	encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	return encoderCfg
}

var (
	Log = zap.S()
)

func InitLog(enableLog bool, useTerminal bool) {
	if enableLog {
		Log = zap.NewNop().Sugar()
		return
	}
	encoderCfg := getLogEncoder()
	if !useTerminal {
		file, _ := os.OpenFile("/tmp/1.log", os.O_APPEND|os.O_CREATE, 0644)
		writeSyncer := zapcore.AddSync(file)
		encoder := zapcore.NewJSONEncoder(encoderCfg)
		core := zapcore.NewCore(encoder, writeSyncer, zapcore.DebugLevel)
		Log = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zap.ErrorLevel)).Sugar()
		return
	}
	loggerCfg := zap.NewDevelopmentConfig()
	loggerCfg.EncoderConfig = encoderCfg
	lg, err := loggerCfg.Build(zap.AddCaller(), zap.AddStacktrace(zap.FatalLevel))
	if err != nil {
		os.Exit(-1)
	}
	Log = lg.Sugar()

}
func init() {
	InitLog(constexpr.EnableLog, constexpr.LogOutputToTerminal)
}
