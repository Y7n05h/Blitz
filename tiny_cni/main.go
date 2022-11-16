package main

import (
	"fmt"
	"runtime"
	"tiny_cni/internal/config"
	"tiny_cni/internal/constexpr"
	"tiny_cni/internal/log"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/version"
)

func cmdAdd(args *skel.CmdArgs) error {
	log.Log.Debugf("[cmdAdd]args:%#v", *args)
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
	log.Log.Debugf("[cmdDel]args:%#v", *args)
	return nil
}
func cmdCheck(args *skel.CmdArgs) error {
	log.Log.Debugf("[cmdCheck]args:%#v", *args)
	return nil
}
func main() {
	log.Log.Debug("[exec]")
	fullVer := fmt.Sprintf("CNI Plugin %s version %s (%s/%s)", constexpr.Program, constexpr.Version, runtime.GOOS, runtime.GOARCH)
	skel.PluginMain(cmdAdd, cmdCheck, cmdDel, version.All, fullVer)
}
