package main

import (
	"fmt"
	"runtime"
	"tiny_cni/internal/config"
	"tiny_cni/internal/constexpr"
	"tiny_cni/internal/log"
	"tiny_cni/pkg/bridge"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	types100 "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/containernetworking/plugins/pkg/ns"
)

func cmdAdd(args *skel.CmdArgs) error {
	log.Log.Debugf("[cmdAdd]args:%#v", *args)
	cfg, err := config.LoadCfg(args.StdinData)
	if err != nil {
		return err
	}
	storage, err := config.LoadStorage()
	ip, err := storage.Ipv4Record.Alloc()
	if err != nil {
		return fmt.Errorf("alloc Ip failed")
	}
	gateway := storage.Ipv4Record.Gateway()
	br, err := bridge.GetBridge(gateway)
	if err != nil {
		return err
	}
	if br == nil {
		return fmt.Errorf("get Bridge failed")
	}
	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		return err
	}

	if err := bridge.SetupVeth(netns, br, args.IfName, ip, gateway.IP); err != nil {
		return err
	}

	err = storage.Store()
	if err != nil {
		return err
	}
	result := types100.Result{
		IPs: []*types100.IPConfig{
			{
				Address: *ip,
				Gateway: gateway.IP,
			},
		},
	}
	return types.PrintResult(&result, cfg.CNIVersion)
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
