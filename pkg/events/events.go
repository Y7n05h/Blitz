package events

import (
	"blitz/pkg/ipnet"
	"blitz/pkg/log"
	nodeMetadata "blitz/pkg/node"

	corev1 "k8s.io/api/core/v1"
)

type EventType uint32

const (
	Add EventType = 0
	Del EventType = 1
	//Update EventType = 2
)

type Event struct {
	Type        EventType
	Name        string
	IPv4PodCIDR *ipnet.IPNet
	IPv6PodCIDR *ipnet.IPNet
	Attr        nodeMetadata.Annotations
}
type EventHandle interface {
	AddHandle(event *Event)
	DelHandle(event *Event)
}

func FromNode(n *corev1.Node, eventType EventType) *Event {
	annotations := nodeMetadata.GetAnnotations(n)
	if annotations == nil {
		return nil
	}
	cidrs, err := nodeMetadata.GetPodCIDRs(n)
	log.Log.Debugf("[reconciler]Node:%s annotations:%v %#v", n.Name, annotations, annotations)
	if err != nil {
		log.Log.Warn("Get Cidr From Node Failed", err)
		return nil
	}
	ipv4CIDR, ipv6CIDR := ipnet.SelectIPv4AndIPv6(cidrs)
	return &Event{
		Type:        eventType,
		Name:        n.Name,
		IPv4PodCIDR: ipv4CIDR,
		IPv6PodCIDR: ipv6CIDR,
		Attr:        *annotations,
	}
}
func (e *Event) Equal(event *Event) bool {
	return (e == event) || (e != nil && event != nil && e.Type == event.Type && e.Name == event.Name && e.IPv4PodCIDR.Equal(event.IPv4PodCIDR) && e.IPv6PodCIDR.Equal(event.IPv6PodCIDR) && e.Attr.Equal(&event.Attr))
}
