package main

import (
	"os"
	"tiny_cni/internal/Reconciler"
	"tiny_cni/internal/config"
	"tiny_cni/internal/log"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	kubeCfg, err := rest.InClusterConfig()
	if err != nil {
		log.Log.Fatalf("Get Cluster Failed. May be not in a Cluster")
	}
	clientSet, err := kubernetes.NewForConfig(kubeCfg)
	if err != nil {
		log.Log.Fatal("Get clientSet Failed", err)
	}
	storage, err := config.LoadStorage()
	if err != nil {
		log.Log.Fatal("Load Storage Failed:", err)
	}
	podName := os.Getenv("POD_NAME")
	reconciler, err := Reconciler.NewReconciler(clientSet, storage, podName)
	if err != nil {
		log.Log.Fatal("Create Reconciler failed:", err)
	}
	reconciler.ReconcilerLoop()
}
