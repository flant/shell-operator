package dedupclient

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/ldmonster/kubeclient"
	"github.com/ldmonster/kubeclient/store"
	apixv1client "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/flant/shell-operator/pkg"
)

// ErrAlreadyStarted is returned by Start when the cache loop is already
// running for this Client instance. It is a non-fatal sentinel intended to
// make idempotent Start() calls safe.
var ErrAlreadyStarted = errors.New("dedupclient: cache already started")

// Client is shell-operator's wrapper around *kubeclient.DedupClient.
//
// It owns the cache lifecycle: a single goroutine invokes the underlying
// kubeclient.Start (which blocks until ctx is cancelled), exposes
// WaitForCacheSync, and provides a Shutdown that signals the run loop and
// waits for it to exit. The wrapper deliberately keeps the public surface
// small — callers reach through to the embedded controller-runtime
// client.Client when they need full read/write access.
type Client struct {
	client.Client
	kubernetes.Interface

	dedup  *kubeclient.DedupClient
	logger *log.Logger

	restConfig      *rest.Config
	cachedDiscovery discovery.CachedDiscoveryInterface
	restMapper      meta.RESTMapper
	contextName     string
	configPath      string
	defaultNS       string
	dynamicClient   dynamic.Interface
	apiExtClient    apixv1client.ApiextensionsV1Interface
	metadataClient  metadata.Interface

	informerStoresMu sync.Mutex
	informerStores   map[string]store.Store

	// startOnce guards Start() so the cache loop is only spawned once even
	// if Start is called concurrently from several places.
	startOnce sync.Once
	started   atomic.Bool

	// cancel cancels the context handed to the underlying cache's Start
	// method, terminating the run loop.
	cancel context.CancelFunc

	// done is closed when the cache run loop returns.
	done chan struct{}

	// startErr captures the error (if any) returned by the underlying
	// kubeclient.DedupClient.Start. It is set at most once.
	startErr atomic.Value // holds error
}

// New constructs a Client from cfg. RESTConfig is required; all other fields
// are optional. The returned Client is not started — call Start to spin up
// the cache informers.
func New(cfg Config, logger *log.Logger) (*Client, error) {
	if logger == nil {
		logger = log.NewLogger()
	}
	logger = logger.With(pkg.LogKeyOperatorComponent, "dedup-kube-client")

	restCfg, defaultNS, err := buildRESTConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("dedupclient: construct rest config: %w", err)
	}
	cfg.RESTConfig = restCfg

	kube, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("dedupclient: construct typed client: %w", err)
	}
	dyn, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("dedupclient: construct dynamic client: %w", err)
	}
	apiExt, err := apixv1client.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("dedupclient: construct apiextensions client: %w", err)
	}
	metaClient, err := metadata.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("dedupclient: construct metadata client: %w", err)
	}
	cachedDiscovery, err := newCachedDiscovery(restCfg)
	if err != nil {
		return nil, fmt.Errorf("dedupclient: construct discovery client: %w", err)
	}
	if cfg.RESTMapper == nil {
		cfg.RESTMapper = newRESTMapper(cachedDiscovery, logger)
	}

	opts := buildOptions(cfg)

	dc, err := kubeclient.New(restCfg, opts...)
	if err != nil {
		return nil, fmt.Errorf("dedupclient: construct kubeclient: %w", err)
	}

	return &Client{
		Client:          dc,
		Interface:       kube,
		dedup:           dc,
		logger:          logger,
		restConfig:      restCfg,
		cachedDiscovery: cachedDiscovery,
		restMapper:      cfg.RESTMapper,
		contextName:     cfg.Context,
		configPath:      cfg.Config,
		defaultNS:       defaultNS,
		dynamicClient:   dyn,
		apiExtClient:    apiExt,
		metadataClient:  metaClient,
		informerStores:  make(map[string]store.Store),
		done:            make(chan struct{}),
	}, nil
}

// buildOptions translates Config into kubeclient.Option values, omitting
// any option whose corresponding Config field is at its zero value so the
// upstream defaults remain in effect.
func buildOptions(cfg Config) []kubeclient.Option {
	var opts []kubeclient.Option
	if cfg.Scheme != nil {
		opts = append(opts, kubeclient.WithScheme(cfg.Scheme))
	}
	if cfg.RESTMapper != nil {
		opts = append(opts, kubeclient.WithRESTMapper(cfg.RESTMapper))
	}
	if len(cfg.Namespaces) > 0 {
		opts = append(opts, kubeclient.WithNamespaces(cfg.Namespaces...))
	}
	if len(cfg.WatchGVKs) > 0 {
		opts = append(opts, kubeclient.WithGVKs(cfg.WatchGVKs...))
	}
	if cfg.ReconstructLRUSize > 0 {
		opts = append(opts, kubeclient.WithReconstructionCache(cfg.ReconstructLRUSize))
	}
	if cfg.GCInterval > 0 {
		opts = append(opts, kubeclient.WithGCInterval(cfg.GCInterval))
	}
	return opts
}

// Dedup returns the underlying *kubeclient.DedupClient for callers that
// need direct access to its full API (Cache, Scheme, RESTMapper, etc.).
// Most callers should use the embedded client.Client surface instead.
func (c *Client) Dedup() *kubeclient.DedupClient {
	return c.dedup
}

func (c *Client) Dynamic() dynamic.Interface {
	return c.dynamicClient
}

func (c *Client) ApiExt() apixv1client.ApiextensionsV1Interface {
	return c.apiExtClient
}

func (c *Client) Metadata() metadata.Interface {
	return c.metadataClient
}

func (c *Client) RestConfig() *rest.Config {
	return c.restConfig
}

func (c *Client) DefaultNamespace() string {
	if c.defaultNS == "" {
		return "default"
	}
	return c.defaultNS
}

func (c *Client) ToRESTConfig() (*rest.Config, error) {
	return c.restConfig, nil
}

func (c *Client) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	return c.cachedDiscovery, nil
}

func (c *Client) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	return newClientConfig(c.contextName, c.configPath)
}

func (c *Client) ToRESTMapper() (meta.RESTMapper, error) {
	return c.restMapper, nil
}

func (c *Client) RESTMapper() meta.RESTMapper {
	return c.restMapper
}

func (c *Client) NewBuilder() *resource.Builder {
	return resource.NewBuilder(c)
}

func (c *Client) InvalidateDiscoveryCache() {
	if c.cachedDiscovery != nil {
		c.cachedDiscovery.Invalidate()
	}
}

func (c *Client) APIResourceList(apiVersion string) ([]*metav1.APIResourceList, error) {
	lists, err := c.apiResourceList(apiVersion)
	if err != nil && strings.Contains(err.Error(), "not found") && c.cachedDiscovery != nil {
		c.cachedDiscovery.Invalidate()
		return c.apiResourceList(apiVersion)
	}
	return lists, err
}

func (c *Client) APIResource(apiVersion, kind string) (*metav1.APIResource, error) {
	resource, err := c.apiResource(apiVersion, kind)
	if err != nil && apierrors.IsNotFound(err) && c.cachedDiscovery != nil {
		c.cachedDiscovery.Invalidate()
		resource, err = c.apiResource(apiVersion, kind)
	}
	if err != nil {
		return nil, fmt.Errorf("apiVersion %q, kind %q is not supported by cluster: %w", apiVersion, kind, err)
	}
	return resource, nil
}

func (c *Client) GroupVersionResource(apiVersion, kind string) (schema.GroupVersionResource, error) {
	apiRes, err := c.APIResource(apiVersion, kind)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}
	return schema.GroupVersionResource{
		Group:    apiRes.Group,
		Version:  apiRes.Version,
		Resource: apiRes.Name,
	}, nil
}

func (c *Client) InformerStore(key string) store.Store {
	c.informerStoresMu.Lock()
	defer c.informerStoresMu.Unlock()
	if c.informerStores == nil {
		c.informerStores = make(map[string]store.Store)
	}
	if s, ok := c.informerStores[key]; ok {
		return s
	}
	s := store.NewDedupStore()
	c.informerStores[key] = s
	return s
}

// Start launches the cache run loop in a dedicated goroutine. Subsequent
// calls return ErrAlreadyStarted without spawning additional goroutines.
//
// The supplied parent context governs the cache's lifetime: when it is
// cancelled (or Shutdown is called) the underlying kubeclient.Start
// returns and the goroutine exits. Start itself is non-blocking and
// returns as soon as the goroutine has been scheduled.
func (c *Client) Start(parent context.Context) error {
	if c.dedup == nil {
		return nil
	}
	if parent == nil {
		parent = context.Background()
	}

	already := true
	c.startOnce.Do(func() {
		already = false

		ctx, cancel := context.WithCancel(parent)
		c.cancel = cancel
		c.started.Store(true)

		go func() {
			defer close(c.done)
			c.logger.Info("dedup cache run loop starting")
			err := c.dedup.Start(ctx)
			if err != nil && !errors.Is(err, context.Canceled) {
				c.startErr.Store(err)
				c.logger.Error("dedup cache run loop exited with error", log.Err(err))
				return
			}
			c.logger.Info("dedup cache run loop exited")
		}()
	})

	if already {
		return ErrAlreadyStarted
	}
	return nil
}

// WaitForCacheSync blocks until every registered informer has performed an
// initial List or until ctx is cancelled. It returns true on success and
// false when the wait was aborted (or no Start has happened yet).
func (c *Client) WaitForCacheSync(ctx context.Context) bool {
	if c.dedup == nil || !c.started.Load() {
		return false
	}
	return c.dedup.WaitForCacheSync(ctx)
}

// EnsureInformer registers obj's GVK with the cache and starts an informer
// for it if one is not already running. It is a convenience pass-through
// to the underlying cache so callers do not have to drill through Dedup().
func (c *Client) EnsureInformer(ctx context.Context, obj client.Object) error {
	if c.dedup == nil {
		return fmt.Errorf("dedupclient: client is not initialised")
	}
	return c.dedup.Cache().EnsureInformer(ctx, obj)
}

// Shutdown signals the run loop to terminate and blocks until it has
// returned (or ctx is cancelled). Calling Shutdown before Start, or after
// a previous Shutdown has already returned, is a safe no-op.
func (c *Client) Shutdown(ctx context.Context) error {
	if c.dedup == nil || !c.started.Load() {
		return nil
	}
	if c.cancel != nil {
		c.cancel()
	}
	select {
	case <-c.done:
		c.logger.Debug("dedup cache shutdown complete")
		if v := c.startErr.Load(); v != nil {
			if err, ok := v.(error); ok {
				return err
			}
		}
		return nil
	case <-ctx.Done():
		c.logger.Warn("dedup cache shutdown timed out", slog.String("error", ctx.Err().Error()))
		return ctx.Err()
	}
}

// Scheme returns the runtime.Scheme used by the underlying client. It is
// kept here so callers don't have to import the kubeclient package just
// to read it back.
func (c *Client) Scheme() *runtime.Scheme {
	if c.dedup == nil {
		return runtime.NewScheme()
	}
	return c.dedup.Scheme()
}
