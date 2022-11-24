package main

import (
	"flag"
	"os"
	"time"
	"tiny_cni/internal/Reconciler"
	"tiny_cni/internal/config"
	"tiny_cni/internal/log"

	"github.com/containernetworking/cni/pkg/types"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Flags struct {
	nwCfgGen    bool
	clusterCIDR string
}

var FlagsValue Flags

func init() {
	flag.BoolVar(&FlagsValue.nwCfgGen, "NetworkCfgGen", false, "Generator Network Cfg")
	flag.StringVar(&FlagsValue.clusterCIDR, "ClusterCIDR", "", "")
}
func main() {
	log.Log.Debugf("tcnid,start")
	flag.Parse()
	podName := os.Getenv("POD_NAME")
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
	currentNode, err := Reconciler.GetCurrentNode(clientset, podName)
	if err != nil {
		log.Log.Fatal("Get Current Node Failed")
	}
	log.Log.Debugf("Get current Node Success")
	podCIDR, err := Reconciler.GetPodCIDR(currentNode)
	if err != nil {
		log.Log.Fatal("Get Node CIDR Failed")
	}
	clusterCIDR, err := types.ParseCIDR(FlagsValue.clusterCIDR)
	if err != nil {
		log.Log.Fatal("Parse clusterCIDR Error:", err)
	}
	log.Log.Debugf("Parse CIDR Success! PodCIDR:%s ClusterCIDR:%s", podCIDR.String(), clusterCIDR.String())
	if FlagsValue.nwCfgGen {
		cfg := config.PlugNetworkCfg{
			ClusterCIDR: *(*types.IPNet)(clusterCIDR),
			NodeCIDR:    *(*types.IPNet)(podCIDR),
		}
		err = cfg.StoreNetworkCfg()
		if err != nil {
			log.Log.Fatal("Generator Network Cfg Failed")
		}
	}
	if 0 == 1 {
	_:
		Run(podName, clientset)
	}
	time.Sleep(time.Hour * 24)
}
func Run(podName string, clientset *kubernetes.Clientset) error {
	storage, err := config.LoadStorage()
	if err != nil {
		log.Log.Fatal("Load Storage Failed:", err)
	}
	reconciler, err := Reconciler.NewReconciler(clientset, storage, podName)
	if err != nil {
		log.Log.Fatal("Create Reconciler failed:", err)
	}
	reconciler.ReconcilerLoop()
	return nil
}
