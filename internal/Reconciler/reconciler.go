package Reconciler

import (
	"context"
	"fmt"
	"net"
	"syscall"
	"time"
	"tiny_cni/internal/config"
	"tiny_cni/internal/constexpr"
	"tiny_cni/internal/log"
	node_metadata "tiny_cni/internal/node"
	"tiny_cni/pkg/devices"
	"tiny_cni/pkg/hardware"
	"tiny_cni/pkg/ipnet"

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
	Clientset  *kubernetes.Clientset
	CniStorage *config.PlugStorage
	PodName    string
	vxlan      netlink.Link
	PodCIDR    ipnet.IPNet
	Node       listers.NodeLister
	Controller cache.Controller
}

func GetPodCIDR(node *corev1.Node) (*ipnet.IPNet, error) {
	if len(node.Spec.PodCIDR) == 0 {
		return nil, fmt.Errorf("get %s PodCIDR Failed", node.Name)
	}
	_, ip, err := net.ParseCIDR(node.Spec.PodCIDR)
	return ipnet.FromNetIPNet(ip), err
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
func (r *Reconciler) AddHandle(obj any) {
	n := obj.(*corev1.Node)
	if n.Name == r.PodName {
		return
	}
	annotations := node_metadata.GetAnnotations(n)
	if annotations == nil {
		return
	}
	cidr, err := GetPodCIDR(n)
	if err != nil {
		log.Log.Warn("Get Cidr From Node Failed", err)
		return
	}
	ifIdx := r.vxlan.Attrs().Index
	//添加路由表中
	route := netlink.Route{
		LinkIndex: ifIdx,
		Scope:     netlink.SCOPE_UNIVERSE,
		Dst:       cidr.ToNetIPNet(),
		Gw:        cidr.IP,
		Flags:     syscall.RTNH_F_ONLINK,
	}
	err = netlink.RouteAdd(&route)
	if err != nil {
		log.Log.Error("Add Route Failed")
		return
	}
	// 添加 Arp 表中条目
	err = devices.AddARP(ifIdx, cidr.IP, annotations.VxlanMacAddr)
	if err != nil {
		log.Log.Error("Add ARP Failed: ", err)
	}
	//删除 Fdb表中条目
	err = devices.AddFDB(ifIdx, annotations.PublicIP.IP, annotations.VxlanMacAddr)
	if err != nil {
		log.Log.Error("Add Fdb Failed: ", err)
	}

}
func (r *Reconciler) UpdateHandle(oldObj, newObj any) {
	oldNode := oldObj.(*corev1.Node)
	newNode := newObj.(*corev1.Node)
	if oldNode.Name == r.PodName || newNode.Name == r.PodName {
		return
	}
	oldAnnotations := node_metadata.GetAnnotations(oldNode)
	newAnnotations := node_metadata.GetAnnotations(newNode)
	if oldAnnotations != nil && newAnnotations != nil && oldAnnotations.VxlanMacAddr.Equal(&newAnnotations.VxlanMacAddr) {
		return
	}
	if oldAnnotations != nil {
		r.DeleteHandle(newObj)
	}
	if newAnnotations != nil {
		r.AddHandle(newObj)
	}
}

func (r *Reconciler) DeleteHandle(obj any) {
	n := obj.(*corev1.Node)
	if n.Name == r.PodName {
		return
	}
	annotations := node_metadata.GetAnnotations(n)
	if annotations == nil {
		return
	}
	cidr, err := GetPodCIDR(n)
	if err != nil {
		log.Log.Warn("Get Cidr From Node Failed", err)
		return
	}

	//删除路由表中条目
	route := devices.GetRouteByDist(r.vxlan.Attrs().Index, *cidr)
	if route != nil {
		err := netlink.RouteDel(route)
		if err != nil {
			log.Log.Error("Del Route Failed:", err)
		}
	}
	// 删除Arp表中条目
	neigh := devices.GetNeighByIP(r.vxlan.Attrs().Index, cidr.IP)
	if neigh != nil {
		err := netlink.NeighDel(neigh)
		if err != nil {
			log.Log.Error("Delete Neigh Failed")
		}
	}
	//删除 Fdb表中条目
	err = devices.DelFDB(r.vxlan.Attrs().Index, annotations.PublicIP.IP, annotations.VxlanMacAddr)
	if err != nil {
		log.Log.Error("Del ARP Failed: ", err)
	}

}
func NewReconciler(ctx context.Context, clientset *kubernetes.Clientset, cniStorage *config.PlugStorage, podName string, podCIDR *ipnet.IPNet, vxlan netlink.Link) (*Reconciler, error) {
	listWatch := cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return clientset.CoreV1().Nodes().List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return clientset.CoreV1().Nodes().Watch(ctx, options)
		},
		DisableChunking: false,
	}
	reconciler := &Reconciler{Clientset: clientset, CniStorage: cniStorage, PodName: podName, PodCIDR: *podCIDR, vxlan: vxlan}
	handles := cache.ResourceEventHandlerFuncs{
		AddFunc:    reconciler.AddHandle,
		UpdateFunc: reconciler.UpdateHandle,
		DeleteFunc: reconciler.DeleteHandle,
	}
	indexer, controller := cache.NewIndexerInformer(&listWatch, &corev1.Node{}, SyncTime, handles, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	reconciler.Node = listers.NewNodeLister(indexer)
	reconciler.Controller = controller
	return reconciler, nil
}
func (r *Reconciler) Run(ctx context.Context) {
	log.Log.Infof("Run Reconciler")
	r.Controller.Run(ctx.Done())
}
