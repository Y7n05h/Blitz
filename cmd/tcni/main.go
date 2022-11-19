package main

import (
	"fmt"
	"net"
	"runtime"
	"tiny_cni/internal/config"
	"tiny_cni/internal/constexpr"
	"tiny_cni/internal/log"
	"tiny_cni/pkg/bridge"

	"github.com/vishvananda/netlink"

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
	if err != nil {
		return fmt.Errorf("load storage failed")
	}
	var ip *net.IPNet
	err = storage.AtomicDo(func() error {
		var err error
		ip, err = storage.Ipv4Record.Alloc(args.ContainerID)
		return err
	})
	if err != nil {
		return err
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
	//cfg, err := config.LoadCfg(args.StdinData)
	//if err != nil {
	//	return err
	//}
	storage, err := config.LoadStorage()
	if err != nil {
		return fmt.Errorf("load storage failed")
	}
	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		return fmt.Errorf("get netns failed")
	}
	err = bridge.DelVeth(netns, args.IfName)
	if err != nil {
		return err
	}
	err = storage.AtomicDo(func() error {
		return storage.Ipv4Record.Release(args.ContainerID)
	})
	if err != nil {
		return err
	}
	return nil
}
func cmdCheck(args *skel.CmdArgs) error {
	log.Log.Debugf("[cmdCheck]args:%#v", *args)
	storage, err := config.LoadStorage()
	if err != nil {
		return fmt.Errorf("load storage failed")
	}
	defer storage.Store()
	ip, ok := storage.Ipv4Record.GetIPByID(args.ContainerID)
	if !ok {
		return fmt.Errorf("can not found IP")
	}
	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		return err
	}
	ipNet := &net.IPNet{
		IP:   ip,
		Mask: storage.Ipv4Record.Mask(),
	}
	return netns.Do(func(_ ns.NetNS) error {
		veth, err := netlink.LinkByName(args.IfName)
		if err != nil {
			return err
		}
		if !bridge.CheckBridge(ipNet, veth) {
			return fmt.Errorf("%s does not have %s", veth.Attrs().Name, ipNet.String())
		}
		return nil
	})
}
func main() {
	log.Log.Debug("[exec]")
	fullVer := fmt.Sprintf("CNI Plugin %s version %s (%s/%s)", constexpr.Program, constexpr.Version, runtime.GOOS, runtime.GOARCH)
	skel.PluginMain(cmdAdd, cmdCheck, cmdDel, version.All, fullVer)
}
