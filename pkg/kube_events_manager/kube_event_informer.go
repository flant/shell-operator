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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"

	"github.com/flant/shell-operator/pkg/kube"
	utils_checksum "github.com/flant/shell-operator/pkg/utils/checksum"
	utils_data "github.com/flant/shell-operator/pkg/utils/data"
)

type KubeEventsInformer interface {
	UpdateConfigId() string
	WithConfigId(string)
	WithNamespace(string)
	CreateSharedInformer() error
	Run()
	Stop()
}

type MainKubeEventsInformer struct {
	Monitor   *MonitorConfig
	Namespace string

	ConfigId string

	Checksum           map[string]string
	SharedInformer     cache.SharedInformer
	SharedInformerStop chan struct{}
}

var NewKubeEventsInformer = func(monitor *MonitorConfig) KubeEventsInformer {
	informer := &MainKubeEventsInformer{
		Monitor:            monitor,
		Checksum:           make(map[string]string),
		SharedInformerStop: make(chan struct{}, 1),
	}
	return informer
}

func (ei *MainKubeEventsInformer) UpdateConfigId() string {
	ei.ConfigId = uuid.NewV4().String()
	if ei.Monitor.ConfigIdPrefix != "" {
		ei.ConfigId = ei.Monitor.ConfigIdPrefix + "-" + ei.ConfigId[len(ei.Monitor.ConfigIdPrefix)+1:]
	}
	return ei.ConfigId
}

func (ei *MainKubeEventsInformer) WithConfigId(configId string) {
	ei.ConfigId = configId
}

func (ei *MainKubeEventsInformer) WithNamespace(ns string) {
	ei.Namespace = ns
}

func (ei *MainKubeEventsInformer) CreateSharedInformer() (err error) {
	// discover GroupVersionResource for informer
	var gvr schema.GroupVersionResource

	rlog.Debugf("KUBE_EVENTS %s informer: discover GVR for apiVersion '%s' kind '%s'...", ei.ConfigId, ei.Monitor.ApiVersion, ei.Monitor.Kind)
	gvr, err = kube.GroupVersionResource(ei.Monitor.ApiVersion, ei.Monitor.Kind)
	if err != nil {
		rlog.Errorf("KUBE_EVENTS %s informer: Cannot get GroupVersionResource info for apiVersion '%s' kind '%s' from api-server. Possibly CRD is not created before informers are started. Error was: %v", ei.ConfigId, ei.Monitor.ApiVersion, ei.Monitor.Kind, err)
		return err
	}
	rlog.Debugf("KUBE_EVENTS %s informer: GVR for kind '%s' is '%s'", ei.ConfigId, ei.Monitor.Kind, gvr.String())

	// define resyncPeriod for informer
	resyncPeriod := time.Duration(2) * time.Hour

	// define indexers for informer
	indexers := cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}

	// define tweakListOptions for informer
	formatSelector, err := FormatLabelSelector(ei.Monitor.LabelSelector)
	if err != nil {
		return fmt.Errorf("format label selector '%s': %s", ei.Monitor.LabelSelector.String(), err)
	}
	fieldSelector, err := FormatFieldSelector(ei.Monitor.FieldSelector)
	if err != nil {
		return fmt.Errorf("format field selector '%+v': %s", ei.Monitor.FieldSelector, err)
	}
	tweakListOptions := func(options *metav1.ListOptions) {
		if fieldSelector != "" {
			options.FieldSelector = fieldSelector
		}
		if formatSelector != "" {
			options.LabelSelector = formatSelector
		}
	}

	// create informer with add, update, delete callbacks
	informer := dynamicinformer.NewFilteredDynamicInformer(kube.DynamicClient, gvr, ei.Namespace, resyncPeriod, indexers, tweakListOptions)
	informer.Informer().AddEventHandler(SharedInformerEventHandler(ei))
	ei.SharedInformer = informer.Informer()

	// initialize ei.Checksum
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

	return nil
}

var SharedInformerEventHandler = func(informer *MainKubeEventsInformer) cache.ResourceEventHandlerFuncs {
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

			if informer.ShouldHandleEvent(WatchEventAdded) {
				rlog.Debugf("KUBE_EVENTS %s informer: add: %s object: jqFilter '%s' output:\n%s",
					informer.ConfigId,
					objectId,
					informer.Monitor.JqFilter,
					utils_data.FormatJsonDataOrError(utils_data.FormatPrettyJson(filtered)))
				informer.HandleKubeEvent(obj, objectId, filtered, checksum, WatchEventAdded)
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

			if informer.ShouldHandleEvent(WatchEventModified) {
				rlog.Debugf("KUBE_EVENTS %s informer: update: %s object: jqFilter '%s' output:\n%s",
					informer.ConfigId,
					objectId,
					informer.Monitor.JqFilter,
					utils_data.FormatJsonDataOrError(utils_data.FormatPrettyJson(filtered)))
				informer.HandleKubeEvent(obj, objectId, filtered, checksum, WatchEventModified)
			}

		},
		DeleteFunc: func(obj interface{}) {
			objectId, err := runtimeResourceId(obj, informer.Monitor.Kind)
			if err != nil {
				rlog.Errorf("KUBE_EVENTS %s informer: delete: get object id: %s", informer.ConfigId, err)
				return
			}

			if informer.ShouldHandleEvent(WatchEventDeleted) {
				rlog.Debugf("KUBE_EVENTS %s informer: delete: %s", informer.ConfigId, objectId)
				informer.HandleKubeEvent(obj, objectId, "", "", WatchEventDeleted)
			}
		},
	}
}

// CalculateChecksumFromExistingObjects fills Checksum map with checksum of existing objects to ignore ADD
// events from them.
func (ei *MainKubeEventsInformer) CalculateChecksumFromExistingObjects(objList *unstructured.UnstructuredList) error {
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
func (ei *MainKubeEventsInformer) HandleKubeEvent(obj interface{}, objectId string, filterResult string, newChecksum string, eventType WatchEventType) {
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
			ConfigId:     ei.ConfigId,
			Events:       []WatchEventType{eventType},
			Namespace:    namespace,
			Kind:         ei.Monitor.Kind,
			Name:         name,
			Object:       obj,
			FilterResult: filterResult,
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

func (ei *MainKubeEventsInformer) ShouldHandleEvent(checkEvent WatchEventType) bool {
	for _, event := range ei.Monitor.EventTypes {
		if event == checkEvent {
			return true
		}
	}
	return false
}

func (ei *MainKubeEventsInformer) Run() {
	rlog.Debugf("KUBE_EVENTS %s informer: RUN", ei.ConfigId)
	ei.SharedInformer.Run(ei.SharedInformerStop)
}

func (ei *MainKubeEventsInformer) Stop() {
	rlog.Debugf("KUBE_EVENTS %s informer: STOP", ei.ConfigId)
	close(ei.SharedInformerStop)
}
