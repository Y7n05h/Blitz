package host_gw

import (
	"blitz/pkg/events"
	"blitz/pkg/log"

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
		Dst:       event.IPv4PodCIDR.ToNetIPNet(),
		Gw:        event.Attr.PublicIPv4.IP,
	}
	err := netlink.RouteAdd(&route)
	if err != nil {
		log.Log.Errorf("Add Route Failed:%v", err)
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
		Dst:       event.IPv4PodCIDR.ToNetIPNet(),
		Gw:        event.Attr.PublicIPv4.IP,
	}
	err := netlink.RouteDel(&route)
	if err != nil {
		log.Log.Errorf("Add Route Failed:%v", err)
		return
	}
}
