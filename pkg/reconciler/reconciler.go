package reconciler

import (
	"blitz/pkg/config"
	"blitz/pkg/events"
	"blitz/pkg/log"
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	"k8s.io/client-go/kubernetes"
)

const (
	SyncTime = time.Minute
)

type Reconciler struct {
	Clientset   *kubernetes.Clientset
	CniStorage  *config.PlugStorage
	Node        listers.NodeLister
	Controller  cache.Controller
	event       chan *events.Event
	eventHandle events.EventHandle
}

func NewReconciler(ctx context.Context, clientset *kubernetes.Clientset, cniStorage *config.PlugStorage, handle events.EventHandle) (*Reconciler, error) {
	log.Log.Debug("New reconciler")
	listWatch := cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return clientset.CoreV1().Nodes().List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return clientset.CoreV1().Nodes().Watch(ctx, options)
		},
		DisableChunking: false,
	}
	eventCh := make(chan *events.Event, 16)
	reconciler := &Reconciler{Clientset: clientset, CniStorage: cniStorage, event: eventCh, eventHandle: handle}
	handles := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			node := obj.(*corev1.Node)
			event := events.FromNode(node, events.Add)
			if event == nil {
				return
			}
			eventCh <- event
		},
		UpdateFunc: func(oldObj, newObj any) {
			oldNode := oldObj.(*corev1.Node)
			// Note: Use events.Add is not a bug. We need compare two events without type.
			oldNodeEvent := events.FromNode(oldNode, events.Add)
			newNode := newObj.(*corev1.Node)
			newNodeEvent := events.FromNode(newNode, events.Add)
			if oldNodeEvent.Equal(newNodeEvent) {
				return
			}
			if oldNodeEvent != nil {
				oldNodeEvent.Type = events.Del
				eventCh <- oldNodeEvent
			}
			if newNodeEvent != nil {
				eventCh <- newNodeEvent
			}
		},
		DeleteFunc: func(obj any) {
			node := obj.(*corev1.Node)
			event := events.FromNode(node, events.Del)
			if event == nil {
				return
			}
			eventCh <- event
		},
	}
	log.Log.Debug("New IndexerInformer")
	indexer, controller := cache.NewIndexerInformer(&listWatch, &corev1.Node{}, SyncTime, handles, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	reconciler.Node = listers.NewNodeLister(indexer)
	reconciler.Controller = controller
	log.Log.Debug("New reconciler Success")
	return reconciler, nil
}
func (r *Reconciler) Run(ctx context.Context) {
	log.Log.Infof("Run reconciler")
	go r.Controller.Run(ctx.Done())
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-r.event:
			switch event.Type {
			case events.Add:
				log.Log.Debugf("Event Add")
				r.eventHandle.AddHandle(event)
			case events.Del:
				log.Log.Debugf("Event Del")
				r.eventHandle.DelHandle(event)
			}
		}
	}
}
