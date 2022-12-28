package vxlan

import (
	"blitz/pkg/constant"
	"blitz/pkg/devices"
	"blitz/pkg/events"
	"blitz/pkg/hardware"
	"blitz/pkg/ipnet"
	"blitz/pkg/log"
	nodeMetadata "blitz/pkg/node"
	"syscall"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/vishvananda/netlink"
)

var _ events.EventHandle = (*Handle)(nil)

type Handle struct {
	NodeName  string
	Ipv4Vxlan netlink.Link
	Ipv6Vxlan netlink.Link
}

func addHandle(ifIdx int, podCIDR, public *ipnet.IPNet, mac hardware.Address) {
	//添加路由表中
	route := netlink.Route{
		LinkIndex: ifIdx,
		Scope:     netlink.SCOPE_UNIVERSE,
		Dst:       podCIDR.ToNetIPNet(),
		Gw:        podCIDR.IP,
		Flags:     syscall.RTNH_F_ONLINK,
	}
	err := netlink.RouteAdd(&route)
	if err != nil {
		log.Log.Error("Add Route Failed")
		return
	}
	// 添加 Arp 表中条目
	err = devices.AddARP(ifIdx, podCIDR.IP, mac)
	if err != nil {
		log.Log.Error("Add ARP Failed: ", err)
	}
	//添加 Fdb表中条目
	err = devices.AddFDB(ifIdx, public.IP, mac)
	if err != nil {
		log.Log.Error("Add Fdb Failed: ", err)
	}
}
func (v *Handle) AddHandle(event *events.Event) {
	if event.Name == v.NodeName {
		return
	}
	if v.Ipv4Vxlan != nil {
		if event.IPv4PodCIDR == nil || event.Attr.IPv4VxlanMacAddr == nil || event.Attr.PublicIPv4 == nil {
			log.Log.Warnf("Invaild event")
			return
		}
		addHandle(v.Ipv4Vxlan.Attrs().Index, event.IPv4PodCIDR, event.Attr.PublicIPv4, event.Attr.IPv4VxlanMacAddr)
	}
	if v.Ipv6Vxlan != nil {
		if event.IPv6PodCIDR == nil || event.Attr.IPv6VxlanMacAddr == nil || event.Attr.PublicIPv6 == nil {
			log.Log.Warnf("Invaild event")
			return
		}
		addHandle(v.Ipv6Vxlan.Attrs().Index, event.IPv6PodCIDR, event.Attr.PublicIPv6, event.Attr.IPv6VxlanMacAddr)
	}
}
func delHandle(ifIdx int, podCIDR, public *ipnet.IPNet, mac hardware.Address) {
	route := devices.GetRouteByDist(ifIdx, *podCIDR)
	if route != nil {
		err := netlink.RouteDel(route)
		if err != nil {
			log.Log.Error("Del Route Failed:", err)
		}
	}
	// 删除Arp表中条目
	neigh := devices.GetNeighByIP(ifIdx, podCIDR.IP)
	if neigh != nil {
		err := netlink.NeighDel(neigh)
		if err != nil {
			log.Log.Error("Delete Neigh Failed")
		}
	}
	//删除 Fdb表中条目
	err := devices.DelFDB(ifIdx, public.IP, mac)
	if err != nil {
		log.Log.Error("Del ARP Failed: ", err)
	}
}

func (v *Handle) DelHandle(event *events.Event) {
	if event.Name == v.NodeName {
		return
	}
	if v.Ipv4Vxlan != nil {
		if event.IPv4PodCIDR == nil || event.Attr.IPv4VxlanMacAddr == nil || event.Attr.PublicIPv4 == nil {
			log.Log.Warnf("Invaild event")
			return
		}
		delHandle(v.Ipv4Vxlan.Attrs().Index, event.IPv4PodCIDR, event.Attr.PublicIPv4, event.Attr.IPv4VxlanMacAddr)
	}
	if v.Ipv6Vxlan != nil {
		if event.IPv6PodCIDR == nil || event.Attr.IPv6VxlanMacAddr == nil || event.Attr.PublicIPv6 == nil {
			log.Log.Warnf("Invaild event")
			return
		}
		delHandle(v.Ipv6Vxlan.Attrs().Index, event.IPv6PodCIDR, event.Attr.PublicIPv6, event.Attr.IPv6VxlanMacAddr)
	}
}

func AddVxlanInfo(clientset *kubernetes.Clientset, n *corev1.Node) error {
	link, err := netlink.LinkByName(constant.VXLANName)
	if err != nil {
		return err
	}
	hardwareAddr := hardware.FromNetHardware(&link.Attrs().HardwareAddr)
	oldAnnotations := nodeMetadata.GetAnnotations(n)
	if oldAnnotations != nil && oldAnnotations.IPv4VxlanMacAddr.Equal(hardwareAddr) {
		return nil
	}
	//TODO: ADD IPv6 Support
	PublicIP, err := devices.GetHostIP(devices.IPv4)
	if err != nil {
		return err
	}
	annotations := nodeMetadata.Annotations{IPv4VxlanMacAddr: *hardwareAddr, PublicIPv4: PublicIP}
	err = nodeMetadata.AddAnnotationsForNode(clientset, &annotations, n)
	if err != nil {
		return err
	}
	return nil
}
