package log

import (
	"fmt"
	"os"
	"time"
	"tiny_cni/internal/constexpr"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func getLogEncoder(useTerminal bool) zapcore.EncoderConfig {

	encoderCfg := zap.NewDevelopmentEncoderConfig()
	if useTerminal {
		encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	encoderCfg.EncodeTime = zapcore.RFC3339NanoTimeEncoder
	return encoderCfg
}

var (
	Log *zap.SugaredLogger
)

func InitLog(enableLog bool, useTerminal bool, prefix string) {
	if !enableLog {
		Log = zap.NewNop().Sugar()
		return
	}
	encoderCfg := getLogEncoder(useTerminal)
	if !useTerminal {
		name := fmt.Sprintf("/tmp/%s-%s-%d.log", prefix, time.Now().Format(time.RFC3339), os.Getpid())
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
	//OutPutEnv()
}
func init() {
	InitLog(constexpr.EnableLog, true, "debug")
}
func OutPutEnv() {
	env := os.Environ()
	Log.Debug(env)
}
