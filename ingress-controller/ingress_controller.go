package ingresscontroller

import (
	"github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type IngressController interface {
	ConfigureResources(parent pulumi.Resource) (resource pulumi.Resource, err error)
}

func NewNginxIngressController(ctx *pulumi.Context, provider *kubernetes.Provider) NginxIngressController {
	return NginxIngressController{
		ctx:      ctx,
		provider: provider,
	}
}
