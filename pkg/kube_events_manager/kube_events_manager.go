package kube_events_manager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"k8s.io/apimachinery/pkg/fields"
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

type KubeEventInformerConfig struct {
	Name          string
	EventTypes    []OnKubernetesEventType
	Kind          string
	Namespace     string
	LabelSelector *metaV1.LabelSelector
	ObjectName    string
	JqFilter      string
}

type KubeEventsManager interface {
	Run(informerConfig KubeEventInformerConfig) (string, error)
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

func (em *MainKubeEventsManager) Run(informerConfig KubeEventInformerConfig) (string, error) {
	kubeEventsInformer, err := em.addKubeEventsInformer(informerConfig, func(informer *KubeEventsInformer) cache.ResourceEventHandlerFuncs {
		return cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				objectId, err := runtimeResourceId(obj, informer.Config.Kind)
				if err != nil {
					rlog.Errorf("KUBE_EVENTS %s informer: add: get object id: %s", informer.ConfigId, err)
					return
				}

				filtered, err := resourceFilter(obj, informer.Config.JqFilter)
				if err != nil {
					rlog.Errorf("KUBE_EVENTS %s informer: add: apply jqFilter on %s: %s",
						informer.ConfigId, objectId, err)
					return
				}

				checksum := utils_checksum.CalculateChecksum(filtered)

				if informer.ShouldHandleEvent(KubernetesEventOnAdd) {
					rlog.Debugf("KUBE_EVENTS %s informer: add: %s object: jqFilter '%s' output:\n%s",
						informer.ConfigId,
						objectId,
						informer.Config.JqFilter,
						utils_data.FormatJsonDataOrError(utils_data.FormatPrettyJson(filtered)))
					informer.HandleKubeEvent(obj, objectId, checksum, KubernetesEventOnAdd)
				}
			},
			UpdateFunc: func(_ interface{}, obj interface{}) {
				objectId, err := runtimeResourceId(obj, informer.Config.Kind)
				if err != nil {
					rlog.Errorf("KUBE_EVENTS %s informer: update: get object id: %s", informer.ConfigId, err)
					return
				}

				filtered, err := resourceFilter(obj, informer.Config.JqFilter)
				if err != nil {
					rlog.Errorf("KUBE_EVENTS %s informer: update: apply jqFilter on %s: %s",
						informer.ConfigId, objectId, err)
					return
				}

				checksum := utils_checksum.CalculateChecksum(filtered)

				if informer.ShouldHandleEvent(KubernetesEventOnUpdate) {
					rlog.Debugf("KUBE_EVENTS %s informer: update: %s object: jqFilter '%s' output:\n%s",
						informer.ConfigId,
						objectId,
						informer.Config.JqFilter,
						utils_data.FormatJsonDataOrError(utils_data.FormatPrettyJson(filtered)))
					informer.HandleKubeEvent(obj, objectId, checksum, KubernetesEventOnUpdate)
				}

			},
			DeleteFunc: func(obj interface{}) {
				objectId, err := runtimeResourceId(obj, informer.Config.Kind)
				if err != nil {
					rlog.Errorf("KUBE_EVENTS %s informer: delete: get object id: %s", informer.ConfigId, err)
					return
				}

				if informer.ShouldHandleEvent(KubernetesEventOnDelete) {
					rlog.Debugf("KUBE_EVENTS %s informer: delete: %s", informer.ConfigId, objectId)
					informer.HandleKubeEvent(obj, objectId, "", KubernetesEventOnDelete)
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

func (em *MainKubeEventsManager) addKubeEventsInformer(informerConfig KubeEventInformerConfig, resourceEventHandlerFuncs func(kubeEventsInformer *KubeEventsInformer) cache.ResourceEventHandlerFuncs) (*KubeEventsInformer, error) {
	kubeEventsInformer := NewKubeEventsInformer()
	kubeEventsInformer.Config = informerConfig
	// if informer config has name, set it as prefix of configId
	kubeEventsInformer.ConfigId = informerConfig.Name + "-" + uuid.NewV4().String()[len(informerConfig.Name)+1:]

	formatSelector, err := formatLabelSelector(informerConfig.LabelSelector)
	if err != nil {
		return nil, fmt.Errorf("failed format label selector '%s'", informerConfig.LabelSelector.String())
	}

	indexers := cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}
	resyncPeriod := time.Duration(2) * time.Hour
	tweakListOptions := func(options *metaV1.ListOptions) {
		if informerConfig.ObjectName != "" {
			options.FieldSelector = fields.OneTermEqualSelector("metadata.name", informerConfig.ObjectName).String()
		}
		if formatSelector != "" {
			options.LabelSelector = formatSelector
		}
	}

	var sharedInformer cache.SharedIndexInformer
	var objects []runtime.Object

	rlog.Debugf("Discover GVR for kind '%s'...", informerConfig.Kind)
	gvr, err := kube.GroupVersionResourceByKind(informerConfig.Kind)
	if err != nil {
		rlog.Errorf("error getting GVR for kind '%s': %v", informerConfig.Kind, err)
		return nil, err
	}
	rlog.Infof("GVR for kind '%s' is %+v", informerConfig.Kind, gvr)
	informer := dynamicinformer.NewFilteredDynamicInformer(kube.DynamicClient, gvr, informerConfig.Namespace, resyncPeriod, indexers, tweakListOptions)
	sharedInformer = informer.Informer()

	// Save already existed resources to IGNORE watch.Added events about them
	selector, err := metaV1.LabelSelectorAsSelector(informerConfig.LabelSelector)
	if err != nil {
		rlog.Infof("error creating labelSelector: %v", err)
		return nil, err
	}
	objects, err = informer.Lister().List(selector)
	if err != nil {
		return nil, fmt.Errorf("failed to list '%s' resources: %v", informerConfig.Kind, err)
	}
	rlog.Debugf("Got %d objects for kind '%s': %+v", len(objects), informerConfig.Kind, objects)
	if len(objects) > 0 {
		err = kubeEventsInformer.InitializeItemsList(objects)
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

func resourceFilter(obj interface{}, jqFilter string) (res string, err error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}

	if jqFilter != "" {
		stdout, stderr, err := execJq(jqFilter, data)
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
	Config KubeEventInformerConfig

	ConfigId string

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

func (ei *KubeEventsInformer) InitializeItemsList(objects []runtime.Object) error {
	for _, obj := range objects {
		resourceId, err := runtimeResourceId(obj, ei.Config.Kind)
		if err != nil {
			return err
		}

		filtered, err := resourceFilter(obj, ei.Config.JqFilter)
		if err != nil {
			return err
		}

		ei.Checksum[resourceId] = utils_checksum.CalculateChecksum(filtered)

		rlog.Debugf("Kube events manager: %+v informer %s: %s object %s initialization: jqFilter '%s': calculated checksum '%s' of object being watched:\n%s",
			ei.Config.EventTypes,
			ei.ConfigId,
			ei.Config.Kind,
			resourceId,
			ei.Config.JqFilter,
			ei.Checksum[resourceId],
			utils_data.FormatJsonDataOrError(utils_data.FormatPrettyJson(filtered)))
	}

	return nil
}

// HandleKubeEvent sends new KubeEvent to KubeEventCh
// obj doesn't contains Kind information, so kind is passed from Run() argument.
// TODO refactor: pass KubeEvent as argument
// TODO add delay to merge Added and Modified events (node added and then labels applied — one hook run on Added+Modifed is enough)
func (ei *KubeEventsInformer) HandleKubeEvent(obj interface{}, objectId string, newChecksum string, eventType OnKubernetesEventType) {
	if ei.Checksum[objectId] != newChecksum {
		ei.Checksum[objectId] = newChecksum

		rlog.Debugf("KUBE_EVENTS %s informer for %+v: handle %s of %s: checksum changed, send KubeEvent",
			ei.ConfigId,
			eventType,
			objectId,
		)
		// Safe to ignore an error because of previous call to runtimeResourceId()
		namespace, name, _ := metaFromEventObject(obj.(runtime.Object))
		KubeEventCh <- KubeEvent{
			ConfigId:  ei.ConfigId,
			Events:    []string{string(eventType)},
			Namespace: namespace,
			Kind:      ei.Config.Kind,
			Name:      name,
		}
	} else {
		rlog.Debugf("KUBE_EVENTS %s informer: handle %s of %s: checksum has not changed",
			ei.ConfigId,
			eventType,
			objectId,
		)
	}

	return
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

func runtimeResourceId(obj interface{}, kind string) (string, error) {
	namespace, name, err := metaFromEventObject(obj)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/%s/%s", namespace, kind, name), nil
}

func (ei *KubeEventsInformer) ShouldHandleEvent(checkEvent OnKubernetesEventType) bool {
	for _, event := range ei.Config.EventTypes {
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

func execJq(jqFilter string, jsonData []byte) (stdout string, stderr string, err error) {
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

	err = executor.Run(cmd)
	stdout = strings.TrimSpace(stdoutBuf.String())
	stderr = strings.TrimSpace(stderrBuf.String())

	return
}
