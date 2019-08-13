# cluster-api-provider-talos on AWS

This guide will detail how to deploy the Talos provider into an existing Kubernetes cluster, as well as how to configure it to create Clusters and Machines in AWS.

#### Prepare bootstrap cluster

In your cluster that you'll be using to create other clusters, you must prepare a few bits.

- Git clone this repo.

- Create a namespace for our provider with `kubectl create ns cluster-api-provider-talos-system`.

- In AWS, generate an access key and secret access key. Save these to a file called `credentials` with the following format. General instructions for generating the keys can be found [here](https://aws.amazon.com/blogs/security/how-to-find-update-access-keys-password-mfa-aws-management-console/).

```
[default]
aws_access_key_id = AKI...
aws_secret_access_key = MhM...
```

- Create a secret with the key generated above: `kubectl create secret generic aws-credentials -n cluster-api-provider-talos-system --from-file /path/to/credentials`.

- Generate the manifests for deploying into the bootstrap cluster with `make manifests` from the `cluster-api-provider-talos` directory.

- Deploy the generated manifests with `kubectl create -f provider-components.yaml`

#### Create new clusters

There are sample kustomize templates in [config/samples/cluster-deployment/aws](../config/samples/cluster-deployment/aws) for deploying clusters. These will be our starting point.

- Edit `platform-config-cluster.yaml`, `platform-config-master.yaml`, and `platform-config-workers.yaml` with your relevant data. 

- From `config/samples/cluster-deployment/aws` issue `kustomize build | kubectl apply -f -`. External IPs will get created and associated with Control Plane nodes automatically.

- The talos config for your master can be found with `kubectl get cm -n cluster-api-provider-talos-system talos-test-cluster-master-0 -o jsonpath='{.data.talosconfig}'`.