# cluster-api-provider-talos on GCE

This guide will detail how to deploy the Talos provider into an existing Kubernetes cluster, as well as how to configure it to create Clusters and Machines in GCE.

#### Import Image

To import the image, you must download a .tar.gz talos release, add it to Google storage, and import it as an image.

- Download the `talos.tar.gz` image from our [Github releases](https://github.com/talos-systems/talos/releases).

- Follow the [Google instructions](https://cloud.google.com/compute/docs/images/import-existing-image#import_image) on importing an image using cloud storage.


#### Prepare bootstrap cluster

In your cluster that you'll be using to create other clusters, you must prepare a few bits.

- Git clone this repo.

- Create a namespace for our provider with `kubectl create ns cluster-api-provider-talos-system`.

- In GCE, create a service account and generate keys for the account. This will result in a JSON file containing the keys. As of now, this file needs to be named `service-account.json`. General instructions for generating the key can be found [here](https://cloud.google.com/iam/docs/creating-managing-service-account-keys).

- Create a secret with the key generated above: `kubectl create secret generic machine-controller-credential -n cluster-api-provider-talos-system --from-file /path/to/service-account.json`.

- Generate the manifests for deploying into the bootstrap cluster with `make manifests` from the `cluster-api-provider-talos` directory.

- Deploy the generated manifests with `kubectl create -f provider-components.yaml`


#### Create new clusters

There are sample kustomize templates in [config/samples/cluster-deployment/gce](../config/samples/cluster-deployment/gce) for deploying clusters. These will be our starting point.

- In GCE, create an external IP address for each of your desired masters. Take note of these IPs for the next step.

- Edit `master-ips.yaml`, `platform-config-master`, and `platform-config-workers.yaml` with your relevant data. 

- From `config/samples/cluster-deployment/gce` issue `kustomize build | kubectl apply -f -`.

- The talos config for your master can be found with `kubectl get cm -n cluster-api-provider-talos-system talos-test-cluster-master-0 -o jsonpath='{.data.admin\.conf}'`.