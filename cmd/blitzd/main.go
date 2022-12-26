package main

import (
	"blitz/pkg/config"
	"blitz/pkg/constant"
	"blitz/pkg/devices"
	"blitz/pkg/events"
	"blitz/pkg/host_gw"
	"blitz/pkg/ipnet"
	"blitz/pkg/iptables"
	"blitz/pkg/log"
	nodeMetadata "blitz/pkg/node"
	Reconciler "blitz/pkg/reconciler"
	"blitz/pkg/vxlan"
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Flags struct {
	version     bool
	ipMasq      bool
	clusterCIDR string
	mode        string
}

var opts Flags

func init() {
	flag.BoolVar(&opts.version, "version", false, "")
	flag.BoolVar(&opts.ipMasq, "ip-Masq", false, "")
	flag.StringVar(&opts.clusterCIDR, "ClusterCIDR", "", "")
	flag.StringVar(&opts.mode, "mode", "vxlan", "Mode of Blitz (vxlan/host-gw)")
}
func main() {
	log.InitLog(constant.EnableLog, false, "blitzd")
	log.Log.Debugf("blitzd,start")
	flag.Parse()
	log.Log.Debugf("flags:%#v", opts)
	if opts.version {
		fmt.Printf("Blitzd %s", constant.FullVersion())
		os.Exit(0)
	}

	nodeName := os.Getenv("NODE_NAME")
	kubeCfg, err := rest.InClusterConfig()
	if err != nil {
		log.Log.Fatalf("Get Cluster Failed. May be not in a Cluster")
	}
	log.Log.Debugf("Get Cluster Config Success")
	clientset, err := kubernetes.NewForConfig(kubeCfg)
	if err != nil {
		log.Log.Fatal("Get clientSet Failed", err)
	}
	log.Log.Debugf("Get clientset Success")
	err = Run(nodeName, clientset)
	if err != nil {
		log.Log.Fatal("Run Failed:", err)
	}
}
func CreateStorage(node *corev1.Node) (*config.PlugStorage, error) {
	var IPv4Cfg *config.NetworkCfg
	var IPv6Cfg *config.NetworkCfg
	PodCIDRs, err := nodeMetadata.GetPodCIDRs(node)
	if err != nil {
		return nil, err
	}
	IPv4CIDR, IPv6CIDR := ipnet.SelectIPv4AndIPv6(PodCIDRs)
	clusterCIDR := ipnet.ParseCIDRs(opts.clusterCIDR)
	IPv4ClusterCIDR, IPv6ClusterCIDR := ipnet.SelectIPv4AndIPv6(clusterCIDR)
	if IPv4CIDR != nil && IPv4ClusterCIDR != nil {
		IPv4Cfg = &config.NetworkCfg{PodCIDR: *IPv4CIDR, ClusterCIDR: *IPv4ClusterCIDR}
	}
	if IPv6CIDR != nil && IPv6ClusterCIDR != nil {
		IPv6Cfg = &config.NetworkCfg{PodCIDR: *IPv6CIDR, ClusterCIDR: *IPv6ClusterCIDR}
	}
	return config.CreateStorage(IPv4Cfg, IPv6Cfg)
}
func Run(nodeName string, clientset *kubernetes.Clientset) error {
	node, err := nodeMetadata.GetCurrentNode(clientset, nodeName)
	if err != nil {
		return nil
	}
	storage, err := config.LoadStorage()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			storage, err = CreateStorage(node)
			if err != nil {
				log.Log.Fatal("CreateStorage Failed")
			}
		} else {
			log.Log.Fatal("Load Storage Failed:", err)
		}
	}
	if opts.ipMasq {
		if storage.EnableIPv4() {
			iptables.CreateChain("nat", "BLITZ-POSTRTG", iptables.IPv4)
			err := iptables.ApplyRulesWithCheck(iptables.MasqRules(&storage.Ipv4Cfg.ClusterCIDR, &storage.Ipv4Cfg.PodCIDR, "BLITZ-POSTRTG", iptables.IPv4), iptables.IPv4)
			if err != nil {
				log.Log.Errorf("Apply IPv4 Rules Failed:%v", err)
			}
		}
		if storage.EnableIPv6() {
			iptables.CreateChain("nat", "BLITZ-POSTRTG", iptables.IPv6)
			err := iptables.ApplyRulesWithCheck(iptables.MasqRules(&storage.Ipv4Cfg.ClusterCIDR, &storage.Ipv4Cfg.PodCIDR, "BLITZ-POSTRTG", iptables.IPv6), iptables.IPv6)
			if err != nil {
				log.Log.Errorf("Apply IPv6 Rules Failed:%v", err)
			}
		}
	}
	var handle events.EventHandle
	switch opts.mode {
	case "vxlan":
		vxlanDevice, err := devices.SetupVXLAN(ipnet.FromIPAndMask(storage.Ipv4Cfg.PodCIDR.IP, net.CIDRMask(32, 32)))
		if err != nil {
			log.Log.Error("SetupVXLAN:", err)
			return err
		}
		log.Log.Debug("SetupVXLAN Success")
		err = vxlan.AddVxlanInfo(clientset, node)
		if err != nil {
			log.Log.Error("AddVxlanInfo:", err)
			return err
		}
		log.Log.Debug("AddVXLAN Info Success")
		handle = &vxlan.Handle{
			NodeName: nodeName,
			Vxlan:    vxlanDevice,
		}
	case "host-gw":
		//TODO: ADD IPv6 Support
		defaultLink, err := devices.GetDefaultGateway(devices.IPv4)
		if err != nil {
			log.Log.Debug("No valid route")
			return err
		}
		//TODO: ADD IPv6 Support
		hostIP, err := devices.GetHostIP(devices.IPv4)
		if err != nil {
			return err
		}
		annotations := nodeMetadata.Annotations{PublicIP: *hostIP}
		err = nodeMetadata.AddAnnotationsForNode(clientset, &annotations, node)
		if err != nil {
			return err
		}
		handle = &host_gw.Handle{NodeName: nodeName, Link: *defaultLink}
	default:
		log.Log.Fatal("Invalid mode.")
	}
	ctx := context.TODO()
	reconciler, err := Reconciler.NewReconciler(ctx, clientset, storage, handle)
	if err != nil {
		log.Log.Fatal("Create Reconciler failed:", err)
	}
	reconciler.Run(ctx)
	return nil
}
