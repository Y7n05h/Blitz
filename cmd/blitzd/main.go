package main

import (
	"context"
	"flag"
	"net"
	"os"
	"tiny_cni/internal/Reconciler"
	"tiny_cni/internal/config"
	"tiny_cni/internal/constexpr"
	"tiny_cni/internal/log"
	"tiny_cni/pkg/devices"
	"tiny_cni/pkg/ipnet"

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
	podCIDR, err := Reconciler.GetPodCIDR(currentNode)
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
	ctx, _ := context.WithCancel(context.TODO())
	node, err := Reconciler.GetCurrentNode(clientset, podName)
	if err != nil {
		return nil
	}
	err = Reconciler.AddVxlanInfo(clientset, node)
	if err != nil {
		return err
	}
	podCIDR, err := Reconciler.GetPodCIDR(node)
	if err != nil {
		return err
	}
	vxlan, err := devices.SetupVXLAN(ipnet.FromIPAndMask(storage.NodeCIDR.IP, net.CIDRMask(32, 32)))
	if err != nil {
		return err
	}
	reconciler, err := Reconciler.NewReconciler(ctx, clientset, storage, podName, podCIDR, vxlan)
	if err != nil {
		log.Log.Fatal("Create Reconciler failed:", err)
	}
	go reconciler.Run(ctx)
	return nil
}
