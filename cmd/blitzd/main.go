package main

import (
	"blitz/pkg/config"
	"blitz/pkg/constant"
	"blitz/pkg/devices"
	"blitz/pkg/events"
	"blitz/pkg/host_gw"
	"blitz/pkg/ipnet"
	"blitz/pkg/log"
	nodeMetadata "blitz/pkg/node"
	Reconciler "blitz/pkg/reconciler"
	"blitz/pkg/vxlan"
	"context"
	"flag"
	"fmt"
	"net"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Flags struct {
	nwCfgGen    bool
	version     bool
	clusterCIDR string
	mode        string
}

var opts Flags

func init() {
	flag.BoolVar(&opts.nwCfgGen, "NetworkCfgGen", false, "Generator Network CniRuntimeCfg")
	flag.BoolVar(&opts.version, "version", false, "")
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
	if opts.nwCfgGen {
		EnvironmentInit(nodeName, clientset)
	} else {
		err := Run(nodeName, clientset)
		if err != nil {
			log.Log.Fatal("Run Failed:", err)
		}
	}
}

// EnvironmentInit will never return!
func EnvironmentInit(nodeName string, clientset *kubernetes.Clientset) {
	currentNode, err := nodeMetadata.GetCurrentNode(clientset, nodeName)
	if err != nil {
		log.Log.Fatal("Get Current Node Failed")
	}
	log.Log.Debugf("Get current Node Success")
	podCIDR, err := nodeMetadata.GetPodCIDR(currentNode)
	if err != nil {
		log.Log.Fatal("Get Node CIDR Failed")
	}
	clusterCIDR, err := ipnet.ParseCIDR(opts.clusterCIDR)
	if err != nil {
		log.Log.Fatal("Parse clusterCIDR Error:", err)
	}
	log.Log.Debugf("Parse CIDR Success! PodCIDR:%s ClusterCIDR:%s", podCIDR.String(), clusterCIDR.String())
	cfg := config.NetworkCfg{
		ClusterCIDR: *clusterCIDR,
		NodeCIDR:    *podCIDR,
	}
	_, err = config.CreateStorage(cfg)
	if err != nil {
		log.Log.Fatal("CreateStorage Failed")
	}
	log.Log.Infof("[blitzd]Run Success")
	os.Exit(0)
}
func Run(nodeName string, clientset *kubernetes.Clientset) error {
	storage, err := config.LoadStorage()
	if err != nil {
		log.Log.Fatal("Load Storage Failed:", err)
	}
	node, err := nodeMetadata.GetCurrentNode(clientset, nodeName)
	if err != nil {
		return nil
	}
	podCIDR, err := nodeMetadata.GetPodCIDR(node)
	if err != nil {
		return err
	}
	log.Log.Debug("Get PodCIDR Success")
	var handle events.EventHandle
	switch opts.mode {
	case "vxlan":
		vxlanDevice, err := devices.SetupVXLAN(ipnet.FromIPAndMask(storage.NodeCIDR.IP, net.CIDRMask(32, 32)))
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
		defaultLink, err := devices.GetDefaultGateway()
		if err != nil {
			log.Log.Debug("No valid route")
			return err
		}
		handle = &host_gw.Handle{NodeName: nodeName, Link: *defaultLink}
	default:
		log.Log.Fatal("Invalid mode.")
	}
	ctx := context.TODO()
	reconciler, err := Reconciler.NewReconciler(ctx, clientset, storage, podCIDR, handle)
	if err != nil {
		log.Log.Fatal("Create Reconciler failed:", err)
	}
	reconciler.Run(ctx)
	return nil
}
