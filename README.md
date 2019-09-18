# GKE Node Drainer
### A tool to drain GKE preemptible nodes

Inspired by https://github.com/estafette/estafette-gke-preemptible-killer, this tool drains the nodes that arent covered by [estafette-gke-preemptible-killer](https://github.com/estafette/estafette-gke-preemptible-killer).

#### Why

Nodes sometimes slip from [estafette-gke-preemptible-killer](https://github.com/estafette/estafette-gke-preemptible-killer) and get terminated before [estafette-gke-preemptible-killer](https://github.com/estafette/estafette-gke-preemptible-killer) gets to rotate it. This tool makes sure that if a preemptible node gets deleted, it gets cordoned and drained to ensure a graceful shutdown is achieved.

#### How

Google logs on stackdriver whenever an instance is getting removed from an instance group, which needs to be exported to pub/sub. Once a message is sent to the topic, we consume it, then cordon the node and drain the pods on it. This is to make sure pods are gracefully terminated.

    
## Usage


### 1. Create Pub/Sub Topic & Subscription

```
gcloud pubsub topics create --project <project> <topic_name>
gcloud pubsub subscriptions create <subscription_name> --topic=<topic_name> --project <project>
```
### 2. Create stackdriver export sink
Create the sink to export logs to and grant the service account generated in the command above Pub/Sub Publisher role on the topic you created.

```
$ gcloud logging sinks create <sink> pubsub.googleapis.com/projects/<project>/topics/<topic> \
--log-filter="protoPayload.methodName='v1.compute.instanceGroups.removeInstances' \
protoPayload.serviceName='compute.googleapis.com' operation.producer='type.googleapis.com'"\
--project <project>

--- output ----

Created [https://logging.googleapis.com/v2/projects/<project>/sinks/<sink>].
Please remember to grant `serviceAccount:<service_account>` the Pub/Sub Publisher role on the topic.
```

### 3. Create service account
Create a service account that will be used by the tool to subscribe to the topic

```
export project=<project>
gcloud iam --project=$project service-accounts create node-drainer \
    --display-name node-drainer
export SERVICE_ACCOUNT=$(gcloud iam --project=$project service-accounts list --filter node-drainer --format 'value([email])')
gcloud projects add-iam-policy-binding $project \
	--member=serviceAccount:$SERVICE_ACCOUNT \
	--role=roles/pubsub.subscriber
gcloud iam --project=$project service-accounts keys create \
    --iam-account $SERVICE_ACCOUNT \
    google_service_account.json

```

## Environment Variables


| Environment Variable  | Description | Default |
| ------  | ------ | -------------|
| PUBSUB_SUBSCRIPTION | subscription to listen to | " " |
| KUBECONFIG | kubeconfig file if using out of cluster auth | " " |
| KUBERNETES_SERVICE_HOST | k8s host to connect to (within cluster auth) | Supplied by k8s by default |
| KUBERNETES_SERVICE_PORT | k8s port to connect to (within cluster auth) | Supplied by k8s by default |
| KUBERNETES_NAMESPACE | k8s namespace to use | Supplied by k8s by default |
| INSTANCE_GROUPS | (comma seperated list) gke instance groups to check if node belongs to | " " |


## Build
`docker build . -t gke-node-drainer`

## Deploy with Helm

`helm install -n autoscaler helm/gke-node-drainer`


