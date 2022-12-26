package host_gw

import (
	"blitz/pkg/events"
	"blitz/pkg/log"

	"github.com/vishvananda/netlink"
)

var _ events.EventHandle = (*Handle)(nil)

type Handle struct {
	NodeName string
	IPv4Link netlink.Link
	IPv6Link netlink.Link
}

func (h *Handle) AddHandle(event *events.Event) {
	if event.Name == h.NodeName {
		return
	}
	if h.IPv4Link != nil {
		if event.Attr.PublicIPv4 == nil {
			log.Log.Errorf("EnableIPv4 but node %s have no Public IPv4 Address", event.Name)
			return
		}
		route := netlink.Route{
			LinkIndex: h.IPv4Link.Attrs().Index,
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
	if h.IPv6Link != nil {
		if event.Attr.PublicIPv6 == nil {
			log.Log.Errorf("EnableIPv6 but node %s have no Public IPv6 Address", event.Name)
			return
		}
		route := netlink.Route{
			LinkIndex: h.IPv6Link.Attrs().Index,
			Scope:     netlink.SCOPE_UNIVERSE,
			Dst:       event.IPv6PodCIDR.ToNetIPNet(),
			Gw:        event.Attr.PublicIPv6.IP,
		}
		err := netlink.RouteAdd(&route)
		if err != nil {
			log.Log.Errorf("Add Route Failed:%v", err)
			return
		}
	}

}
func (h *Handle) DelHandle(event *events.Event) {
	if event.Name == h.NodeName {
		return
	}
	if h.IPv4Link != nil {
		if event.Attr.PublicIPv4 == nil {
			log.Log.Errorf("EnableIPv4 but node %s have no Public IPv4 Address", event.Name)
			return
		}
		route := netlink.Route{
			LinkIndex: h.IPv4Link.Attrs().Index,
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
	if h.IPv6Link != nil {
		if event.Attr.PublicIPv6 == nil {
			log.Log.Errorf("EnableIPv6 but node %s have no Public IPv6 Address", event.Name)
			return
		}
		route := netlink.Route{
			LinkIndex: h.IPv6Link.Attrs().Index,
			Scope:     netlink.SCOPE_UNIVERSE,
			Dst:       event.IPv6PodCIDR.ToNetIPNet(),
			Gw:        event.Attr.PublicIPv6.IP,
		}
		err := netlink.RouteDel(&route)
		if err != nil {
			log.Log.Errorf("Add Route Failed:%v", err)
			return
		}
	}
}
