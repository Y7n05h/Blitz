package node

import (
	"blitz/pkg/hardware"
	"blitz/pkg/ipnet"
	"blitz/pkg/log"
	"context"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/client-go/kubernetes"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

const (
	AnnotationsPath = "blitz.y7n05h.dev"
)

type Annotations struct {
	IPv4VxlanMacAddr hardware.Address `json:"IPv4VxlanMac,omitempty"`
	IPv6VxlanMacAddr hardware.Address `json:"IPv6VxlanMac,omitempty"`
	PublicIPv4       *ipnet.IPNet     `json:"PublicIPv4,omitempty"`
	PublicIPv6       *ipnet.IPNet     `json:"PublicIPv6,omitempty"`
}

func (a *Annotations) Equal(annotations *Annotations) bool {
	return a == annotations ||
		(a != nil && annotations != nil &&
			a.IPv4VxlanMacAddr.Equal(&annotations.IPv4VxlanMacAddr) &&
			a.IPv6VxlanMacAddr.Equal(&annotations.IPv6VxlanMacAddr) &&
			a.PublicIPv4.Equal(annotations.PublicIPv4) &&
			a.PublicIPv6.Equal(annotations.PublicIPv6))
}
func AddAnnotationsForNode(clientset *kubernetes.Clientset, annotations *Annotations, node *corev1.Node) error {
	data, err := json.Marshal(annotations)
	if err != nil {
		return err
	}
	n := node.DeepCopy()
	n.Annotations[AnnotationsPath] = string(data)
	oldData, err := json.Marshal(node)
	if err != nil {
		return err
	}
	newData, err := json.Marshal(n)
	if err != nil {
		return err
	}
	patch, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, corev1.Node{})
	if err != nil {
		return err
	}
	_, err = clientset.CoreV1().Nodes().Patch(context.TODO(), n.Name, types.StrategicMergePatchType, patch, metav1.PatchOptions{}, "status")
	return err
}
func GetAnnotations(node *corev1.Node) *Annotations {
	annotations := Annotations{}
	annotationsData, ok := node.Annotations[AnnotationsPath]
	if !ok {
		return nil
	}
	err := json.Unmarshal([]byte(annotationsData), &annotations)
	if err != nil {
		return nil
	}
	return &annotations
}

func GetPodCIDRs(node *corev1.Node) ([]*ipnet.IPNet, error) {
	size := len(node.Spec.PodCIDRs)
	if size <= 0 {
		return nil, fmt.Errorf("get %s PodCIDRs Failed:PodCIDRs is empty", node.Name)
	}
	if size > 2 {
		return nil, fmt.Errorf("get %s PodCIDRs Failed:PodCIDRs more than 2", node.Name)
	}
	result := make([]*ipnet.IPNet, 0)
	for _, cidrString := range node.Spec.PodCIDRs {
		cidr, err := ipnet.ParseCIDR(cidrString)
		if err != nil {
			log.Log.Warnf("Parse PodCIDR (%s) Error:%v.Error Ignored.", cidrString, err)
			continue
		}
		result = append(result, cidr)
	}
	if len(result) < 1 {
		return nil, fmt.Errorf("no valid PodCIDR")
	}
	return result, nil
}
func GetCurrentNode(clientset *kubernetes.Clientset, nodeName string) (*corev1.Node, error) {
	node, err := clientset.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		log.Log.Error("Get Node Info Failed:", err)
		return nil, err
	}
	if node == nil {
		return nil, fmt.Errorf("invaild node")
	}
	return node, nil
}
