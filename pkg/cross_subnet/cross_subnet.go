package cross_subneu

import (
	"blitz/pkg/config"
	"blitz/pkg/devices"
	"blitz/pkg/events"
	"blitz/pkg/host_gw"
	"blitz/pkg/ipnet"
	"blitz/pkg/log"
	"blitz/pkg/vxlan"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
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
func Register(nodeName string, storage *config.PlugStorage, clientset *kubernetes.Clientset, node *corev1.Node) (*Handle, error) {
	vxlanHandle, err := vxlan.Register(nodeName, storage, clientset, node)
	if err != nil {
		return nil, fmt.Errorf("create vxlan Handle failed:%w", err)
	}
	HostGwHandle, err := host_gw.Register(nodeName, storage, clientset, node)
	if err != nil {
		return nil, fmt.Errorf("create host-gw Handle failed:%w", err)
	}
	handle := Handle{
		vxlanHandle:  vxlanHandle,
		hostGwHandle: HostGwHandle,
		ipv4HostCIDR: nil,
		ipv6HostCIDR: nil,
	}
	if storage.EnableIPv4() {
		if hostIP, err := devices.GetHostIP(devices.IPv4); err != nil {
			return nil, fmt.Errorf("get IPv4 HostIP failed:%w", err)
		} else {
			handle.ipv4HostCIDR = hostIP
		}
	}
	if storage.EnableIPv6() {
		if hostIP, err := devices.GetHostIP(devices.IPv6); err != nil {
			return nil, fmt.Errorf("get IPv6 HostIP failed:%w", err)
		} else {
			handle.ipv6HostCIDR = hostIP
		}
	}
	return &handle, nil
}
