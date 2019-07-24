# cluster-api-provider-talos on Azure

This guide will detail how to deploy the Talos provider into an existing Kubernetes cluster, as well as how to configure it to create Clusters and Machines in Azure.

#### Prereqs

This guide assumes you have several prereqs created in Azure. These are:

- Resource group
- Virtual network
- Subnet in the virtual network
- A security group for the subnet that allows ports 443, 50000, 50001
- Public IPs for each master
- Storage account for image upload
- The Azure CLI configured and talking to your Azure account

#### Import Image

To import the image, you must download a .tar.gz talos release, add it to Google storage, and import it as an image.

- Download the `talos-azure.tar.gz` image from our [Github releases](https://github.com/talos-systems/talos/releases) and extract it to get `disk.vhd`.

- Follow the [Azure instructions](https://docs.microsoft.com/en-us/azure/virtual-machines/linux/upload-vhd) on importing the vhd to cloud storage and creating an image.


#### Prepare bootstrap cluster

In your cluster that you'll be using to create other clusters, you must prepare a few bits.

- Git clone this repo.

- Create a namespace for our provider with `kubectl create ns cluster-api-provider-talos-system`.

- In Azure, create service account keys and write them to a file called `service-account-azure.json` with the command `az ad sp create-for-rbac --sdk-auth > /path/to/service-account-azure.json`.

- Create a secret with the key generated above: `kubectl create secret generic azure-credentials -n cluster-api-provider-talos-system --from-file /path/to/service-account-azure.json`.

- Generate the manifests for deploying into the bootstrap cluster with `make manifests` from the `cluster-api-provider-talos` directory.

- Deploy the generated manifests with `kubectl create -f provider-components.yaml`

#### Create new clusters

There are sample kustomize templates in [config/samples/cluster-deployment/azure](../config/samples/cluster-deployment/azure) for deploying clusters. These will be our starting point.

- Edit `master-ips.yaml`, `platform-config-master.yaml`, and `platform-config-workers.yaml` with your relevant data from the prereqs mentioned above. 

- From `config/samples/cluster-deployment/azure` issue `kustomize build | kubectl apply -f -`.

- The talos config for your master can be found with `kubectl get cm -n cluster-api-provider-talos-system talos-test-cluster-master-0 -o jsonpath='{.data.talosconfig}'`.