package elasticsearchlogging

import (
	"github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/core/v1"
	"github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/helm/v3"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

type Elasticsearch interface {
	CreateResources(parent pulumi.Resource) (*corev1.Namespace, *helm.Release, error)
}

type resource struct {
	ctx      *pulumi.Context
	provider *kubernetes.Provider
	cfg      *config.Config
}

func (e resource) CreateResources(parent pulumi.Resource) (*corev1.Namespace, *helm.Release, error) {
	namespace, err := corev1.NewNamespace(e.ctx, "efk-namespace", &corev1.NamespaceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Labels: pulumi.StringMap{
				"name": pulumi.String("efk-logging"),
			},
			Name: pulumi.String("efk-logging"),
		},
	}, pulumi.Provider(e.provider), pulumi.DependsOn([]pulumi.Resource{parent}))
	if err != nil {
		return nil, nil, err
	}
	release, err := helm.NewRelease(e.ctx, "elasticsearch", &helm.ReleaseArgs{
		Name:      pulumi.String("elasticsearch"),
		Namespace: namespace.Metadata.Name(),
		Chart:     pulumi.String("elasticsearch"),
		Version:   pulumi.String("19.5.4"),
		RepositoryOpts: helm.RepositoryOptsArgs{
			Repo: pulumi.String("https://charts.bitnami.com/bitnami"),
		},
		Values: pulumi.Map{
			"global": pulumi.Map{
				"storageClass": pulumi.String("linode-block-storage"),
			},
			"security": pulumi.Map{
				"elasticPassword": e.cfg.GetSecret("elasticsearch_pwd"),
			},
		},
		Timeout: pulumi.Int(600),
	}, pulumi.Provider(e.provider), pulumi.Parent(namespace))
	return namespace, release, err
}

func NewElasticsearch(ctx *pulumi.Context, provider *kubernetes.Provider, cfg *config.Config) Elasticsearch {
	return resource{
		ctx:      ctx,
		provider: provider,
		cfg:      cfg,
	}
}
