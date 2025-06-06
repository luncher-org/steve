package client

import (
	"fmt"
	"net/http"
	"time"

	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/steve/pkg/attributes"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/rest"
)

const (
	// defaultQPS and defaultBurst are used to configure the rest.Config for
	// factory created clients.
	defaultQPS   float32 = 10000
	defaultBurst int     = 100
)

type Factory struct {
	impersonate         bool
	tableClientCfg      *rest.Config
	tableWatchClientCfg *rest.Config
	clientCfg           *rest.Config
	watchClientCfg      *rest.Config
	metadata            metadata.Interface
	dynamic             dynamic.Interface
	Config              *rest.Config
}

type addQuery struct {
	values map[string]string
	next   http.RoundTripper
}

var _ utilnet.RoundTripperWrapper = (*addQuery)(nil)

func (a *addQuery) RoundTrip(req *http.Request) (*http.Response, error) {
	q := req.URL.Query()
	for k, v := range a.values {
		q.Set(k, v)
	}
	req.Header.Set("Accept", "application/json;as=Table;v=v1;g=meta.k8s.io,application/json;as=Table;v=v1beta1;g=meta.k8s.io")
	req.URL.RawQuery = q.Encode()
	return a.next.RoundTrip(req)
}

func (a *addQuery) WrappedRoundTripper() http.RoundTripper {
	return a.next
}

type factoryOptions struct {
	qps   float32
	burst int
}

func defaultFactoryOptions() *factoryOptions {
	return &factoryOptions{
		qps:   defaultQPS,
		burst: defaultBurst,
	}
}

// WithQPSAndBurst configures the rest.Config used for creating the clients in
// the factory with the provided burst and qps configuration.
//
// See https://pkg.go.dev/k8s.io/client-go/rest#Config for more.
func WithQPSAndBurst(qps float32, burst int) FactoryOption {
	return func(opts *factoryOptions) {
		opts.qps = qps
		opts.burst = burst
	}
}

// FactoryOption is an option-func for configuring the newly created factory.
type FactoryOption func(*factoryOptions)

func NewFactory(cfg *rest.Config, impersonate bool, opts ...FactoryOption) (*Factory, error) {
	clientCfg := rest.CopyConfig(cfg)
	options := defaultFactoryOptions()
	for _, opt := range opts {
		opt(options)
	}

	clientCfg.QPS = options.qps
	clientCfg.Burst = options.burst

	watchClientCfg := rest.CopyConfig(clientCfg)
	watchClientCfg.Timeout = 30 * time.Minute

	setTable := func(rt http.RoundTripper) http.RoundTripper {
		return &addQuery{
			values: map[string]string{
				"includeObject": "Object",
			},
			next: rt,
		}
	}

	tableClientCfg := rest.CopyConfig(clientCfg)
	tableClientCfg.Wrap(setTable)
	tableClientCfg.AcceptContentTypes = "application/json;as=Table;v=v1;g=meta.k8s.io,application/json;as=Table;v=v1beta1;g=meta.k8s.io"
	tableWatchClientCfg := rest.CopyConfig(watchClientCfg)
	tableWatchClientCfg.Wrap(setTable)
	tableWatchClientCfg.AcceptContentTypes = "application/json;as=Table;v=v1;g=meta.k8s.io,application/json;as=Table;v=v1beta1;g=meta.k8s.io"

	md, err := metadata.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	d, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	return &Factory{
		dynamic:             d,
		metadata:            md,
		impersonate:         impersonate,
		tableClientCfg:      tableClientCfg,
		tableWatchClientCfg: tableWatchClientCfg,
		clientCfg:           clientCfg,
		watchClientCfg:      watchClientCfg,
		Config:              watchClientCfg,
	}, nil
}

func (p *Factory) MetadataClient() metadata.Interface {
	return p.metadata
}

func (p *Factory) AdminDynamicClient() dynamic.Interface {
	return p.dynamic
}

func (p *Factory) IsImpersonating() bool {
	return p.impersonate
}

func (p *Factory) K8sInterface(ctx *types.APIRequest) (kubernetes.Interface, error) {
	cfg, err := setupConfig(ctx, p.clientCfg, p.impersonate)
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(cfg)
}

func (p *Factory) AdminK8sInterface() (kubernetes.Interface, error) {
	return kubernetes.NewForConfig(p.clientCfg)
}

func (p *Factory) DynamicClient(ctx *types.APIRequest, warningHandler rest.WarningHandler) (dynamic.Interface, error) {
	return newDynamicClient(ctx, p.clientCfg, p.impersonate, warningHandler)
}

func (p *Factory) Client(ctx *types.APIRequest, s *types.APISchema, namespace string, warningHandler rest.WarningHandler) (dynamic.ResourceInterface, error) {
	return newClient(ctx, p.clientCfg, s, namespace, p.impersonate, warningHandler)
}

func (p *Factory) AdminClient(ctx *types.APIRequest, s *types.APISchema, namespace string, warningHandler rest.WarningHandler) (dynamic.ResourceInterface, error) {
	return newClient(ctx, p.clientCfg, s, namespace, false, warningHandler)
}

func (p *Factory) ClientForWatch(ctx *types.APIRequest, s *types.APISchema, namespace string, warningHandler rest.WarningHandler) (dynamic.ResourceInterface, error) {
	return newClient(ctx, p.watchClientCfg, s, namespace, p.impersonate, warningHandler)
}

func (p *Factory) AdminClientForWatch(ctx *types.APIRequest, s *types.APISchema, namespace string, warningHandler rest.WarningHandler) (dynamic.ResourceInterface, error) {
	return newClient(ctx, p.watchClientCfg, s, namespace, false, warningHandler)
}

func (p *Factory) TableClient(ctx *types.APIRequest, s *types.APISchema, namespace string, warningHandler rest.WarningHandler) (dynamic.ResourceInterface, error) {
	if attributes.Table(s) {
		return newClient(ctx, p.tableClientCfg, s, namespace, p.impersonate, warningHandler)
	}
	return p.Client(ctx, s, namespace, warningHandler)
}

func (p *Factory) TableAdminClient(ctx *types.APIRequest, s *types.APISchema, namespace string, warningHandler rest.WarningHandler) (dynamic.ResourceInterface, error) {
	if attributes.Table(s) {
		return newClient(ctx, p.tableClientCfg, s, namespace, false, warningHandler)
	}
	return p.AdminClient(ctx, s, namespace, warningHandler)
}

func (p *Factory) TableClientForWatch(ctx *types.APIRequest, s *types.APISchema, namespace string, warningHandler rest.WarningHandler) (dynamic.ResourceInterface, error) {
	if attributes.Table(s) {
		return newClient(ctx, p.tableWatchClientCfg, s, namespace, p.impersonate, warningHandler)
	}
	return p.ClientForWatch(ctx, s, namespace, warningHandler)
}

func (p *Factory) TableAdminClientForWatch(ctx *types.APIRequest, s *types.APISchema, namespace string, warningHandler rest.WarningHandler) (dynamic.ResourceInterface, error) {
	if attributes.Table(s) {
		return newClient(ctx, p.tableWatchClientCfg, s, namespace, false, warningHandler)
	}
	return p.AdminClientForWatch(ctx, s, namespace, warningHandler)
}

func setupConfig(ctx *types.APIRequest, cfg *rest.Config, impersonate bool) (*rest.Config, error) {
	if impersonate {
		user, ok := request.UserFrom(ctx.Context())
		if !ok {
			return nil, fmt.Errorf("user not found for impersonation")
		}
		cfg = rest.CopyConfig(cfg)
		cfg.Impersonate.UserName = user.GetName()
		cfg.Impersonate.Groups = user.GetGroups()
		cfg.Impersonate.Extra = user.GetExtra()
	}
	return cfg, nil
}

func newDynamicClient(ctx *types.APIRequest, cfg *rest.Config, impersonate bool, warningHandler rest.WarningHandler) (dynamic.Interface, error) {
	cfg, err := setupConfig(ctx, cfg, impersonate)
	cfg.WarningHandler = warningHandler
	if err != nil {
		return nil, err
	}

	return dynamic.NewForConfig(cfg)
}

func newClient(ctx *types.APIRequest, cfg *rest.Config, s *types.APISchema, namespace string, impersonate bool, warningHandler rest.WarningHandler) (dynamic.ResourceInterface, error) {
	client, err := newDynamicClient(ctx, cfg, impersonate, warningHandler)
	if err != nil {
		return nil, err
	}

	gvr := attributes.GVR(s)
	return client.Resource(gvr).Namespace(namespace), nil
}
