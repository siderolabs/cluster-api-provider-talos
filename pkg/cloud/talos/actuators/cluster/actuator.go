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
	"strconv"

	"github.com/talos-systems/cluster-api-provider-talos/pkg/cloud/talos/utils"
	"github.com/talos-systems/talos/pkg/userdata/generate"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	ClusterAPIProviderTalosNamespace = "cluster-api-provider-talos-system"
)

// ClusterActuator is responsible for performing machine reconciliation
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
type ClusterActuator struct{}

// ClusterActuatorParams holds parameter information for Actuator
type ClusterActuatorParams struct{}

// NewClusterActuator creates a new Actuator
func NewClusterActuator(mgr manager.Manager, params ClusterActuatorParams) (*ClusterActuator, error) {
	return &ClusterActuator{}, nil
}

// Reconcile reconciles a cluster and is invoked by the Cluster Controller
// TODO: This needs to be idempotent. Check if these certs and stuff already exist
func (a *ClusterActuator) Reconcile(cluster *clusterv1.Cluster) error {
	// creates the clientset
	// It feels like I may want a global k8s client set. We use it everywhere in both actuators
	clientset, err := utils.CreateK8sClientSet()
	if err != nil {
		return err
	}

	spec, err := utils.ClusterProviderFromSpec(cluster.Spec.ProviderSpec)
	if err != nil {
		return err
	}

	input, err := generate.NewInput(cluster.ObjectMeta.Name, spec.Masters.IPs)
	if err != nil {
		return err
	}

	err = createMasterConfigMaps(cluster, clientset, input)
	if err != nil {
		return err
	}
	err = createWorkerConfigMaps(cluster, clientset, input)
	if err != nil {
		return err
	}
	return nil
}

// Delete deletes a cluster and is invoked by the Cluster Controller
func (a *ClusterActuator) Delete(cluster *clusterv1.Cluster) error {
	clientset, err := utils.CreateK8sClientSet()
	if err != nil {
		return err
	}

	err = deleteConfigMaps(cluster, clientset)
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

	initData, err := generate.Userdata(generate.TypeInit, input)
	if err != nil {
		return err
	}

	controlPlaneData, err := generate.Userdata(generate.TypeControlPlane, input)
	if err != nil {
		return err
	}

	allData := []string{initData, controlPlaneData, controlPlaneData}

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
	if errors.IsAlreadyExists(err) {
		_, err = clientset.CoreV1().ConfigMaps(ClusterAPIProviderTalosNamespace).Update(cm)
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

	for index := range spec.Masters.IPs {
		name := cluster.ObjectMeta.Name + "-master-" + strconv.Itoa(index)
		err = clientset.CoreV1().ConfigMaps(ClusterAPIProviderTalosNamespace).Delete(name, nil)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	err = clientset.CoreV1().ConfigMaps(ClusterAPIProviderTalosNamespace).Delete(cluster.ObjectMeta.Name+"-workers", nil)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}
