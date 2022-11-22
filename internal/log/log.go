package log

import (
	"fmt"
	"os"
	"time"
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
	Log *zap.SugaredLogger
)

func InitLog(enableLog bool, useTerminal bool) {
	if !enableLog {
		Log = zap.NewNop().Sugar()
		return
	}
	encoderCfg := getLogEncoder()
	if !useTerminal {
		name := fmt.Sprintf("/tmp/tcni-%s-%d.log", time.Now().Format(time.RFC3339), os.Getpid())
		file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			panic(err)
		}
		writeSyncer := zapcore.AddSync(file)
		encoder := zapcore.NewConsoleEncoder(encoderCfg)
		core := zapcore.NewCore(encoder, writeSyncer, zapcore.DebugLevel)
		Log = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zap.ErrorLevel)).Sugar()
		OutPutEnv()
		return
	}
	loggerCfg := zap.NewDevelopmentConfig()
	loggerCfg.EncoderConfig = encoderCfg
	lg, err := loggerCfg.Build(zap.AddCaller(), zap.AddStacktrace(zap.FatalLevel))
	if err != nil {
		os.Exit(-1)
	}
	Log = lg.Sugar()
	OutPutEnv()
}
func init() {
	InitLog(constexpr.EnableLog, constexpr.LogOutputToTerminal)
}
func OutPutEnv() {
	env := os.Environ()
	Log.Debug(env)
}
