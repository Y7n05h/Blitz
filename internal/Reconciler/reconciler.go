package Reconciler

import (
	"context"
	"fmt"
	"net"
	"tiny_cni/internal/config"
	"tiny_cni/internal/log"
	"tiny_cni/pkg/bridge"

	"github.com/vishvananda/netlink"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
)

type Reconciler struct {
	Clientset  *kubernetes.Clientset
	CniStorage *config.PlugStorage
	PodName    string
	HostCidr   net.IPNet
	Link       netlink.Link
}

func GetPodCIDR(node *corev1.Node) (*net.IPNet, error) {
	if len(node.Spec.PodCIDR) == 0 {
		return nil, fmt.Errorf("get PodCIDR Failed")
	}
	_, ip, err := net.ParseCIDR(node.Spec.PodCIDR)
	return ip, err
}
func NewReconciler(clientset *kubernetes.Clientset, cniStorage *config.PlugStorage, podName string) (*Reconciler, error) {
	node, err := clientset.CoreV1().Nodes().Get(context.TODO(), podName, v1.GetOptions{})
	if err != nil {
		log.Log.Error("Get Node Info Failed:", err)
		return nil, err
	}
	if node == nil {
		return nil, fmt.Errorf("invaild node")
	}
	cidr, err := GetPodCIDR(node)
	if err != nil {
		return nil, err
	}
	link, err := bridge.LinkByIP(cidr)
	if err != nil {
		return nil, err
	}
	reconciler := &Reconciler{Clientset: clientset, CniStorage: cniStorage, PodName: podName, HostCidr: *cidr, Link: link}
	return reconciler, nil
}
func (r *Reconciler) ReconcilerLoop() error {

}
