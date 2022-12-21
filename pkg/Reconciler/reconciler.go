package Reconciler

import (
	"context"
	"fmt"
	"time"
	"tiny_cni/pkg/config"
	"tiny_cni/pkg/constexpr"
	"tiny_cni/pkg/devices"
	"tiny_cni/pkg/events"
	"tiny_cni/pkg/hardware"
	"tiny_cni/pkg/ipnet"
	"tiny_cni/pkg/log"
	node_metadata "tiny_cni/pkg/node"

	"github.com/vishvananda/netlink"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	"k8s.io/client-go/kubernetes"
)

const (
	SyncTime = time.Minute
)

type Reconciler struct {
	Clientset   *kubernetes.Clientset
	CniStorage  *config.PlugStorage
	PodCIDR     ipnet.IPNet
	Node        listers.NodeLister
	Controller  cache.Controller
	event       chan *events.Event
	eventHandle events.EventHandle
}

func AddVxlanInfo(clientset *kubernetes.Clientset, n *corev1.Node) error {
	link, err := netlink.LinkByName(constexpr.VXLANName)
	if err != nil {
		return err
	}
	hardwareAddr := hardware.FromNetHardware(&link.Attrs().HardwareAddr)
	oldAnnotations := node_metadata.GetAnnotations(n)
	if oldAnnotations != nil && oldAnnotations.VxlanMacAddr.Equal(hardwareAddr) {
		return nil
	}
	PublicIP, err := devices.GetHostIP()
	if err != nil {
		return err
	}
	annotations := node_metadata.Annotations{VxlanMacAddr: *hardwareAddr, PublicIP: *PublicIP}
	err = node_metadata.AddAnnotationsForNode(clientset, &annotations, n)
	if err != nil {
		return err
	}
	return nil
}
func GetCurrentNode(clientset *kubernetes.Clientset, podName string) (*corev1.Node, error) {
	node, err := clientset.CoreV1().Nodes().Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		log.Log.Error("Get Node Info Failed:", err)
		return nil, err
	}
	if node == nil {
		return nil, fmt.Errorf("invaild node")
	}
	return node, nil
}

func NewReconciler(ctx context.Context, clientset *kubernetes.Clientset, cniStorage *config.PlugStorage, podCIDR *ipnet.IPNet, handle events.EventHandle) (*Reconciler, error) {
	log.Log.Debug("New Reconciler")
	listWatch := cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return clientset.CoreV1().Nodes().List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return clientset.CoreV1().Nodes().Watch(ctx, options)
		},
		DisableChunking: false,
	}
	eventCh := make(chan *events.Event, 16)
	reconciler := &Reconciler{Clientset: clientset, CniStorage: cniStorage, PodCIDR: *podCIDR, event: eventCh, eventHandle: handle}
	handles := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			node := obj.(*corev1.Node)
			event := events.FromNode(node, events.Add)
			if event == nil {
				return
			}
			eventCh <- event
		},
		UpdateFunc: func(oldObj, newObj any) {
			oldNode := oldObj.(*corev1.Node)
			// Note: Use events.Add is not a bug. We need compare two events without type.
			oldNodeEvent := events.FromNode(oldNode, events.Add)
			newNode := newObj.(*corev1.Node)
			newNodeEvent := events.FromNode(newNode, events.Add)
			if oldNodeEvent.Equal(newNodeEvent) {
				return
			}
			if oldNodeEvent != nil {
				oldNodeEvent.Type = events.Del
				eventCh <- oldNodeEvent
			}
			if newNodeEvent != nil {
				eventCh <- newNodeEvent
			}
		},
		DeleteFunc: func(obj any) {
			node := obj.(*corev1.Node)
			event := events.FromNode(node, events.Del)
			if event == nil {
				return
			}
			eventCh <- event
		},
	}
	log.Log.Debug("New IndexerInformer")
	indexer, controller := cache.NewIndexerInformer(&listWatch, &corev1.Node{}, SyncTime, handles, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	reconciler.Node = listers.NewNodeLister(indexer)
	reconciler.Controller = controller
	log.Log.Debug("New Reconciler Success")
	return reconciler, nil
}
func (r *Reconciler) Run(ctx context.Context) {
	log.Log.Infof("Run Reconciler")
	go r.Controller.Run(ctx.Done())
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-r.event:
			switch event.Type {
			case events.Add:
				r.eventHandle.AddHandle(event)
			case events.Del:
				r.eventHandle.DelHandle(event)
			}
		}
	}
}
