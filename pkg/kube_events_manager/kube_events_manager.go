package kube_events_manager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/romana/rlog"
	"gopkg.in/satori/go.uuid.v1"
	"k8s.io/client-go/dynamic/dynamicinformer"

	"k8s.io/apimachinery/pkg/api/meta"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"

	"github.com/flant/shell-operator/pkg/executor"
	"github.com/flant/shell-operator/pkg/kube"
	utils_checksum "github.com/flant/shell-operator/pkg/utils/checksum"
	utils_data "github.com/flant/shell-operator/pkg/utils/data"
)

type KubeEventsManager interface {
	Run(eventTypes []OnKubernetesEventType, kind, namespace string, labelSelector *metaV1.LabelSelector, jqFilter string, debug bool) (string, error)
	Stop(configId string) error
}

type MainKubeEventsManager struct {
	// all created kube informers. Informers are addressed by config id — uuid
	KubeEventsInformersByConfigId map[string]*KubeEventsInformer
}

func NewMainKubeEventsManager() *MainKubeEventsManager {
	em := &MainKubeEventsManager{}
	em.KubeEventsInformersByConfigId = make(map[string]*KubeEventsInformer)
	return em
}

func Init() (KubeEventsManager, error) {
	em := NewMainKubeEventsManager()
	KubeEventCh = make(chan KubeEvent, 1)
	return em, nil
}

func (em *MainKubeEventsManager) Run(eventTypes []OnKubernetesEventType, kind, namespace string, labelSelector *metaV1.LabelSelector, jqFilter string, debug bool) (string, error) {
	kubeEventsInformer, err := em.addKubeEventsInformer(kind, namespace, labelSelector, eventTypes, jqFilter, debug, func(kubeEventsInformer *KubeEventsInformer) cache.ResourceEventHandlerFuncs {
		return cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				objectId, err := runtimeResourceId(obj)
				if err != nil {
					rlog.Errorf("failed to get object id: %s", err)
					return
				}

				filtered, err := resourceFilter(obj, jqFilter, debug)
				if err != nil {
					rlog.Errorf("Kube events manager: %s/%+v informer %s got %s of object kind=%s %s", kind, eventTypes, kubeEventsInformer.ConfigId, kind, objectId, err)
					return
				}

				checksum := utils_checksum.CalculateChecksum(filtered)

				if debug && kubeEventsInformer.ShouldHandleEvent(KubernetesEventOnAdd) {
					rlog.Debugf("Kube events manager: %s/%+v informer %s: add %s object %s: jqFilter '%s': calculated checksum '%s' of object being watched:\n%s",
						kind, eventTypes, kubeEventsInformer.ConfigId, kind, objectId, jqFilter, checksum, utils_data.FormatJsonDataOrError(utils_data.FormatPrettyJson(filtered)))
				}

				err = kubeEventsInformer.HandleKubeEvent(obj, kind, checksum, string(KubernetesEventOnAdd), kubeEventsInformer.ShouldHandleEvent(KubernetesEventOnAdd), debug)
				if err != nil {
					rlog.Errorf("Kube events manager: %+v informer %s: %s object %s: %s", eventTypes, kubeEventsInformer.ConfigId, kind, objectId, err)
					return
				}
			},
			UpdateFunc: func(_ interface{}, obj interface{}) {
				objectId, err := runtimeResourceId(obj)
				if err != nil {
					rlog.Errorf("failed to get object id: %s", err)
					return
				}

				filtered, err := resourceFilter(obj, jqFilter, debug)
				if err != nil {
					rlog.Errorf("Kube events manager: %+v informer %s: %s object %s: %s", eventTypes, kubeEventsInformer.ConfigId, kind, objectId, err)
					return
				}

				checksum := utils_checksum.CalculateChecksum(filtered)

				if debug && kubeEventsInformer.ShouldHandleEvent(KubernetesEventOnUpdate) {
					rlog.Debugf("Kube events manager: %+v informer %s: update %s object %s: jqFilter '%s': calculated checksum '%s' of object being watched:\n%s",
						eventTypes, kubeEventsInformer.ConfigId, kind, objectId, jqFilter, checksum, utils_data.FormatJsonDataOrError(utils_data.FormatPrettyJson(filtered)))
				}

				err = kubeEventsInformer.HandleKubeEvent(obj, kind, checksum, string(KubernetesEventOnUpdate), kubeEventsInformer.ShouldHandleEvent(KubernetesEventOnUpdate), debug)
				if err != nil {
					rlog.Errorf("Kube events manager: %+v informer %s: %s object %s: %s", eventTypes, kubeEventsInformer.ConfigId, kind, objectId, err)
					return
				}
			},
			DeleteFunc: func(obj interface{}) {
				objectId, err := runtimeResourceId(obj)
				if err != nil {
					rlog.Errorf("failed to get object id: %s", err)
					return
				}

				if debug && kubeEventsInformer.ShouldHandleEvent(KubernetesEventOnDelete) {
					rlog.Debugf("Kube events manager: %+v informer %s: delete %s object %s", eventTypes, kubeEventsInformer.ConfigId, kind, objectId)
				}

				err = kubeEventsInformer.HandleKubeEvent(obj, kind, "", string(KubernetesEventOnDelete), kubeEventsInformer.ShouldHandleEvent(KubernetesEventOnDelete), debug)
				if err != nil {
					rlog.Errorf("Kube events manager: %+v informer %s: %s object %s: %s", eventTypes, kubeEventsInformer.ConfigId, kind, objectId, err)
					return
				}
			},
		}
	})

	if err != nil {
		return "", err
	}

	go kubeEventsInformer.Run()

	return kubeEventsInformer.ConfigId, nil
}

func (em *MainKubeEventsManager) addKubeEventsInformer(kind, namespace string, labelSelector *metaV1.LabelSelector, eventTypes []OnKubernetesEventType, jqFilter string, debug bool, resourceEventHandlerFuncs func(kubeEventsInformer *KubeEventsInformer) cache.ResourceEventHandlerFuncs) (*KubeEventsInformer, error) {
	kubeEventsInformer := NewKubeEventsInformer()
	kubeEventsInformer.ConfigId = uuid.NewV4().String()
	kubeEventsInformer.Kind = kind
	kubeEventsInformer.EventTypes = eventTypes
	kubeEventsInformer.JqFilter = jqFilter

	formatSelector, err := formatLabelSelector(labelSelector)
	if err != nil {
		return nil, fmt.Errorf("failed format label selector '%s'", labelSelector.String())
	}

	indexers := cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}
	resyncPeriod := time.Duration(2) * time.Hour
	tweakListOptions := func(options *metaV1.ListOptions) {
		if formatSelector != "" {
			options.LabelSelector = formatSelector
		}
	}

	var sharedInformer cache.SharedIndexInformer
	var objects []runtime.Object

	rlog.Debugf("Discover GVR for %s...", kind)
	kubeEventsInformer.Kind = "pod"
	gvr, err := kube.GroupVersionResourceByKind(kind)
	if err != nil {
		rlog.Errorf("error getting GVR for kind '%s': %v", kind, err)
		return nil, err
	}
	rlog.Infof("GVR for kind '%s' is %+v", kind, gvr)
	informer := dynamicinformer.NewFilteredDynamicInformer(kube.DynamicClient, gvr, namespace, resyncPeriod, indexers, tweakListOptions)
	sharedInformer = informer.Informer()

	// Save already existed resources to IGNORE watch.Added events about them
	selector, err := metaV1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		rlog.Infof("error creating labelSelector: %v", err)
		return nil, err
	}
	objects, err = informer.Lister().List(selector)
	if err != nil {
		return nil, fmt.Errorf("failed to list '%s' resources: %v", kind, err)
	}
	rlog.Debugf("Got %d objects for kind '%s': %+v", len(objects), kind, objects)
	if len(objects) > 0 {
		err = kubeEventsInformer.InitializeItemsList(objects, debug)
		if err != nil {
			return nil, err
		}
	}

	kubeEventsInformer.SharedInformer = sharedInformer
	kubeEventsInformer.SharedInformer.AddEventHandler(resourceEventHandlerFuncs(kubeEventsInformer))

	em.KubeEventsInformersByConfigId[kubeEventsInformer.ConfigId] = kubeEventsInformer

	return kubeEventsInformer, nil
}

func formatLabelSelector(selector *metaV1.LabelSelector) (string, error) {
	res, err := metaV1.LabelSelectorAsSelector(selector)
	if err != nil {
		return "", err
	}

	return res.String(), nil
}

func resourceFilter(obj interface{}, jqFilter string, debug bool) (res string, err error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}

	if jqFilter != "" {
		stdout, stderr, err := execJq(jqFilter, data, debug)
		if err != nil {
			return "", fmt.Errorf("failed exec jq: \nerr: '%s'\nstderr: '%s'", err, stderr)
		}

		res = stdout
	} else {
		res = string(data)
	}
	return
}

func (em *MainKubeEventsManager) Stop(configId string) error {
	kubeEventsInformer, ok := em.KubeEventsInformersByConfigId[configId]
	if ok {
		kubeEventsInformer.Stop()
	} else {
		rlog.Errorf("Kube events informer '%s' not found!", configId)
	}
	return nil
}

type KubeEventsInformer struct {
	ConfigId           string
	Kind               string
	EventTypes         []OnKubernetesEventType
	JqFilter           string
	Checksum           map[string]string
	SharedInformer     cache.SharedInformer
	SharedInformerStop chan struct{}
}

func NewKubeEventsInformer() *KubeEventsInformer {
	kubeEventsInformer := &KubeEventsInformer{}
	kubeEventsInformer.Checksum = make(map[string]string)
	kubeEventsInformer.SharedInformerStop = make(chan struct{}, 1)
	return kubeEventsInformer
}

func (ei *KubeEventsInformer) InitializeItemsList(objects []runtime.Object, debug bool) error {
	for _, obj := range objects {
		resourceId, err := runtimeResourceId(obj)
		if err != nil {
			return err
		}

		filtered, err := resourceFilter(obj, ei.JqFilter, debug)
		if err != nil {
			return err
		}

		ei.Checksum[resourceId] = utils_checksum.CalculateChecksum(filtered)

		if debug {
			rlog.Debugf("Kube events manager: %+v informer %s: %s object %s initialization: jqFilter '%s': calculated checksum '%s' of object being watched:\n%s",
				ei.EventTypes,
				ei.ConfigId,
				ei.Kind,
				resourceId,
				ei.JqFilter,
				ei.Checksum[resourceId],
				utils_data.FormatJsonDataOrError(utils_data.FormatPrettyJson(filtered)))
		}
	}

	return nil
}

// HandleKubeEvent sends new KubeEvent to KubeEventCh
// obj doesn't contains Kind information, so kind is passed from Run() argument.
// TODO refactor: pass KubeEvent as argument
// TODO add delay to merge Added and Modified events (node added and then labels applied — one hook run on Added+Modifed is enough)
func (ei *KubeEventsInformer) HandleKubeEvent(obj interface{}, kind string, newChecksum string, eventType string, sendSignal bool, debug bool) error {
	objectId, err := runtimeResourceId(obj.(runtime.Object))
	if err != nil {
		return fmt.Errorf("failed to get object id: %s", err)
	}

	if ei.Checksum[objectId] != newChecksum {
		oldChecksum := ei.Checksum[objectId]
		ei.Checksum[objectId] = newChecksum

		if debug {
			rlog.Debugf("Kube events manager: %+v informer %s: %s object %s: checksum has changed: '%s' -> '%s'", ei.EventTypes, ei.ConfigId, ei.Kind, objectId, oldChecksum, newChecksum)
		}

		if sendSignal {
			if debug {
				rlog.Debugf("Kube events manager: %+v informer %s: %s object %s: sending EVENT", ei.EventTypes, ei.ConfigId, ei.Kind, objectId)
			}
			// Safe to ignore an error because of previous call to runtimeResourceId()
			namespace, name, _ := metaFromEventObject(obj.(runtime.Object))
			KubeEventCh <- KubeEvent{
				ConfigId:  ei.ConfigId,
				Events:    []string{eventType},
				Namespace: namespace,
				Kind:      kind,
				Name:      name,
			}
		}
	} else if debug {
		rlog.Debugf("Kube events manager: %+v informer %s: %s object %s: checksum '%s' has not changed", ei.EventTypes, ei.ConfigId, ei.Kind, objectId, newChecksum)
	}

	return nil
}

// metaFromEventObject returns name and namespace from api object
func metaFromEventObject(obj interface{}) (namespace string, name string, err error) {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return
	}
	namespace = accessor.GetNamespace()
	name = accessor.GetName()
	return
}

func runtimeResourceId(obj interface{}) (string, error) {
	namespace, name, err := metaFromEventObject(obj)
	if err != nil {
		return "", err
	}

	return generateChecksumId(name, namespace), nil
}

func generateChecksumId(name, namespace string) string {
	return fmt.Sprintf("name=%s namespace=%s", name, namespace)
}

func (ei *KubeEventsInformer) ShouldHandleEvent(checkEvent OnKubernetesEventType) bool {
	for _, event := range ei.EventTypes {
		if event == checkEvent {
			return true
		}
	}
	return false
}

func (ei *KubeEventsInformer) Run() {
	rlog.Debugf("Kube events manager: run informer %s", ei.ConfigId)
	ei.SharedInformer.Run(ei.SharedInformerStop)
}

func (ei *KubeEventsInformer) Stop() {
	rlog.Debugf("Kube events manager: stop informer %s", ei.ConfigId)
	close(ei.SharedInformerStop)
}

func execJq(jqFilter string, jsonData []byte, debug bool) (stdout string, stderr string, err error) {
	cmd := exec.Command("/usr/bin/jq", jqFilter)

	var stdinBuf bytes.Buffer
	_, err = stdinBuf.WriteString(string(jsonData))
	if err != nil {
		panic(err)
	}
	cmd.Stdin = &stdinBuf
	var stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	err = executor.Run(cmd, debug)
	stdout = strings.TrimSpace(stdoutBuf.String())
	stderr = strings.TrimSpace(stderrBuf.String())

	return
}
