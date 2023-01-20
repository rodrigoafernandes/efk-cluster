package ingresscontroller

import (
	"fmt"
	"github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes"

	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/core/v1"
	"github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/helm/v3"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type NginxIngressController struct {
	ctx      *pulumi.Context
	provider *kubernetes.Provider
}

func (n *NginxIngressController) CreateResources(parent pulumi.Resource) (*helm.Release, pulumi.StringOutput, error) {
	namespace, err := corev1.NewNamespace(n.ctx, "nginx-ingress-namespace", &corev1.NamespaceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Labels: pulumi.StringMap{
				"name": pulumi.String("nginx-ingress"),
			},
			Name: pulumi.String("nginx-ingress"),
		},
	}, pulumi.Provider(n.provider), pulumi.DependsOn([]pulumi.Resource{parent}))

	if err != nil {
		return nil, pulumi.String("").ToStringOutput(), err
	}

	rel, err := helm.NewRelease(n.ctx, "nginx-ingress", &helm.ReleaseArgs{
		Name:      pulumi.String("ingress-nginx"),
		Namespace: namespace.Metadata.Name(),
		Chart:     pulumi.String("ingress-nginx"),
		RepositoryOpts: helm.RepositoryOptsArgs{
			Repo: pulumi.String("https://kubernetes.github.io/ingress-nginx"),
		},
		Timeout: pulumi.Int(120),
	}, pulumi.Provider(n.provider), pulumi.Parent(namespace))

	if err != nil {
		return nil, pulumi.String("").ToStringOutput(), err
	}

	hostName := pulumi.All(rel.Status.Namespace(), rel.Status.Name()).
		ApplyT(func(r interface{}) (pulumi.StringOutput, error) {
			arr := r.([]interface{})
			ns := arr[0].(*string)
			name := arr[1].(*string)
			svc, err := corev1.GetService(n.ctx,
				"ingress-nginx-controller-svc",
				pulumi.ID(fmt.Sprintf("%s/%s-controller", *ns, *name)),
				nil,
				pulumi.Provider(n.provider), pulumi.Parent(namespace))
			if err != nil {
				return pulumi.String("").ToStringOutput(), err
			}
			return svc.Status.LoadBalancer().Ingress().Index(pulumi.Int(0)).Hostname().Elem().ToStringOutput(), nil
		}).(pulumi.StringOutput)

	return rel, hostName, err
}
