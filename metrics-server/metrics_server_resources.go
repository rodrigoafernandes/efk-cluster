package metricsserver

import (
	"github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes"
	"github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/yaml"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type MetricsServerResource interface {
	CreateResources() (pulumi.Resource, error)
}

type resource struct {
	ctx      *pulumi.Context
	provider *kubernetes.Provider
}

func NewMetricsServerResource(context *pulumi.Context, provider *kubernetes.Provider) MetricsServerResource {
	return resource{
		ctx:      context,
		provider: provider,
	}
}

func (m resource) CreateResources() (pulumi.Resource, error) {
	resources, err := yaml.NewConfigFile(m.ctx, "metrics-server", &yaml.ConfigFileArgs{
		File: "metrics-server/metrics-server.yaml",
	}, pulumi.Provider(m.provider))

	return resources, err
}
