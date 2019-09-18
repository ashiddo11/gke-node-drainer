package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/rs/zerolog/log"

	"cloud.google.com/go/pubsub"
	"github.com/ericchiang/k8s"
)

type Instance struct {
	ProtoPayload struct {
		Request struct {
			Instances []struct {
				Instance string `json:"instance"`
			} `json:"instances"`
		} `json:"request"`
	} `json:"protoPayload"`
}

var (
	kubeClient     *k8s.Client
	kubernetes     KubernetesClient
	instanceGroups []string
	proj           string
	sub            string
)

func init() {
	// kubeconfig := "/tmp/config"
	// data, err := ioutil.ReadFile(kubeconfig)
	// if err != nil {
	// 	panic(err.Error())
	// }

	// // Unmarshal YAML into a Kubernetes config object.
	// var config k8s.Config
	// if err := yaml.Unmarshal(data, &config); err != nil {
	// 	panic(err.Error())
	// }
	// kubeClient, err = k8s.NewClient(&config)
	// if err != nil {
	// 	panic(err.Error())
	// }
	var err error
	kubernetes, err = NewKubernetesClient(os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT"),
		os.Getenv("KUBERNETES_NAMESPACE"), os.Getenv("KUBECONFIG"))

	if err != nil {
		log.Fatal().Err(err).Msg("Error initializing Kubernetes client")
	}

	instanceGroups = strings.Split(os.Getenv("INSTANCE_GROUPS"), ",")

	proj = os.Getenv("PROJECT")
	sub = os.Getenv("PUBSUB_SUBSCRIPTION")

}

func main() {

	ctx := context.Background()
	if sub == "" {
		log.Error().Msgf("PUBSUB_SUB environment variable must be set.\n")
		os.Exit(1)
	}
	if proj == "" {
		log.Error().Msgf("GOOGLE_CLOUD_PROJECT environment variable must be set.\n")
		os.Exit(1)
	}
	client, err := pubsub.NewClient(ctx, proj)
	if err != nil {
		log.Error().Err(err).Msgf("Could not create pubsub Client")
		os.Exit(1)
	}

	// Pull messages via the subscription.
	err = pullMsgsSettings(client, sub)
	if err != nil {
		log.Error().Err(err).Msg("Please check subscription exists")
	}
}

func pullMsgsSettings(client *pubsub.Client, subName string) error {
	ctx := context.Background()
	// [START pubsub_subscriber_flow_settings]
	instance := ""
	sub := client.Subscription(subName)
	sub.ReceiveSettings.Synchronous = true
	sub.ReceiveSettings.MaxOutstandingMessages = 10
	log.Info().Msgf("Starting consumer")
	err := sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		defer msg.Ack()
		data := Instance{}
		json.Unmarshal([]byte(msg.Data), &data)
		instance = data.ProtoPayload.Request.Instances[0].Instance
		rawInstance := strings.Split(instance, "/")
		nodeName := rawInstance[len(rawInstance)-1]
		node, err := kubernetes.GetNode(nodeName)
		if err != nil {
			log.Error().Err(err).Msgf("Couldn't find host %s", nodeName)
			return
		}

		// check node belongs to instance groups we're monitoring
		if !checkInstanceGroup(*node.Metadata.Name) {
			log.Info().Str("host: ", *node.Metadata.Name).Msg("Host not in instance groups specified, skipping")
			return
		}

		// Make sure we don't drain nodes already cordoned by k8s
		if *node.Spec.Unschedulable == true {
			log.Info().Str("host: ", *node.Metadata.Name).Msg("Host is already marked as unschedulable, skipping")
			return
		}

		// set node unschedulable
		err = kubernetes.SetUnschedulableState(*node.Metadata.Name, true)
		if err != nil {
			log.Error().Err(err).Msgf("Error setting node to unschedulable state")
			return
		}
		// drain kubernetes node
		err = kubernetes.DrainNode(*node.Metadata.Name, 60)

		if err != nil {
			log.Error().Err(err).Msgf("Error draining %v node", *node.Metadata.Name)
			return
		}

		// drain kube-dns from kubernetes node
		err = kubernetes.DrainKubeDNSFromNode(*node.Metadata.Name, 60)

		if err != nil {
			log.Error().Err(err).Msgf("Error draining kube-dns from kubernetes node")
			return
		}
	})
	if err != nil {
		return err
	}
	// [END pubsub_subscriber_flow_settings]
	return nil
}

func checkInstanceGroup(name string) bool {
	if len(instanceGroups) < 1 {
		log.Info().Msgf("INSTANCE_GROUPS ENV not set, skipping instance groups check")
		return true
	}
	for _, i := range instanceGroups {
		if strings.Contains(name, i) {
			return true
		}
	}
	return false
}
