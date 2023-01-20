package app

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes"
	networkingv1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/networking/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"

	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/apps/v1"
	autoscalingv2 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/autoscaling/v2"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type App interface {
	CreateResources(hostname pulumi.StringOutput, dependentServices ...pulumi.Resource) error
}

type resource struct {
	ctx      *pulumi.Context
	provider *kubernetes.Provider
	cfg      *config.Config
}

type dockerconfig struct {
	Auths dockerauthentication `json:"auths"`
}

type dockerauthentication struct {
	Ghcr containerregistry `json:"ghcr.io"`
}

type containerregistry struct {
	Auth     string `json:"auth"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func NewApp(context *pulumi.Context, provider *kubernetes.Provider, cfg *config.Config) App {
	return resource{
		ctx:      context,
		provider: provider,
		cfg:      cfg,
	}
}

func (a resource) CreateResources(hostname pulumi.StringOutput, dependsOnResources ...pulumi.Resource) error {
	redisService := dependsOnResources[0].(*corev1.Service)
	mongodbService := dependsOnResources[1].(*corev1.Service)

	namespace, err := corev1.NewNamespace(a.ctx, "alura", &corev1.NamespaceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Labels: pulumi.StringMap{
				"name": pulumi.String("alura"),
			},
			Name: pulumi.String("alura"),
		},
	}, pulumi.Provider(a.provider), pulumi.DependsOn(dependsOnResources))

	if err != nil {
		return err
	}

	ghRegistrySecret, err := corev1.NewSecret(a.ctx, "gh-registry-secrets", &corev1.SecretArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("gh-registry-secret"),
			Namespace: namespace.Metadata.Name(),
		},
		Type: pulumi.String("kubernetes.io/dockerconfigjson"),
		StringData: pulumi.StringMap{
			".dockerconfigjson": a.createDockerConfigJson(),
		},
	}, pulumi.Provider(a.provider), pulumi.Parent(namespace))

	if err != nil {
		return err
	}

	mongoDBURI := pulumi.Sprintf("mongodb://%s.%s.svc.cluster.local:%d", mongodbService.Metadata.Name().Elem(), mongodbService.Metadata.Namespace().Elem(), mongodbService.Spec.Ports().Index(pulumi.Int(0)).Port())

	redisHost := pulumi.Sprintf("redis://%s.%s.svc.cluster.local:%d", redisService.Metadata.Name().Elem(), redisService.Metadata.Namespace().Elem(), redisService.Spec.Ports().Index(pulumi.Int(0)).Port())

	secret, err := corev1.NewSecret(a.ctx, "languages-api-secrets", &corev1.SecretArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("languages-api"),
			Namespace: namespace.Metadata.Name(),
		},
		Type: pulumi.String("Opaque"),
		StringData: pulumi.StringMap{
			"MONGODB_URI": mongoDBURI,
			"REDIS_HOST":  redisHost,
		},
	}, pulumi.Provider(a.provider), pulumi.Parent(namespace), pulumi.DependsOn([]pulumi.Resource{ghRegistrySecret}))

	if err != nil {
		return err
	}

	appLabels := pulumi.StringMap{
		"app": pulumi.String("languages-api"),
	}
	deployment, err := appsv1.NewDeployment(a.ctx, "languages-api", &appsv1.DeploymentArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("languages-api"),
			Namespace: namespace.Metadata.Name(),
			Labels:    appLabels,
		},
		Spec: appsv1.DeploymentSpecArgs{
			Selector: &metav1.LabelSelectorArgs{
				MatchLabels: appLabels,
			},
			Replicas: pulumi.Int(2),
			Template: &corev1.PodTemplateSpecArgs{
				Metadata: &metav1.ObjectMetaArgs{
					Labels: appLabels,
				},
				Spec: &corev1.PodSpecArgs{
					Containers: corev1.ContainerArray{
						corev1.ContainerArgs{
							Name:            pulumi.String("languages-api"),
							Image:           pulumi.String("ghcr.io/rodrigoafernandes/languages-api:24b7518bff37f26e9592fdf29c0e7d1da748ed4d"),
							ImagePullPolicy: pulumi.String("IfNotPresent"),
							Ports: corev1.ContainerPortArray{
								&corev1.ContainerPortArgs{
									ContainerPort: pulumi.Int(8080),
									Protocol:      pulumi.String("TCP"),
								},
							},
							Resources: &corev1.ResourceRequirementsArgs{
								Requests: pulumi.StringMap{
									"memory": pulumi.String("256Mi"),
									"cpu":    pulumi.String("80m"),
								},
								Limits: pulumi.StringMap{
									"memory": pulumi.String("800Mi"),
									"cpu":    pulumi.String("500m"),
								},
							},
							ReadinessProbe: &corev1.ProbeArgs{
								HttpGet: corev1.HTTPGetActionArgs{
									Path: pulumi.String("/q/health/ready"),
									Port: pulumi.Int(8080),
								},
								InitialDelaySeconds: pulumi.Int(1),
								PeriodSeconds:       pulumi.Int(30),
								TimeoutSeconds:      pulumi.Int(10),
								SuccessThreshold:    pulumi.Int(1),
								FailureThreshold:    pulumi.Int(3),
							},
							LivenessProbe: &corev1.ProbeArgs{
								HttpGet: corev1.HTTPGetActionArgs{
									Path: pulumi.String("/q/health/live"),
									Port: pulumi.Int(8080),
								},
								InitialDelaySeconds: pulumi.Int(1),
								PeriodSeconds:       pulumi.Int(30),
								TimeoutSeconds:      pulumi.Int(10),
								SuccessThreshold:    pulumi.Int(1),
								FailureThreshold:    pulumi.Int(3),
							},
							EnvFrom: &corev1.EnvFromSourceArray{
								corev1.EnvFromSourceArgs{
									SecretRef: corev1.SecretEnvSourceArgs{
										Name: secret.Metadata.Name(),
									},
								},
							},
						},
					},
					ImagePullSecrets: corev1.LocalObjectReferenceArray{
						corev1.LocalObjectReferenceArgs{
							Name: pulumi.String("gh-registry-secret"),
						},
					},
				},
			},
		},
	}, pulumi.Provider(a.provider), pulumi.Parent(namespace), pulumi.DependsOn([]pulumi.Resource{secret}))

	if err != nil {
		return err
	}

	service, err := corev1.NewService(a.ctx, "languages-api", &corev1.ServiceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("languages-api"),
			Namespace: namespace.Metadata.Name(),
		},
		Spec: &corev1.ServiceSpecArgs{
			Selector: pulumi.StringMap{
				"app": pulumi.String("languages-api"),
			},
			Ports: &corev1.ServicePortArray{
				corev1.ServicePortArgs{
					Name: pulumi.String("mainport"),
					Port: pulumi.Int(8080),
				},
			},
			Type: pulumi.String("ClusterIP"),
		},
	}, pulumi.Provider(a.provider), pulumi.Parent(namespace), pulumi.DependsOn([]pulumi.Resource{deployment}))

	if err != nil {
		return err
	}

	_, err = networkingv1.NewIngress(a.ctx, "languages-api", &networkingv1.IngressArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("languages-api"),
			Namespace: namespace.Metadata.Name(),
			Annotations: pulumi.StringMap{
				"kubernetes.io/ingress.class":                pulumi.String("nginx"),
				"nginx.ingress.kubernetes.io/rewrite-target": pulumi.String("/$2"),
			},
		},
		Spec: networkingv1.IngressSpecArgs{
			Rules: networkingv1.IngressRuleArray{
				networkingv1.IngressRuleArgs{
					Host: hostname,
					Http: networkingv1.HTTPIngressRuleValueArgs{
						Paths: networkingv1.HTTPIngressPathArray{
							networkingv1.HTTPIngressPathArgs{
								Path:     pulumi.String("/alura-languages(/|$)(.*)"),
								PathType: pulumi.String("Prefix"),
								Backend: networkingv1.IngressBackendArgs{
									Service: networkingv1.IngressServiceBackendArgs{
										Name: service.Metadata.Name().Elem().ToStringOutput(),
										Port: networkingv1.ServiceBackendPortArgs{
											Number: pulumi.Int(8080),
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}, pulumi.Provider(a.provider), pulumi.Parent(namespace), pulumi.DependsOn([]pulumi.Resource{service}))

	if err != nil {
		return err
	}

	_, err = autoscalingv2.NewHorizontalPodAutoscaler(a.ctx, "languages-api", &autoscalingv2.HorizontalPodAutoscalerArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("languages-api"),
			Namespace: namespace.Metadata.Name(),
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpecArgs{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReferenceArgs{
				ApiVersion: pulumi.String("apps/v1"),
				Kind:       pulumi.String("Deployment"),
				Name:       deployment.Metadata.Name().Elem().ToStringOutput(),
			},
			MinReplicas: pulumi.Int(2),
			MaxReplicas: pulumi.Int(10),
			Metrics: autoscalingv2.MetricSpecArray{
				autoscalingv2.MetricSpecArgs{
					Type: pulumi.String("Resource"),
					Resource: autoscalingv2.ResourceMetricSourceArgs{
						Name: pulumi.String("cpu"),
						Target: autoscalingv2.MetricTargetArgs{
							Type:               pulumi.String("Utilization"),
							AverageUtilization: pulumi.Int(313),
						},
					},
				},
			},
		},
	}, pulumi.Provider(a.provider), pulumi.Parent(namespace), pulumi.DependsOn([]pulumi.Resource{deployment}))

	if err != nil {
		return err
	}

	return nil
}

func (a resource) createDockerConfigJson() pulumi.StringOutput {
	user := a.cfg.Get("gh_user")
	token := a.cfg.GetSecret("gh_pat")
	return token.ApplyT(func(pat string) string {
		dockerConfig := dockerconfig{
			Auths: dockerauthentication{
				Ghcr: containerregistry{
					Auth:     base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", user, pat))),
					Username: user,
					Password: pat,
					Email:    "rodrigoforit@gmail.com",
				},
			},
		}
		dockerconfigJson, _ := json.Marshal(dockerConfig)
		return string(dockerconfigJson)
	}).(pulumi.StringOutput)
}
