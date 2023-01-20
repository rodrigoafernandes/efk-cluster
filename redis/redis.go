package redis

import (
	"github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes"
	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Redis interface {
	CreateResources(parents ...pulumi.Resource) (*corev1.Namespace, *corev1.Service, error)
}

type resource struct {
	ctx      *pulumi.Context
	provider *kubernetes.Provider
}

func NewRedis(context *pulumi.Context, provider *kubernetes.Provider) Redis {
	return resource{
		ctx:      context,
		provider: provider,
	}
}

func (r resource) CreateResources(parents ...pulumi.Resource) (namespace *corev1.Namespace, redisService *corev1.Service, err error) {
	namespace, err = corev1.NewNamespace(r.ctx, "databases-namespace", &corev1.NamespaceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Labels: pulumi.StringMap{
				"name": pulumi.String("databases"),
			},
			Name: pulumi.String("databases"),
		},
	}, pulumi.Provider(r.provider), pulumi.DependsOn(parents))

	if err != nil {
		return nil, nil, err
	}

	redisAppLabels := pulumi.StringMap{
		"app": pulumi.String("redis"),
	}

	deployment, err := appsv1.NewDeployment(r.ctx, "redis", &appsv1.DeploymentArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("redis"),
			Labels:    redisAppLabels,
			Namespace: namespace.Metadata.Name(),
		},
		Spec: appsv1.DeploymentSpecArgs{
			Selector: &metav1.LabelSelectorArgs{
				MatchLabels: redisAppLabels,
			},
			Replicas: pulumi.Int(1),
			Template: &corev1.PodTemplateSpecArgs{
				Metadata: &metav1.ObjectMetaArgs{
					Labels: redisAppLabels,
				},
				Spec: &corev1.PodSpecArgs{
					Containers: corev1.ContainerArray{
						corev1.ContainerArgs{
							Name:            pulumi.String("redis"),
							Image:           pulumi.String("docker.io/redis:7.0.4-alpine3.16"),
							ImagePullPolicy: pulumi.String("IfNotPresent"),
							Ports: corev1.ContainerPortArray{
								&corev1.ContainerPortArgs{
									ContainerPort: pulumi.Int(6379),
									Protocol:      pulumi.String("TCP"),
								},
							},
						},
					},
				},
			},
		},
	}, pulumi.Provider(r.provider), pulumi.Parent(namespace))

	if err != nil {
		return nil, nil, err
	}

	redisService, err = corev1.NewService(r.ctx, "redis", &corev1.ServiceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("redis"),
			Namespace: namespace.Metadata.Name(),
		},
		Spec: &corev1.ServiceSpecArgs{
			Selector: redisAppLabels,
			Ports: &corev1.ServicePortArray{
				corev1.ServicePortArgs{
					Name: pulumi.String("mainport"),
					Port: pulumi.Int(6379),
				},
			},
			Type: pulumi.String("ClusterIP"),
		},
	}, pulumi.Provider(r.provider), pulumi.Parent(namespace), pulumi.DependsOn([]pulumi.Resource{deployment}))

	return
}
