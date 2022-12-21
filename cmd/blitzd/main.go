package main

import (
	"context"
	"flag"
	"net"
	"os"
	"tiny_cni/pkg/Reconciler"
	"tiny_cni/pkg/config"
	"tiny_cni/pkg/constexpr"
	"tiny_cni/pkg/devices"
	"tiny_cni/pkg/events"
	"tiny_cni/pkg/ipnet"
	"tiny_cni/pkg/log"
	node_metadata "tiny_cni/pkg/node"
	"tiny_cni/pkg/vxlan"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Flags struct {
	nwCfgGen    bool
	clusterCIDR string
}

var FlagsValue Flags

func init() {
	flag.BoolVar(&FlagsValue.nwCfgGen, "NetworkCfgGen", false, "Generator Network CniRuntimeCfg")
	flag.StringVar(&FlagsValue.clusterCIDR, "ClusterCIDR", "", "")
}
func main() {
	log.InitLog(constexpr.EnableLog, false, "blitzd")
	log.Log.Debugf("blitzd,start")
	flag.Parse()
	log.Log.Debugf("flags:%#v", FlagsValue)

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
	if FlagsValue.nwCfgGen {
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
	currentNode, err := Reconciler.GetCurrentNode(clientset, nodeName)
	if err != nil {
		log.Log.Fatal("Get Current Node Failed")
	}
	log.Log.Debugf("Get current Node Success")
	podCIDR, err := node_metadata.GetPodCIDR(currentNode)
	if err != nil {
		log.Log.Fatal("Get Node CIDR Failed")
	}
	clusterCIDR, err := ipnet.ParseCIDR(FlagsValue.clusterCIDR)
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
func Run(podName string, clientset *kubernetes.Clientset) error {
	storage, err := config.LoadStorage()
	if err != nil {
		log.Log.Fatal("Load Storage Failed:", err)
	}
	node, err := Reconciler.GetCurrentNode(clientset, podName)
	if err != nil {
		return nil
	}
	podCIDR, err := node.GetPodCIDR(node)
	if err != nil {
		return err
	}
	log.Log.Debug("Get PodCIDR Success")
	var handle events.EventHandle
	{
		vxlanDevice, err := devices.SetupVXLAN(ipnet.FromIPAndMask(storage.NodeCIDR.IP, net.CIDRMask(32, 32)))
		if err != nil {
			log.Log.Error("SetupVXLAN:", err)
			return err
		}
		log.Log.Debug("SetupVXLAN Success")
		err = Reconciler.AddVxlanInfo(clientset, node)
		if err != nil {
			log.Log.Error("AddVxlanInfo:", err)
			return err
		}
		log.Log.Debug("AddVXLAN Info Success")
		handle = &vxlan.Handle{
			NodeName: podName,
			Vxlan:    vxlanDevice,
		}
	}
	ctx := context.TODO()
	reconciler, err := Reconciler.NewReconciler(ctx, clientset, storage, podCIDR, handle)
	if err != nil {
		log.Log.Fatal("Create Reconciler failed:", err)
	}
	reconciler.Run(ctx)
	return nil
}
