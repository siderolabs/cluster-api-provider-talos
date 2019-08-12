# cluster-api-provider-talos on Packet

This guide will detail how to deploy the Talos provider into an existing Kubernetes cluster, as well as how to configure it to create Clusters and Machines in Packet.

**NOTE: This guide assumes you have a PXE server setup in Packet with the relevant Talos objects already present** 

#### Prepare bootstrap cluster

In your cluster that you'll be using to create other clusters, you must prepare a few bits.

- Git clone this repo.

- Create a namespace for our provider with `kubectl create ns cluster-api-provider-talos-system`.

- In the Packet console, create an API token for the provider to use. 

- In this repo, edit `config/manager/manager.yaml` and replace `{{PACKET_AUTH_TOKEN}}` with your Packet API token you just generated.

- Generate the manifests for deploying into the bootstrap cluster with `make manifests` from the `cluster-api-provider-talos` directory.

- Deploy the generated manifests with `kubectl create -f provider-components.yaml`


#### Create new clusters

There are sample kustomize templates in [config/samples/cluster-deployment/packet](../config/samples/cluster-deployment/packet) for deploying clusters. These will be our starting point.

- In Packet, create a small subnet of elastic IPs for attaching to your masters. A /30 subnet should work well. Take note of the IPs in this subnet for the next step. General instructions for creating these [here](https://support.packet.com/kb/articles/elastic-ips).

- Edit `platform-config-cluster.yaml`, `platform-config-master.yaml`, and `platform-config-workers.yaml` with your relevant data, adding the IP block created above to `platform-config-cluster.yaml`.

- From `config/samples/cluster-deployment/packet` issue `kustomize build | kubectl apply -f -`.

- The talos config for your master can be found with `kubectl get cm -n cluster-api-provider-talos-system talos-test-cluster-master-0 -o jsonpath='{.data.talosconfig}'`.