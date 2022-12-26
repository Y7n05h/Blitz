package main

import (
	"blitz/pkg/config"
	"blitz/pkg/constant"
	"blitz/pkg/devices"
	"blitz/pkg/ipnet"
	"blitz/pkg/log"
	"errors"
	"fmt"
	"os"
	"runtime"

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
	log.Log.Debug("[Success]LoadCfg")
	if err != nil {
		log.Log.Debug("Err:", err)
		return err
	}
	storage, err := config.LoadStorage()
	log.Log.Debug("[Finished]LoadStorage")
	if err != nil {
		log.Log.Debug("Err:", err)
		return err
	}
	log.Log.Debug("[Success]LoadStorage")
	info := make([]devices.NetworkInfo, 0)
	err = storage.AtomicDo(func() error {
		if storage.EnableIPv4() {
			ip, err := storage.Ipv4Record.Alloc(args.ContainerID)
			if err != nil {
				return err
			}
			info = append(info, devices.NetworkInfo{
				PodIP:       *ip,
				Gateway:     *storage.Ipv4Record.GetGateway(),
				ClusterCIDR: storage.Ipv4Cfg.ClusterCIDR,
			})
		}
		if storage.EnableIPv6() {
			ip, err := storage.Ipv6Record.Alloc(args.ContainerID)
			if err != nil {
				return err
			}
			info = append(info, devices.NetworkInfo{
				PodIP:       *ip,
				Gateway:     *storage.Ipv6Record.GetGateway(),
				ClusterCIDR: storage.Ipv6Cfg.ClusterCIDR,
			})
		}
		return nil
	})
	log.Log.Debug("[Done]Alloc IP")
	if err != nil {
		log.Log.Debug("Err:", err)
		return err
	}
	log.Log.Debug("[Success]Alloc IP")
	gateway := make([]*ipnet.IPNet, 0)
	for _, i := range info {
		gateway = append(gateway, &i.Gateway)
	}
	br, err := devices.GetBridge(gateway)
	if err != nil {
		log.Log.Debugf("Err:%v", err)
		return err
	}
	if br == nil {
		log.Log.Debug("Err:", err)
		return fmt.Errorf("get Bridge failed")
	}
	log.Log.Debug("[Success]get Bridge")
	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		log.Log.Debug("Err:", err)
		return err
	}
	if err := devices.SetupVeth(netns, br, args.IfName, info); err != nil {
		log.Log.Debug("Err:", err)
		return err
	}

	result := types100.Result{
		IPs: make([]*types100.IPConfig, 0),
	}
	for _, i := range info {
		result.IPs = append(result.IPs, &types100.IPConfig{
			Address: *i.PodIP.ToNetIPNet(),
			Gateway: i.Gateway.IP,
		})
	}
	log.Log.Debug("Success")
	return types.PrintResult(&result, cfg.CNIVersion)
}

func cmdDel(args *skel.CmdArgs) error {
	log.Log.Debugf("[cmdDel]args:%#v", *args)
	//cfg, err := config.LoadCfg(args.StdinData)
	//if err != nil {
	//	return err
	//}
	storage, err := config.LoadStorage()
	log.Log.Debug("Load Storage Finished")
	if err != nil {
		return err
	}
	log.Log.Debug("Load Storage Success")
	netns, err := ns.GetNS(args.Netns)
	if err == nil {
		log.Log.Debug("Get Namespace Success,Del Veth")
		err = devices.DelVeth(netns, args.IfName)
		if err != nil {
			log.Log.Debug("Del Veth failed: ", err)
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	log.Log.Debug("Done Release IP")
	err = storage.AtomicDo(func() error {
		if storage.EnableIPv4() {
			err := storage.Ipv4Record.Release(args.ContainerID)
			if err != nil {
				return err
			}
		}
		if storage.EnableIPv6() {
			err := storage.Ipv6Record.Release(args.ContainerID)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	log.Log.Debug("[cmdDel]Success")
	return nil
}
func cmdCheck(args *skel.CmdArgs) error {
	log.Log.Debugf("[cmdCheck]args:%#v", *args)
	storage, err := config.LoadStorage()
	log.Log.Debug("Load Storage Finished")
	if err != nil {
		return err
	}
	log.Log.Debug("Load Storage Success")
	ips := make([]*ipnet.IPNet, 0)
	err = storage.AtomicDo(func() error {
		if storage.EnableIPv4() {
			ipNet, ok := storage.Ipv4Record.GetIPByID(args.ContainerID)
			if !ok {
				//TODO
				log.Log.Debug("Get IP by ID failed")
				//return fmt.Errorf("can not found IP")
				return nil
			}
			ips = append(ips, ipNet)
		}
		if storage.EnableIPv6() {
			ipNet, ok := storage.Ipv6Record.GetIPByID(args.ContainerID)
			if !ok {
				//TODO
				log.Log.Debug("Get IP by ID failed")
				//return fmt.Errorf("can not found IP")
				return nil
			}
			ips = append(ips, ipNet)
		}
		return nil
	})
	log.Log.Debug("Get NS")
	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		return err
	}
	err = netns.Do(func(_ ns.NetNS) error {
		veth, err := netlink.LinkByName(args.IfName)
		if err != nil {
			return err
		}
		if !devices.CheckLinkContainIPNNet(ips, veth) {
			return fmt.Errorf("%s does not have %v", veth.Attrs().Name, ips)
		}
		return nil
	})
	log.Log.Debug("[Check]Success")
	return err
}
func main() {
	log.InitLog(constant.EnableLog, false, "blitz")
	log.Log.Debug("[exec]")
	fullVer := fmt.Sprintf("Blitz %s\tRuntime:%s %s", constant.FullVersion(), runtime.GOOS, runtime.GOARCH)
	skel.PluginMain(cmdAdd, cmdCheck, cmdDel, version.All, fullVer)
}
