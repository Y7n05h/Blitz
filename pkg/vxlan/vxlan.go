package vxlan

import (
	"blitz/pkg/config"
	"blitz/pkg/constant"
	"blitz/pkg/devices"
	"blitz/pkg/events"
	"blitz/pkg/hardware"
	"blitz/pkg/ipnet"
	"blitz/pkg/log"
	nodeMetadata "blitz/pkg/node"
	"fmt"
	"net"
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
func Register(nodeName string, storage *config.PlugStorage, clientset *kubernetes.Clientset, node *corev1.Node) (*Handle, error) {
	annotations := nodeMetadata.Annotations{}
	vxlanHandle := Handle{NodeName: nodeName}
	var err error
	if storage.EnableIPv4() {
		annotations.PublicIPv4, err = devices.GetHostIP(devices.IPv4)
		if err != nil {
			return nil, err
		}
		vxlanHandle.Ipv4Vxlan, err = devices.SetupVXLAN(ipnet.FromIPAndMask(storage.Ipv4Cfg.PodCIDR.IP, net.CIDRMask(32, 32)), annotations.PublicIPv4.IP, constant.VXLANName)
		if err != nil {
			log.Log.Error("SetupVXLAN:", err)
			return nil, err
		}
		log.Log.Debug("SetupVXLAN for IPv4 Success")
		macAddr := *hardware.FromNetHardware(&vxlanHandle.Ipv4Vxlan.Attrs().HardwareAddr)
		if macAddr == nil {
			return nil, fmt.Errorf("get Mac Addr Error")
		}
		annotations.IPv4VxlanMacAddr = macAddr
	}
	if storage.EnableIPv6() {
		annotations.PublicIPv6, err = devices.GetHostIP(devices.IPv6)
		if err != nil {
			return nil, err
		}
		vxlanHandle.Ipv6Vxlan, err = devices.SetupVXLAN(ipnet.FromIPAndMask(storage.Ipv6Cfg.PodCIDR.IP, net.CIDRMask(128, 128)), annotations.PublicIPv6.IP, constant.VXLANName+"v6")
		if err != nil {
			log.Log.Error("SetupVXLAN:", err)
			return nil, err
		}
		log.Log.Debug("SetupVXLAN for IPv6 Success")
		macAddr := *hardware.FromNetHardware(&vxlanHandle.Ipv6Vxlan.Attrs().HardwareAddr)
		if macAddr == nil {
			return nil, fmt.Errorf("get Mac Addr Error")
		}
		annotations.IPv6VxlanMacAddr = macAddr
	}
	err = nodeMetadata.AddAnnotationsForNode(clientset, &annotations, node)
	if err != nil {
		return nil, err
	}
	return &vxlanHandle, nil
}
