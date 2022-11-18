package main

import (
	"tiny_cni/internal/log"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	kubeCfg, err := rest.InClusterConfig()
	if err != nil {
		log.Log.Fatalf("Get Cluster Failed. May be not in a Cluster")
	}
	clientSet, err = kubernetes.NewForConfig(kubeCfg)
	if err != nil {
		log.Log.Fatalf("Get clientSet Failed")
	}
}
