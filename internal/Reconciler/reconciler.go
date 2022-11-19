package Reconciler

import (
	"context"
	"fmt"
	"net"
	"tiny_cni/internal/config"
	"tiny_cni/internal/log"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
)

type Reconciler struct {
	Clientset  *kubernetes.Clientset
	CniStorage *config.PlugStorage
	PodName    string
	HostIP     net.IPNet
}

func NewReconciler(clientset *kubernetes.Clientset, cniStorage *config.PlugStorage, podName string) (*Reconciler, error) {
	node, err := clientset.CoreV1().Nodes().Get(context.TODO(), podName, v1.GetOptions{})
	if err != nil {
		log.Log.Error("Get Node Info Failed:", err)
		return nil, err
	}
	if node == nil || len(node.Spec.PodCIDR) == 0 {
		return nil, fmt.Errorf("get PodCIDR Failed")
	}
	_, ip, err := net.ParseCIDR(node.Spec.PodCIDR)
	if err != nil {
		log.Log.Fatal("Parse CIDR Failed:", err)
		return nil, err
	}
	reconciler := &Reconciler{Clientset: clientset, CniStorage: cniStorage, PodName: podName, HostIP: *ip}
	return reconciler, nil
}
func (r *Reconciler) ReconcilerLoop() error {

}
