package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
	"github.com/rodrigoafernandes/efk-cluster/app"
	"github.com/rodrigoafernandes/efk-cluster/cluster"
	es "github.com/rodrigoafernandes/efk-cluster/elasticsearch_logging"
	fluentdlogging "github.com/rodrigoafernandes/efk-cluster/fluentd_logging"
	ingresscontroller "github.com/rodrigoafernandes/efk-cluster/ingress-controller"
	kibanalogging "github.com/rodrigoafernandes/efk-cluster/kibana_logging"
	metricsserver "github.com/rodrigoafernandes/efk-cluster/metrics-server"
	"github.com/rodrigoafernandes/efk-cluster/mongodb"
	"github.com/rodrigoafernandes/efk-cluster/redis"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		k8sCluster := cluster.NewCluster(ctx)
		provider, err := k8sCluster.Create()
		if err != nil {
			return err
		}
		metricsServer := metricsserver.NewMetricsServerResource(ctx, provider)
		metricsServerResource, err := metricsServer.CreateResources()
		if err != nil {
			return err
		}
		ingressNginx := ingresscontroller.NewNginxIngressController(ctx, provider)
		ingressController, hostname, err := ingressNginx.CreateResources(metricsServerResource)
		if err != nil {
			return err
		}
		elasticSearch := es.NewElasticsearch(ctx, provider, cfg)
		logginNamespace, elasticSearchRelease, err := elasticSearch.CreateResources(ingressController)
		if err != nil {
			return err
		}
		kibana := kibanalogging.NewKibana(ctx, provider)
		err = kibana.CreateResources(logginNamespace, elasticSearchRelease, hostname)
		if err != nil {
			return err
		}
		fluentd := fluentdlogging.NewFluentD(ctx, provider, cfg)
		fluentdRelease, err := fluentd.ConfigureResources(logginNamespace, elasticSearchRelease)
		if err != nil {
			return err
		}
		redis := redis.NewRedis(ctx, provider)
		databasesNamespace, redisService, err := redis.CreateResources(ingressController, fluentdRelease)
		if err != nil {
			return err
		}
		mongoDB := mongodb.NewMongoDB(ctx, provider)
		mongodbService, err := mongoDB.CreateResources(databasesNamespace, ingressController, fluentdRelease)
		if err != nil {
			return err
		}
		application := app.NewApp(ctx, provider, cfg)
		err = application.CreateResources(hostname, redisService, mongodbService, fluentdRelease)
		if err != nil {
			return err
		}
		return nil
	})
}
