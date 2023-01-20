package mongodb

import (
	"github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes"
	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type MongoDB interface {
	CreateResources(databasesNamespace *corev1.Namespace, dependsOn ...pulumi.Resource) (*corev1.Service, error)
}

type resource struct {
	ctx      *pulumi.Context
	provider *kubernetes.Provider
}

func NewMongoDB(context *pulumi.Context, provider *kubernetes.Provider) MongoDB {
	return resource{
		ctx:      context,
		provider: provider,
	}
}

func (m resource) CreateResources(databasesNamespace *corev1.Namespace, dependsOn ...pulumi.Resource) (mongodbService *corev1.Service, err error) {
	mongoDBAppLabels := pulumi.StringMap{
		"app": pulumi.String("mongodb"),
	}

	deployment, err := appsv1.NewDeployment(m.ctx, "mongodb", &appsv1.DeploymentArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("mongodb"),
			Labels:    mongoDBAppLabels,
			Namespace: databasesNamespace.Metadata.Name(),
		},
		Spec: appsv1.DeploymentSpecArgs{
			Selector: &metav1.LabelSelectorArgs{
				MatchLabels: mongoDBAppLabels,
			},
			Replicas: pulumi.Int(1),
			Template: &corev1.PodTemplateSpecArgs{
				Metadata: &metav1.ObjectMetaArgs{
					Labels: mongoDBAppLabels,
				},
				Spec: &corev1.PodSpecArgs{
					Containers: corev1.ContainerArray{
						corev1.ContainerArgs{
							Name:            pulumi.String("mongodb"),
							Image:           pulumi.String("docker.io/mongo:5.0.9"),
							ImagePullPolicy: pulumi.String("IfNotPresent"),
							Ports: corev1.ContainerPortArray{
								&corev1.ContainerPortArgs{
									ContainerPort: pulumi.Int(27017),
									Protocol:      pulumi.String("TCP"),
								},
							},
						},
					},
				},
			},
		},
	}, pulumi.Provider(m.provider), pulumi.Parent(databasesNamespace), pulumi.DependsOn(dependsOn))

	if err != nil {
		return nil, err
	}

	mongodbService, err = corev1.NewService(m.ctx, "mongodb", &corev1.ServiceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("mongodb"),
			Namespace: databasesNamespace.Metadata.Name(),
		},
		Spec: &corev1.ServiceSpecArgs{
			Selector: pulumi.StringMap{
				"app": pulumi.String("mongodb"),
			},
			Ports: &corev1.ServicePortArray{
				corev1.ServicePortArgs{
					Name: pulumi.String("mainport"),
					Port: pulumi.Int(27017),
				},
			},
			Type: pulumi.String("ClusterIP"),
		},
	}, pulumi.Provider(m.provider), pulumi.Parent(databasesNamespace), pulumi.DependsOn([]pulumi.Resource{deployment}))

	return
}
