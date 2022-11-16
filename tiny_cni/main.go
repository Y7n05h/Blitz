package main

import (
	"fmt"
	"os"
	"runtime"
	"tiny_cni/tiny_cni/internal/pkg/config"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/version"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	Program   = "tiny_cni"
	Version   = "0.0.1"
	enableLog = true
)

func init() {
	var lg *zap.Logger
	if enableLog {
		lg = zap.NewNop()
	} else {
		file, _ := os.OpenFile("/tmp/1.log", os.O_APPEND|os.O_CREATE, 0644)
		writeSyncer := zapcore.AddSync(file)
		encoderCfg := zap.NewDevelopmentEncoderConfig()
		encoderCfg.EncodeTime = zapcore.RFC3339NanoTimeEncoder
		encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder := zapcore.NewJSONEncoder(encoderCfg)
		core := zapcore.NewCore(encoder, writeSyncer, zapcore.DebugLevel)
		lg = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zap.ErrorLevel))
	}
	zap.ReplaceGlobals(lg)
}

func cmdAdd(args *skel.CmdArgs) error {
	zap.S().Debugf("[cmdAdd]args:%#v", *args)
	_, err := config.LoadCfg(args.StdinData)
	if err != nil {
		return err
	}
	storage, err := config.LoadStorage()
	ip := storage.Ipv4Record.Alloc()
	if ip == nil {
		return fmt.Errorf("alloc Ip failed")
	}

	err = storage.Store()
	if err != nil {
		return err
	}
	return nil
}

func cmdDel(args *skel.CmdArgs) error {
	zap.S().Debugf("[cmdDel]args:%#v", *args)
	return nil
}
func cmdCheck(args *skel.CmdArgs) error {
	zap.S().Debugf("[cmdCheck]args:%#v", *args)
	return nil
}
func main() {
	zap.S().Debug("[exec]")
	fullVer := fmt.Sprintf("CNI Plugin %s version %s (%s/%s)", Program, Version, runtime.GOOS, runtime.GOARCH)
	skel.PluginMain(cmdAdd, cmdCheck, cmdDel, version.All, fullVer)
}
