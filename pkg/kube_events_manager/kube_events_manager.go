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

	"k8s.io/apimachinery/pkg/api/meta"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	appsV1 "k8s.io/client-go/informers/apps/v1"
	batchV1 "k8s.io/client-go/informers/batch/v1"
	batchV2Alpha1 "k8s.io/client-go/informers/batch/v2alpha1"
	coreV1 "k8s.io/client-go/informers/core/v1"
	extensionsV1Beta1 "k8s.io/client-go/informers/extensions/v1beta1"
	storageV1 "k8s.io/client-go/informers/storage/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/flant/shell-operator/pkg/kube"
	"github.com/flant/shell-operator/pkg/executor"
	utils_checksum "github.com/flant/shell-operator/pkg/utils/checksum"
	utils_data "github.com/flant/shell-operator/pkg/utils/data"
)

type KubeEventsManager interface {
	Run(eventTypes []OnKubernetesEventType, kind, namespace string, labelSelector *metaV1.LabelSelector, jqFilter string, debug bool) (string, error)
	Stop(configId string) error
}

type MainKubeEventsManager struct {
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
					rlog.Error("Kube events manager: %+v informer %s: %s object %s: %s", eventTypes, kubeEventsInformer.ConfigId, kind, objectId, err)
					return
				}

				checksum := utils_checksum.CalculateChecksum(filtered)

				if debug {
					rlog.Debugf("Kube events manager: %+v informer %s: add %s object %s: jqFilter '%s': calculated checksum '%s' of object being watched:\n%s",
						eventTypes, kubeEventsInformer.ConfigId, kind, objectId, jqFilter, checksum, utils_data.FormatJsonDataOrError(utils_data.FormatPrettyJson(filtered)))
				}

				err = kubeEventsInformer.HandleKubeEvent(obj, kind, checksum, "ADDED", kubeEventsInformer.ShouldHandleEvent(KubernetesEventOnAdd), debug)
				if err != nil {
					rlog.Error("Kube events manager: %+v informer %s: %s object %s: %s", eventTypes, kubeEventsInformer.ConfigId, kind, objectId, err)
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
					rlog.Error("Kube events manager: %+v informer %s: %s object %s: %s", eventTypes, kubeEventsInformer.ConfigId, kind, objectId, err)
					return
				}

				checksum := utils_checksum.CalculateChecksum(filtered)

				if debug {
					rlog.Debugf("Kube events manager: %+v informer %s: update %s object %s: jqFilter '%s': calculated checksum '%s' of object being watched:\n%s",
						eventTypes, kubeEventsInformer.ConfigId, kind, objectId, jqFilter, checksum, utils_data.FormatJsonDataOrError(utils_data.FormatPrettyJson(filtered)))
				}

				err = kubeEventsInformer.HandleKubeEvent(obj, kind, checksum, "MODIFIED", kubeEventsInformer.ShouldHandleEvent(KubernetesEventOnUpdate), debug)
				if err != nil {
					rlog.Error("Kube events manager: %+v informer %s: %s object %s: %s", eventTypes, kubeEventsInformer.ConfigId, kind, objectId, err)
					return
				}
			},
			DeleteFunc: func(obj interface{}) {
				objectId, err := runtimeResourceId(obj)
				if err != nil {
					rlog.Errorf("failed to get object id: %s", err)
					return
				}

				if debug {
					rlog.Debugf("Kube events manager: %+v informer %s: delete %s object %s", eventTypes, kubeEventsInformer.ConfigId, kind, objectId)
				}

				err = kubeEventsInformer.HandleKubeEvent(obj, kind, "", "DELETED", kubeEventsInformer.ShouldHandleEvent(KubernetesEventOnDelete), debug)
				if err != nil {
					rlog.Error("Kube events manager: %+v informer %s: %s object %s: %s", eventTypes, kubeEventsInformer.ConfigId, kind, objectId, err)
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

	listOptions := metaV1.ListOptions{}
	if formatSelector != "" {
		listOptions.LabelSelector = formatSelector
	}

	var sharedInformer cache.SharedIndexInformer
	var resourceList runtime.Object
	var listErr error

	switch strings.ToLower(kind) {
	case "namespace":
		sharedInformer = coreV1.NewFilteredNamespaceInformer(kube.Kubernetes, resyncPeriod, indexers, tweakListOptions)
		resourceList, listErr = kube.Kubernetes.CoreV1().Namespaces().List(listOptions)

	case "cronjob":
		sharedInformer = batchV2Alpha1.NewFilteredCronJobInformer(kube.Kubernetes, namespace, resyncPeriod, indexers, tweakListOptions)
		resourceList, listErr = kube.Kubernetes.BatchV2alpha1().CronJobs(namespace).List(listOptions)

	case "daemonset":
		sharedInformer = appsV1.NewFilteredDaemonSetInformer(kube.Kubernetes, namespace, resyncPeriod, indexers, tweakListOptions)
		resourceList, listErr = kube.Kubernetes.AppsV1().DaemonSets(namespace).List(listOptions)

	case "deployment":
		sharedInformer = appsV1.NewFilteredDeploymentInformer(kube.Kubernetes, namespace, resyncPeriod, indexers, tweakListOptions)
		resourceList, listErr = kube.Kubernetes.AppsV1().Deployments(namespace).List(listOptions)

	case "job":
		sharedInformer = batchV1.NewFilteredJobInformer(kube.Kubernetes, namespace, resyncPeriod, indexers, tweakListOptions)
		resourceList, listErr = kube.Kubernetes.BatchV1().Jobs(namespace).List(listOptions)

	case "pod":
		sharedInformer = coreV1.NewFilteredPodInformer(kube.Kubernetes, namespace, resyncPeriod, indexers, tweakListOptions)
		resourceList, listErr = kube.Kubernetes.CoreV1().Pods(namespace).List(listOptions)

	case "replicaset":
		sharedInformer = appsV1.NewFilteredReplicaSetInformer(kube.Kubernetes, namespace, resyncPeriod, indexers, tweakListOptions)
		resourceList, listErr = kube.Kubernetes.AppsV1().ReplicaSets(namespace).List(listOptions)

	case "replicationcontroller":
		sharedInformer = coreV1.NewFilteredReplicationControllerInformer(kube.Kubernetes, namespace, resyncPeriod, indexers, tweakListOptions)
		resourceList, listErr = kube.Kubernetes.CoreV1().ReplicationControllers(namespace).List(listOptions)

	case "statefulset":
		sharedInformer = appsV1.NewFilteredStatefulSetInformer(kube.Kubernetes, namespace, resyncPeriod, indexers, tweakListOptions)
		resourceList, listErr = kube.Kubernetes.AppsV1().StatefulSets(namespace).List(listOptions)

	case "endpoints":
		sharedInformer = coreV1.NewFilteredEndpointsInformer(kube.Kubernetes, namespace, resyncPeriod, indexers, tweakListOptions)
		resourceList, listErr = kube.Kubernetes.CoreV1().Endpoints(namespace).List(listOptions)

	case "ingress":
		sharedInformer = extensionsV1Beta1.NewFilteredIngressInformer(kube.Kubernetes, namespace, resyncPeriod, indexers, tweakListOptions)
		resourceList, listErr = kube.Kubernetes.ExtensionsV1beta1().Ingresses(namespace).List(listOptions)

	case "service":
		sharedInformer = coreV1.NewFilteredServiceInformer(kube.Kubernetes, namespace, resyncPeriod, indexers, tweakListOptions)
		resourceList, listErr = kube.Kubernetes.CoreV1().Services(namespace).List(listOptions)

	case "configmap":
		sharedInformer = coreV1.NewFilteredConfigMapInformer(kube.Kubernetes, namespace, resyncPeriod, indexers, tweakListOptions)
		resourceList, listErr = kube.Kubernetes.CoreV1().ConfigMaps(namespace).List(listOptions)

	case "secret":
		sharedInformer = coreV1.NewFilteredSecretInformer(kube.Kubernetes, namespace, resyncPeriod, indexers, tweakListOptions)
		resourceList, listErr = kube.Kubernetes.CoreV1().Secrets(namespace).List(listOptions)

	case "persistentvolumeclaim":
		sharedInformer = coreV1.NewFilteredPersistentVolumeClaimInformer(kube.Kubernetes, namespace, resyncPeriod, indexers, tweakListOptions)
		resourceList, listErr = kube.Kubernetes.CoreV1().PersistentVolumeClaims(namespace).List(listOptions)

	case "storageclass":
		sharedInformer = storageV1.NewFilteredStorageClassInformer(kube.Kubernetes, resyncPeriod, indexers, tweakListOptions)
		resourceList, listErr = kube.Kubernetes.StorageV1().StorageClasses().List(listOptions)

	case "node":
		sharedInformer = coreV1.NewFilteredNodeInformer(kube.Kubernetes, resyncPeriod, indexers, tweakListOptions)
		resourceList, listErr = kube.Kubernetes.CoreV1().Nodes().List(listOptions)

	case "serviceaccount":
		sharedInformer = coreV1.NewFilteredServiceAccountInformer(kube.Kubernetes, namespace, resyncPeriod, indexers, tweakListOptions)
		resourceList, err = kube.Kubernetes.CoreV1().ServiceAccounts(namespace).List(listOptions)

	default:
		return nil, fmt.Errorf("kind '%s' isn't supported", kind)
	}

	if listErr != nil {
		return nil, fmt.Errorf("failed to list '%s' resources: %v", kind, err)
	}

	// Save already existed resources to IGNORE watch.Added events about them
	err = kubeEventsInformer.InitializeItemsList(resourceList, debug)
	if err != nil {
		return nil, err
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

func (ei *KubeEventsInformer) InitializeItemsList(list runtime.Object, debug bool) error {
	objects, err := meta.ExtractList(list)
	if err != nil {
		rlog.Errorf("InitializeItemsList got invalid List of type %T from API: %v", list, err)
	}

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
