package fluentdlogging

import (
	"github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/core/v1"
	"github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/helm/v3"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/meta/v1"
	rbac "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/rbac/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
	"io/ioutil"
)

type FluentD interface {
	ConfigureResources(*corev1.Namespace, *helm.Release) (pulumi.Resource, error)
}

type resource struct {
	ctx      *pulumi.Context
	provider *kubernetes.Provider
	cfg      *config.Config
}

func NewFluentD(context *pulumi.Context, provider *kubernetes.Provider, cfg *config.Config) FluentD {
	return resource{
		ctx:      context,
		provider: provider,
		cfg:      cfg,
	}
}

func (f resource) ConfigureResources(namespace *corev1.Namespace, elasticSearch *helm.Release) (release pulumi.Resource, err error) {
	fluentdConf, err := ioutil.ReadFile("fluentd_logging/fluentd.conf")
	if err != nil {
		return
	}
	esOutputConfigMap, err := corev1.NewConfigMap(f.ctx, "elasticsearch-output", &corev1.ConfigMapArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("elasticsearch-output-cm"),
			Namespace: namespace.Metadata.Name(),
		},
		Data: pulumi.StringMap{
			"fluentd.conf": pulumi.String(fluentdConf[:]),
		},
	}, pulumi.Provider(f.provider), pulumi.Parent(namespace), pulumi.DependsOn([]pulumi.Resource{elasticSearch}))
	if err != nil {
		return nil, err
	}
	clusterRole, err := rbac.NewClusterRole(f.ctx, "fluentd-aggregator-cr", &rbac.ClusterRoleArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("fluentd-aggregator-cr"),
			Labels: pulumi.StringMap{
				"app.kubernetes.io/instance": pulumi.String("fluentd"),
				"app.kubernetes.io/name":     pulumi.String("fluentd"),
			},
		},
		Rules: &rbac.PolicyRuleArray{
			&rbac.PolicyRuleArgs{
				ApiGroups: pulumi.StringArray{
					pulumi.String(""),
				},
				Resources: pulumi.StringArray{
					pulumi.String("namespaces"),
					pulumi.String("pods"),
				},
				Verbs: pulumi.StringArray{
					pulumi.String("get"),
					pulumi.String("watch"),
					pulumi.String("list"),
				},
			},
		},
	}, pulumi.Provider(f.provider), pulumi.Parent(namespace), pulumi.DependsOn([]pulumi.Resource{esOutputConfigMap}))
	if err != nil {
		return nil, err
	}
	aggregatorSa, err := corev1.NewServiceAccount(f.ctx, "fluentd-aggregator-sa", &corev1.ServiceAccountArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("fluentd-aggregator-sa"),
			Namespace: namespace.Metadata.Name(),
			Labels: pulumi.StringMap{
				"app.kubernetes.io/component": pulumi.String("aggregator"),
				"app.kubernetes.io/instance":  pulumi.String("fluentd"),
				"app.kubernetes.io/name":      pulumi.String("fluentd"),
			},
		},
		AutomountServiceAccountToken: pulumi.Bool(true),
	}, pulumi.Provider(f.provider), pulumi.Parent(namespace), pulumi.DependsOn([]pulumi.Resource{clusterRole}))
	if err != nil {
		return nil, err
	}
	crb, err := rbac.NewClusterRoleBinding(f.ctx, "fluentd-agrregator-crb", &rbac.ClusterRoleBindingArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("fluentd-aggregator-crb"),
			Labels: pulumi.StringMap{
				"app.kubernetes.io/instance": pulumi.String("fluentd"),
				"app.kubernetes.io/name":     pulumi.String("fluentd"),
			},
		},
		Subjects: &rbac.SubjectArray{
			&rbac.SubjectArgs{
				Kind:      pulumi.String("ServiceAccount"),
				Name:      aggregatorSa.Metadata.Name().Elem(),
				Namespace: namespace.Metadata.Name(),
			},
		},
		RoleRef: &rbac.RoleRefArgs{
			ApiGroup: pulumi.String("rbac.authorization.k8s.io"),
			Kind:     pulumi.String("ClusterRole"),
			Name:     clusterRole.Metadata.Name().Elem(),
		},
	}, pulumi.Provider(f.provider), pulumi.Parent(namespace), pulumi.DependsOn([]pulumi.Resource{aggregatorSa}))
	elasticsearchHost := pulumi.String("elasticsearch.efk-logging.svc.cluster.local")
	elasticsearchPort := pulumi.String("9200")
	release, err = helm.NewRelease(f.ctx, "fluentd", &helm.ReleaseArgs{
		Name:      pulumi.String("fluentd"),
		Namespace: namespace.Metadata.Name(),
		Chart:     pulumi.String("fluentd"),
		Version:   pulumi.String("5.5.12"),
		RepositoryOpts: helm.RepositoryOptsArgs{
			Repo: pulumi.String("https://charts.bitnami.com/bitnami"),
		},
		Values: pulumi.Map{
			"aggregator": pulumi.Map{
				"configMap": esOutputConfigMap.Metadata.Name(),
				"extraEnv": pulumi.MapArray{
					pulumi.Map{
						"name":  pulumi.String("ELASTICSEARCH_HOST"),
						"value": elasticsearchHost,
					},
					pulumi.Map{
						"name":  pulumi.String("ELASTICSEARCH_PORT"),
						"value": elasticsearchPort,
					},
					pulumi.Map{
						"name":  pulumi.String("ELASTICSEARCH_USER"),
						"value": pulumi.String(f.cfg.Get("elasticsearch_user")),
					},
					pulumi.Map{
						"name":  pulumi.String("ELASTICSEARCH_PASSWORD"),
						"value": f.cfg.GetSecret("elasticsearch_pwd"),
					},
				},
				"serviceAccount": pulumi.Map{
					"name": aggregatorSa.Metadata.Name(),
				},
			},
		},
		Timeout: pulumi.Int(300),
	}, pulumi.Provider(f.provider), pulumi.Parent(namespace), pulumi.DependsOn([]pulumi.Resource{esOutputConfigMap, crb}))

	return
}
