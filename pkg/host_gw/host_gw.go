package host_gw

import (
	"blitz/pkg/config"
	"blitz/pkg/devices"
	"blitz/pkg/events"
	"blitz/pkg/log"
	nodeMetadata "blitz/pkg/node"

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
func Register(nodeName string, storage *config.PlugStorage, annotations *nodeMetadata.Annotations) (*Handle, error) {
	hostGwHandle := Handle{NodeName: nodeName}
	if storage.EnableIPv4() {
		defaultLink, err := devices.GetDefaultGateway(devices.IPv4)
		if err != nil {
			log.Log.Debug("No valid route")
			return nil, err
		}
		hostGwHandle.IPv4Link = defaultLink
		hostIP, err := devices.GetHostIP(devices.IPv4)
		if err != nil {
			return nil, err
		}
		annotations.PublicIPv4 = hostIP
	}
	if storage.EnableIPv6() {
		defaultLink, err := devices.GetDefaultGateway(devices.IPv6)
		if err != nil {
			log.Log.Debug("No valid route")
			return nil, err
		}
		hostGwHandle.IPv6Link = defaultLink
		hostIP, err := devices.GetHostIP(devices.IPv6)
		if err != nil {
			return nil, err
		}
		annotations.PublicIPv6 = hostIP
	}
	return &hostGwHandle, nil
}
