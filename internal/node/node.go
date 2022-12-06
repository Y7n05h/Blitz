package node

import (
	"context"
	"encoding/json"
	"tiny_cni/pkg/hardware"
	"tiny_cni/pkg/ipnet"

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
	VxlanMacAddr hardware.Address
	PublicIP     ipnet.IPNet
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
