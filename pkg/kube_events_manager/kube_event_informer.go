package kube_events_manager

import (
	"fmt"
	"strings"
	"time"

	"github.com/romana/rlog"
	uuid "gopkg.in/satori/go.uuid.v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"

	"github.com/flant/shell-operator/pkg/kube"
	utils_checksum "github.com/flant/shell-operator/pkg/utils/checksum"
	utils_data "github.com/flant/shell-operator/pkg/utils/data"
)

type KubeEventsInformer struct {
	Monitor   *MonitorConfig
	Namespace string

	ConfigId string

	Checksum           map[string]string
	SharedInformer     cache.SharedInformer
	SharedInformerStop chan struct{}
}

func NewKubeEventsInformer(monitor *MonitorConfig) *KubeEventsInformer {
	informer := &KubeEventsInformer{
		Monitor:            monitor,
		Checksum:           make(map[string]string),
		SharedInformerStop: make(chan struct{}, 1),
	}
	return informer
}

func (ei *KubeEventsInformer) UpdateConfigId() string {
	ei.ConfigId = uuid.NewV4().String()
	if ei.Monitor.ConfigIdPrefix != "" {
		ei.ConfigId = ei.Monitor.ConfigIdPrefix + "-" + ei.ConfigId[len(ei.Monitor.ConfigIdPrefix)+1:]
	}
	return ei.ConfigId
}

func (ei *KubeEventsInformer) CreateSharedInformer() error {
	formatSelector, err := formatLabelSelector(ei.Monitor.LabelSelector)
	if err != nil {
		return fmt.Errorf("format label selector '%s': %s", ei.Monitor.LabelSelector.String(), err)
	}

	fieldSelector, err := formatFieldSelector(ei.Monitor.FieldSelector)
	if err != nil {
		return fmt.Errorf("format field selector '%s': %s", ei.Monitor.LabelSelector.String(), err)
	}

	indexers := cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}
	resyncPeriod := time.Duration(2) * time.Hour
	tweakListOptions := func(options *metav1.ListOptions) {
		if fieldSelector != "" {
			options.FieldSelector = fieldSelector
		}
		if formatSelector != "" {
			options.LabelSelector = formatSelector
		}
	}

	var sharedInformer cache.SharedIndexInformer

	rlog.Debugf("KUBE_EVENTS %s informer: discover GVR for kind '%s'...", ei.ConfigId, ei.Monitor.Kind)
	gvr, err := kube.GroupVersionResourceByKind(ei.Monitor.Kind)
	if err != nil {
		rlog.Errorf("KUBE_EVENTS %s informer: Cannot get GroupVersionResource info for kind '%s' from api-server. Possibly CRD is not created before informers are started. Error was: %v", ei.ConfigId, ei.Monitor.Kind, err)
		return err
	}
	rlog.Debugf("KUBE_EVENTS %s informer: GVR for kind '%s' is '%s'", ei.ConfigId, ei.Monitor.Kind, gvr.String())

	informer := dynamicinformer.NewFilteredDynamicInformer(kube.DynamicClient, gvr, ei.Namespace, resyncPeriod, indexers, tweakListOptions)
	sharedInformer = informer.Informer()

	listOptions := metav1.ListOptions{}
	tweakListOptions(&listOptions)

	objList, err := kube.DynamicClient.Resource(gvr).Namespace(ei.Namespace).List(listOptions)
	if err != nil {
		rlog.Errorf("KUBE_EVENTS %s informer: initial list resources of kind '%s': %v", ei.ConfigId, ei.Monitor.Kind, err)
		return err
	}
	err = ei.CalculateChecksumFromExistingObjects(objList)
	if err != nil {
		return err
	}

	ei.SharedInformer = sharedInformer
	ei.SharedInformer.AddEventHandler(SharedInformerEventHandler(ei))

	return nil
}

func SharedInformerEventHandler(informer *KubeEventsInformer) cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			objectId, err := runtimeResourceId(obj, informer.Monitor.Kind)
			if err != nil {
				rlog.Errorf("KUBE_EVENTS %s informer: add: get object id: %s", informer.ConfigId, err)
				return
			}

			filtered, err := resourceFilter(obj, informer.Monitor.JqFilter)
			if err != nil {
				rlog.Errorf("KUBE_EVENTS %s informer: add: apply jqFilter on %s: %s",
					informer.ConfigId, objectId, err)
				return
			}

			checksum := utils_checksum.CalculateChecksum(filtered)

			if informer.ShouldHandleEvent(KubeEventAdd) {
				rlog.Debugf("KUBE_EVENTS %s informer: add: %s object: jqFilter '%s' output:\n%s",
					informer.ConfigId,
					objectId,
					informer.Monitor.JqFilter,
					utils_data.FormatJsonDataOrError(utils_data.FormatPrettyJson(filtered)))
				informer.HandleKubeEvent(obj, objectId, checksum, KubeEventAdd)
			}
		},
		UpdateFunc: func(_ interface{}, obj interface{}) {
			objectId, err := runtimeResourceId(obj, informer.Monitor.Kind)
			if err != nil {
				rlog.Errorf("KUBE_EVENTS %s informer: update: get object id: %s", informer.ConfigId, err)
				return
			}

			filtered, err := resourceFilter(obj, informer.Monitor.JqFilter)
			if err != nil {
				rlog.Errorf("KUBE_EVENTS %s informer: update: apply jqFilter on %s: %s",
					informer.ConfigId, objectId, err)
				return
			}

			checksum := utils_checksum.CalculateChecksum(filtered)

			if informer.ShouldHandleEvent(KubeEventUpdate) {
				rlog.Debugf("KUBE_EVENTS %s informer: update: %s object: jqFilter '%s' output:\n%s",
					informer.ConfigId,
					objectId,
					informer.Monitor.JqFilter,
					utils_data.FormatJsonDataOrError(utils_data.FormatPrettyJson(filtered)))
				informer.HandleKubeEvent(obj, objectId, checksum, KubeEventUpdate)
			}

		},
		DeleteFunc: func(obj interface{}) {
			objectId, err := runtimeResourceId(obj, informer.Monitor.Kind)
			if err != nil {
				rlog.Errorf("KUBE_EVENTS %s informer: delete: get object id: %s", informer.ConfigId, err)
				return
			}

			if informer.ShouldHandleEvent(KubeEventDelete) {
				rlog.Debugf("KUBE_EVENTS %s informer: delete: %s", informer.ConfigId, objectId)
				informer.HandleKubeEvent(obj, objectId, "", KubeEventDelete)
			}
		},
	}
}

// CalculateChecksumFromExistingObjects fills Checksum map with checksum of existing objects to ignore ADD
// events from them.
func (ei *KubeEventsInformer) CalculateChecksumFromExistingObjects(objList *unstructured.UnstructuredList) error {
	if objList == nil || len(objList.Items) == 0 {
		rlog.Debugf("KUBE_EVENTS %s informer: Got no existing '%s' resources", ei.ConfigId, ei.Monitor.Kind)
		return nil
	}

	rlog.Debugf("KUBE_EVENTS %s informer: Got %d existing '%s' resources: %+v", ei.ConfigId, len(objList.Items), ei.Monitor.Kind, objList.Items)

	for _, obj := range objList.Items {
		resourceId, err := runtimeResourceId(&obj, ei.Monitor.Kind)
		if err != nil {
			return err
		}

		filtered, err := resourceFilter(&obj, ei.Monitor.JqFilter)
		if err != nil {
			return err
		}

		ei.Checksum[resourceId] = utils_checksum.CalculateChecksum(filtered)

		rlog.Debugf("KUBE_EVENTS %s informer: initial checksum of %s is %s. jqFilter '%s' output:\n%s",
			ei.ConfigId,
			resourceId,
			ei.Checksum[resourceId],
			ei.Monitor.JqFilter,
			utils_data.FormatJsonDataOrError(utils_data.FormatPrettyJson(filtered)))
	}

	return nil
}

// HandleKubeEvent sends new KubeEvent to KubeEventCh
// obj doesn't contains Kind information, so kind is passed from Run() argument.
// TODO refactor: pass KubeEvent as argument
// TODO add delay to merge Added and Modified events (node added and then labels applied — one hook run on Added+Modifed is enough)
func (ei *KubeEventsInformer) HandleKubeEvent(obj interface{}, objectId string, newChecksum string, eventType KubeEventType) {
	if ei.Checksum[objectId] != newChecksum {
		ei.Checksum[objectId] = newChecksum

		rlog.Debugf("KUBE_EVENTS %s informer: %+v %s: checksum changed, send KubeEvent",
			ei.ConfigId,
			strings.ToUpper(string(eventType)),
			objectId,
		)
		// Safe to ignore an error because of previous call to runtimeResourceId()
		namespace, name, _ := metaFromEventObject(obj.(runtime.Object))
		KubeEventCh <- KubeEvent{
			ConfigId:  ei.ConfigId,
			Events:    []KubeEventType{eventType},
			Namespace: namespace,
			Kind:      ei.Monitor.Kind,
			Name:      name,
		}
	} else {
		rlog.Debugf("KUBE_EVENTS %s informer: %+v %s: checksum has not changed",
			ei.ConfigId,
			strings.ToUpper(string(eventType)),
			objectId,
		)
	}

	return
}

func (ei *KubeEventsInformer) ShouldHandleEvent(checkEvent KubeEventType) bool {
	for _, event := range ei.Monitor.EventTypes {
		if event == checkEvent {
			return true
		}
	}
	return false
}

func (ei *KubeEventsInformer) Run() {
	rlog.Debugf("KUBE_EVENTS %s informer: RUN", ei.ConfigId)
	ei.SharedInformer.Run(ei.SharedInformerStop)
}

func (ei *KubeEventsInformer) Stop() {
	rlog.Debugf("KUBE_EVENTS %s informer: STOP", ei.ConfigId)
	close(ei.SharedInformerStop)
}
