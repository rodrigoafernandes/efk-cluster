package kibanalogging

import (
	"github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/core/v1"
	"github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/helm/v3"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/meta/v1"
	networkingv1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/networking/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Kibana interface {
	CreateResources(namespace *corev1.Namespace, elasticSeearch *helm.Release, hostname pulumi.StringOutput) (err error)
}

type resource struct {
	ctx      *pulumi.Context
	provider *kubernetes.Provider
}

func NewKibana(context *pulumi.Context, provider *kubernetes.Provider) Kibana {
	return resource{
		ctx:      context,
		provider: provider,
	}
}

func (k resource) CreateResources(namespace *corev1.Namespace, elasticSearch *helm.Release, hostname pulumi.StringOutput) (err error) {
	rel, err := helm.NewRelease(k.ctx, "kibana", &helm.ReleaseArgs{
		Name:      pulumi.String("kibana"),
		Namespace: namespace.Metadata.Name(),
		Chart:     pulumi.String("kibana"),
		Version:   pulumi.String("10.2.9"),
		RepositoryOpts: helm.RepositoryOptsArgs{
			Repo: pulumi.String("https://charts.bitnami.com/bitnami"),
		},
		Values: pulumi.Map{
			"elasticsearch": pulumi.Map{
				"hosts": pulumi.StringArray{
					pulumi.String("elasticsearch.efk-logging.svc.cluster.local"),
				},
				"port": pulumi.String("9200"),
			},
		},
		Timeout: pulumi.Int(300),
	}, pulumi.Provider(k.provider), pulumi.Parent(namespace), pulumi.DependsOn([]pulumi.Resource{elasticSearch}))
	if err != nil {
		return err
	}
	svc, err := corev1.GetService(
		k.ctx,
		"kibana",
		pulumi.ID("efk-logging/kibana"),
		nil,
		pulumi.Provider(k.provider),
		pulumi.Parent(namespace),
		pulumi.DependsOn([]pulumi.Resource{rel}),
	)
	_, err = networkingv1.NewIngress(k.ctx, "kibana-ingress", &networkingv1.IngressArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("kibana-ingress"),
			Namespace: namespace.Metadata.Name(),
			Annotations: pulumi.StringMap{
				"kubernetes.io/ingress.class":           pulumi.String("nginx"),
				"nginx.ingress.kubernetes.io/use-regex": pulumi.String("true"),
			},
		},
		Spec: networkingv1.IngressSpecArgs{
			Rules: networkingv1.IngressRuleArray{
				networkingv1.IngressRuleArgs{
					Host: hostname,
					Http: networkingv1.HTTPIngressRuleValueArgs{
						Paths: networkingv1.HTTPIngressPathArray{
							networkingv1.HTTPIngressPathArgs{
								Path:     pulumi.String("/*"),
								PathType: pulumi.String("Prefix"),
								Backend: networkingv1.IngressBackendArgs{
									Service: networkingv1.IngressServiceBackendArgs{
										Name: svc.Metadata.Name().Elem().ToStringOutput(),
										Port: networkingv1.ServiceBackendPortArgs{
											Number: pulumi.Int(5601),
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}, pulumi.Provider(k.provider), pulumi.Parent(namespace), pulumi.DependsOn([]pulumi.Resource{rel, svc}))

	return
}
