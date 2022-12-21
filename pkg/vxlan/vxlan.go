package vxlan

import (
	"syscall"
	"tiny_cni/pkg/devices"
	"tiny_cni/pkg/events"
	"tiny_cni/pkg/log"

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
	log.Log.Debugf("[Reconciler]DEBUG Node:%s annotations:%v %#v", event.Name, event.Attr, event.Attr)
	err = devices.AddFDB(ifIdx, event.Attr.PublicIP.IP, event.Attr.VxlanMacAddr)
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
	err := devices.DelFDB(v.Vxlan.Attrs().Index, event.Attr.PublicIP.IP, event.Attr.VxlanMacAddr)
	if err != nil {
		log.Log.Error("Del ARP Failed: ", err)
	}
}
