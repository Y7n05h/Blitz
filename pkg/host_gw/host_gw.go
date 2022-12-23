package host_gw

import (
	"blitz/pkg/events"
	"blitz/pkg/log"
	"syscall"

	cip "github.com/containernetworking/plugins/pkg/ip"
	"github.com/vishvananda/netlink"
)

var _ events.EventHandle = (*Handle)(nil)

type Handle struct {
	NodeName string
	Link     netlink.Link
}

func (h *Handle) AddHandle(event *events.Event) {
	if event.Name == h.NodeName {
		return
	}
	route := netlink.Route{
		LinkIndex: h.Link.Attrs().Index,
		Scope:     netlink.SCOPE_UNIVERSE,
		Dst:       event.PodCIDR.ToNetIPNet(),
		Gw:        cip.NextIP(event.PodCIDR.IP),
		Flags:     syscall.RTNH_F_ONLINK,
	}
	err := netlink.RouteAdd(&route)
	if err != nil {
		log.Log.Error("Add Route Failed")
		return
	}
}
func (h *Handle) DelHandle(event *events.Event) {
	if event.Name == h.NodeName {
		return
	}
	route := netlink.Route{
		LinkIndex: h.Link.Attrs().Index,
		Scope:     netlink.SCOPE_UNIVERSE,
		Dst:       event.PodCIDR.ToNetIPNet(),
		Gw:        cip.NextIP(event.PodCIDR.IP),
		Flags:     syscall.RTNH_F_ONLINK,
	}
	err := netlink.RouteDel(&route)
	if err != nil {
		log.Log.Error("Add Route Failed")
		return
	}
}
