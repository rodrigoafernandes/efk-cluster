package cluster

import (
	"encoding/base64"
	"fmt"
	"github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes"
	"github.com/pulumi/pulumi-linode/sdk/v3/go/linode"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"io/fs"
	"io/ioutil"
)

type K8sCluster interface {
	Create() (*kubernetes.Provider, error)
}

type cluster struct {
	ctx *pulumi.Context
}

func (c cluster) Create() (*kubernetes.Provider, error) {
	k8sCluster, err := linode.NewLkeCluster(c.ctx, "efk-cluster", &linode.LkeClusterArgs{
		K8sVersion: pulumi.String("1.25"),
		Label:      pulumi.String("efk-cluster"),
		Pools: linode.LkeClusterPoolArray{
			&linode.LkeClusterPoolArgs{
				Count: pulumi.Int(3),
				Type:  pulumi.String("g6-dedicated-4"),
			},
		},
		Region: pulumi.String("us-central"),
		Tags: pulumi.StringArray{
			pulumi.String("dev"),
			pulumi.String("poc"),
		},
	})
	if err != nil {
		return nil, err
	}
	provider, err := kubernetes.NewProvider(c.ctx, "k8s_provider", &kubernetes.ProviderArgs{
		Kubeconfig:            c.createKubeconfig(k8sCluster.Kubeconfig),
		EnableServerSideApply: pulumi.Bool(true),
	})
	if err != nil {
		return nil, err
	}
	c.ctx.Export("kubeconfig", c.createKubeconfig(k8sCluster.Kubeconfig).ApplyT(func(kcfg string) string {
		err = ioutil.WriteFile("efk-cluster-kubeconfig.yaml", []byte(kcfg), fs.FileMode(0600))
		if err != nil {
			return ""
		}
		return kcfg
	}).(pulumi.StringOutput))
	c.ctx.Export("kubeconfig-context", k8sCluster.ID().ApplyT(func(v pulumi.ID) string {
		return fmt.Sprintf("lke%s-ctx", v)
	}))
	return provider, nil
}

func NewCluster(ctx *pulumi.Context) K8sCluster {
	return cluster{ctx: ctx}
}

func (c cluster) createKubeconfig(kubeconfig pulumi.StringOutput) pulumi.StringOutput {
	return kubeconfig.ApplyT(func(v string) string {
		decodedLen := make([]byte, len(v))
		decoded, _ := base64.StdEncoding.Decode(decodedLen, []byte(v))
		return string(decodedLen[:decoded])
	}).(pulumi.StringOutput)
}
