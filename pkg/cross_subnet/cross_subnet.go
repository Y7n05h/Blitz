package cross_subneu

import (
	"blitz/pkg/events"
	"blitz/pkg/host_gw"
	"blitz/pkg/ipnet"
	"blitz/pkg/log"
	"blitz/pkg/vxlan"
)

var _ events.EventHandle = (*Handle)(nil)

type Handle struct {
	vxlanHandle  *vxlan.Handle
	hostGwHandle *host_gw.Handle
	ipv4HostCIDR *ipnet.IPNet
	ipv6HostCIDR *ipnet.IPNet
}

func (v *Handle) AddHandle(event *events.Event) {
	if event.Name == v.vxlanHandle.NodeName {
		return
	}
	if v.ipv4HostCIDR != nil {
		if event.Attr.PublicIPv4 == nil {
			log.Log.Errorf("EnableIPv4 but node %s have no Public IPv4 Address", event.Name)
			return
		}
		if v.ipv4HostCIDR.Contains(event.Attr.PublicIPv4.IP) {
			v.hostGwHandle.AddHandle(event)
		} else {
			v.vxlanHandle.AddHandle(event)
		}
	}
	if v.ipv6HostCIDR != nil {
		if event.Attr.PublicIPv6 == nil {
			log.Log.Errorf("EnableIPv6 but node %s have no Public IPv6 Address", event.Name)
			return
		}
		if v.ipv6HostCIDR.Contains(event.Attr.PublicIPv6.IP) {
			v.hostGwHandle.AddHandle(event)
		} else {
			v.vxlanHandle.AddHandle(event)
		}
	}
}

func (v *Handle) DelHandle(event *events.Event) {
	if event.Name == v.vxlanHandle.NodeName {
		return
	}
	if v.ipv4HostCIDR != nil {
		if event.Attr.PublicIPv4 == nil {
			log.Log.Errorf("EnableIPv4 but node %s have no Public IPv4 Address", event.Name)
			return
		}
		if v.ipv4HostCIDR.Contains(event.Attr.PublicIPv4.IP) {
			v.hostGwHandle.DelHandle(event)
		} else {
			v.vxlanHandle.DelHandle(event)
		}
	}
	if v.ipv6HostCIDR != nil {
		if event.Attr.PublicIPv6 == nil {
			log.Log.Errorf("EnableIPv6 but node %s have no Public IPv6 Address", event.Name)
			return
		}
		if v.ipv6HostCIDR.Contains(event.Attr.PublicIPv6.IP) {
			v.hostGwHandle.DelHandle(event)
		} else {
			v.vxlanHandle.DelHandle(event)
		}
	}
}
