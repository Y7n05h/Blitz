package vxlan

import (
	"blitz/pkg/constant"
	"blitz/pkg/devices"
	"blitz/pkg/events"
	"blitz/pkg/hardware"
	"blitz/pkg/log"
	nodeMetadata "blitz/pkg/node"
	"syscall"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/vishvananda/netlink"
)

var _ events.EventHandle = (*Handle)(nil)

type Handle struct {
	NodeName string
	Vxlan    netlink.Link
}

func (v *Handle) AddHandle(event *events.Event) {
	if event.Name == v.NodeName {
		return
	}
	ifIdx := v.Vxlan.Attrs().Index
	//添加路由表中
	route := netlink.Route{
		LinkIndex: ifIdx,
		Scope:     netlink.SCOPE_UNIVERSE,
		Dst:       event.PodCIDR.ToNetIPNet(),
		Gw:        event.PodCIDR.IP,
		Flags:     syscall.RTNH_F_ONLINK,
	}
	err := netlink.RouteAdd(&route)
	if err != nil {
		log.Log.Error("Add Route Failed")
		return
	}
	// 添加 Arp 表中条目
	err = devices.AddARP(ifIdx, event.PodCIDR.IP, event.Attr.VxlanMacAddr)
	if err != nil {
		log.Log.Error("Add ARP Failed: ", err)
	}
	//添加 Fdb表中条目
	log.Log.Debugf("[reconciler]DEBUG Node:%s annotations:%v %#v", event.Name, event.Attr, event.Attr)
	err = devices.AddFDB(ifIdx, event.Attr.PublicIPv4.IP, event.Attr.VxlanMacAddr)
	if err != nil {
		log.Log.Error("Add Fdb Failed: ", err)
	}
}
func (v *Handle) DelHandle(event *events.Event) {
	if event.Name == v.NodeName {
		return
	}
	route := devices.GetRouteByDist(v.Vxlan.Attrs().Index, event.PodCIDR)
	if route != nil {
		err := netlink.RouteDel(route)
		if err != nil {
			log.Log.Error("Del Route Failed:", err)
		}
	}
	// 删除Arp表中条目
	neigh := devices.GetNeighByIP(v.Vxlan.Attrs().Index, event.PodCIDR.IP)
	if neigh != nil {
		err := netlink.NeighDel(neigh)
		if err != nil {
			log.Log.Error("Delete Neigh Failed")
		}
	}
	//删除 Fdb表中条目
	err := devices.DelFDB(v.Vxlan.Attrs().Index, event.Attr.PublicIPv4.IP, event.Attr.VxlanMacAddr)
	if err != nil {
		log.Log.Error("Del ARP Failed: ", err)
	}
}

func AddVxlanInfo(clientset *kubernetes.Clientset, n *corev1.Node) error {
	link, err := netlink.LinkByName(constant.VXLANName)
	if err != nil {
		return err
	}
	hardwareAddr := hardware.FromNetHardware(&link.Attrs().HardwareAddr)
	oldAnnotations := nodeMetadata.GetAnnotations(n)
	if oldAnnotations != nil && oldAnnotations.VxlanMacAddr.Equal(hardwareAddr) {
		return nil
	}
	//TODO: ADD IPv6 Support
	PublicIP, err := devices.GetHostIP(devices.IPv4)
	if err != nil {
		return err
	}
	annotations := nodeMetadata.Annotations{VxlanMacAddr: *hardwareAddr, PublicIPv4: PublicIP}
	err = nodeMetadata.AddAnnotationsForNode(clientset, &annotations, n)
	if err != nil {
		return err
	}
	return nil
}
