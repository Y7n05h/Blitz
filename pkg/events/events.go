package events

import (
	"tiny_cni/pkg/ipnet"
	"tiny_cni/pkg/log"
	nodeMetadata "tiny_cni/pkg/node"

	corev1 "k8s.io/api/core/v1"
)

type EventType uint32

const (
	Add EventType = 0
	Del EventType = 1
	//Update EventType = 2
)

type Event struct {
	Type    EventType
	Name    string
	PodCIDR ipnet.IPNet
	Attr    nodeMetadata.Annotations
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
	cidr, err := nodeMetadata.GetPodCIDR(n)
	log.Log.Debugf("[reconciler]Node:%s annotations:%v %#v", n.Name, annotations, annotations)
	if err != nil {
		log.Log.Warn("Get Cidr From Node Failed", err)
		return nil
	}
	return &Event{
		Type:    eventType,
		Name:    n.Name,
		PodCIDR: *cidr,
		Attr:    *annotations,
	}
}
func (e *Event) Equal(event *Event) bool {
	return (e == event) || (e != nil && event != nil && e.Type == event.Type && e.Name == event.Name && e.PodCIDR.Equal(&event.PodCIDR) && e.Attr.Equal(&event.Attr))
}
