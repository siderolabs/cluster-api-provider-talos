/*
Copyright 2018 The Kubernetes authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cluster

import (
	"context"
	"errors"
	"log"
	"net"
	"strconv"

	"github.com/talos-systems/cluster-api-provider-talos/pkg/cloud/talos/provisioners"
	"github.com/talos-systems/cluster-api-provider-talos/pkg/cloud/talos/utils"
	"github.com/talos-systems/talos/pkg/userdata/v1/generate"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	ClusterAPIProviderTalosNamespace = "cluster-api-provider-talos-system"
)

// ClusterActuator is responsible for performing machine reconciliation
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// Add RBAC rules to access cluster-api resources
//+kubebuilder:rbac:groups=cluster.k8s.io,resources=clusters;clusters/status,verbs=get;list;watch
type ClusterActuator struct {
	Clientset        *kubernetes.Clientset
	controllerClient client.Client
}

// ClusterActuatorParams holds parameter information for Actuator
type ClusterActuatorParams struct {
}

// NewClusterActuator creates a new Actuator
func NewClusterActuator(mgr manager.Manager, params ClusterActuatorParams) (*ClusterActuator, error) {
	clientset, err := utils.CreateK8sClientSet()
	if err != nil {
		return nil, err
	}

	return &ClusterActuator{Clientset: clientset, controllerClient: mgr.GetClient()}, nil
}

// Reconcile reconciles a cluster and is invoked by the Cluster Controller
// TODO: This needs to be idempotent. Check if these certs and stuff already exist
func (a *ClusterActuator) Reconcile(cluster *clusterv1.Cluster) error {
	log.Printf("Reconciling cluster %v.", cluster.Name)

	spec, err := utils.ClusterProviderFromSpec(cluster.Spec.ProviderSpec)
	if err != nil {
		return err
	}

	//Generate external IPs depending on provisioner
	provisioner, err := provisioners.NewProvisioner(spec.Platform.Type)
	if err != nil {
		return err
	}

	masterIPs, err := provisioner.AllocateExternalIPs(cluster, a.Clientset)
	if err != nil {
		return err
	}

	//Create machine config, using IPs allocated above
	input, err := generate.NewInput(cluster.ObjectMeta.Name, masterIPs)
	if err != nil {
		return err
	}

	err = createMasterConfigMaps(cluster, a.Clientset, input)
	if err != nil {
		return err
	}
	err = createWorkerConfigMaps(cluster, a.Clientset, input)
	if err != nil {
		return err
	}
	return nil
}

// Delete deletes a cluster and is invoked by the Cluster Controller
func (a *ClusterActuator) Delete(cluster *clusterv1.Cluster) error {

	//Find all machines associated with this cluster and error if there are any
	//Avoids orphaning machines
	machines := &clusterv1.MachineList{}
	listOptions := &client.ListOptions{}
	listOptions.MatchingLabels(map[string]string{"cluster.k8s.io/cluster-name": cluster.ObjectMeta.Name})
	listOptions.InNamespace("")
	a.controllerClient.List(context.Background(), listOptions, machines)
	if len(machines.Items) > 0 {
		return errors.New("machines exist for cluster. failing to delete")
	}

	spec, err := utils.ClusterProviderFromSpec(cluster.Spec.ProviderSpec)
	if err != nil {
		return err
	}

	//Clean up external IPs depending on provisioner
	provisioner, err := provisioners.NewProvisioner(spec.Platform.Type)
	if err != nil {
		return err
	}

	err = provisioner.DeAllocateExternalIPs(cluster, a.Clientset)
	if err != nil {
		return err
	}

	//Clean up configmaps we create a cluster creation time
	err = deleteConfigMaps(cluster, a.Clientset)
	if err != nil {
		return err
	}

	return nil
}

// createMasterConfigMaps generates certs and creates configmaps that define the userdata for each node
func createMasterConfigMaps(cluster *clusterv1.Cluster, clientset *kubernetes.Clientset, input *generate.Input) error {

	talosconfig, err := generate.Talosconfig(input)
	if err != nil {
		return err
	}

	input.IP = net.ParseIP(input.MasterIPs[0])
	initData, err := generate.Userdata(generate.TypeInit, input)
	if err != nil {
		return err
	}

	allData := []string{initData}

	for i := 1; i < len(input.MasterIPs); i++ {
		input.IP = net.ParseIP(input.MasterIPs[i])
		input.Index = i

		controlPlaneData, err := generate.Userdata(generate.TypeControlPlane, input)
		if err != nil {
			return err
		}
		allData = append(allData, controlPlaneData)
	}

	for index, userdata := range allData {
		name := cluster.ObjectMeta.Name + "-master-" + strconv.Itoa(index)
		data := map[string]string{"userdata": userdata, "talosconfig": talosconfig}

		if err := createConfigMap(clientset, name, data); err != nil {
			return err
		}
	}

	return nil
}

// createWorkerConfigMaps creates a configmap for a machineset of workers
func createWorkerConfigMaps(cluster *clusterv1.Cluster, clientset *kubernetes.Clientset, input *generate.Input) error {
	workerData, err := generate.Userdata(generate.TypeJoin, input)
	if err != nil {
		return err
	}

	name := cluster.ObjectMeta.Name + "-workers"
	data := map[string]string{"userdata": workerData}

	return createConfigMap(clientset, name, data)
}

// createConfigMap creates a config map in kubernetes with given data
func createConfigMap(clientset *kubernetes.Clientset, name string, data map[string]string) error {
	cm := &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ClusterAPIProviderTalosNamespace,
		},
		Data: data,
	}

	_, err := clientset.CoreV1().ConfigMaps(ClusterAPIProviderTalosNamespace).Create(cm)
	// Essentially no-op if CMs are already there
	if k8serrors.IsAlreadyExists(err) {
		return nil
		//_, err = clientset.CoreV1().ConfigMaps(ClusterAPIProviderTalosNamespace).Update(cm)
	}

	return err
}

// deleteConfigMaps cleans up all configmaps associated with this cluster
func deleteConfigMaps(cluster *clusterv1.Cluster, clientset *kubernetes.Clientset) error {
	// TODO(andrewrynhard): We should add labels to the ConfigMaps and use
	// the lables to find the ConfigMaps.
	spec, err := utils.ClusterProviderFromSpec(cluster.Spec.ProviderSpec)
	if err != nil {
		return err
	}

	for index := 0; index < spec.ControlPlane.Count; index++ {
		name := cluster.ObjectMeta.Name + "-master-" + strconv.Itoa(index)
		err = clientset.CoreV1().ConfigMaps(ClusterAPIProviderTalosNamespace).Delete(name, nil)
		if err != nil && !k8serrors.IsNotFound(err) {
			return err
		}
	}

	err = clientset.CoreV1().ConfigMaps(ClusterAPIProviderTalosNamespace).Delete(cluster.ObjectMeta.Name+"-workers", nil)
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	return nil
}
