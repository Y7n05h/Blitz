package events

import (
	"tiny_cni/internal/Reconciler"
	"tiny_cni/internal/log"
	node_metadata "tiny_cni/internal/node"
	"tiny_cni/pkg/ipnet"

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
	Attr    node_metadata.Annotations
}
type EventHandle interface {
	AddHandle(event *Event)
	DelHandle(event *Event)
}

func FromNode(n *corev1.Node, eventType EventType) *Event {
	annotations := node_metadata.GetAnnotations(n)
	if annotations == nil {
		return nil
	}
	cidr, err := Reconciler.GetPodCIDR(n)
	log.Log.Debugf("[Reconciler]Node:%s annotations:%v %#v", n.Name, annotations, annotations)
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
