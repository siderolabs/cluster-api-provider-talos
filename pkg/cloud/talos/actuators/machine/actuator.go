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

package machine

import (
	"context"
	"log"

	"github.com/talos-systems/cluster-api-provider-talos/pkg/cloud/talos/provisioners"
	"github.com/talos-systems/cluster-api-provider-talos/pkg/cloud/talos/utils"
	"k8s.io/client-go/kubernetes"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

const (
	ProviderName = "talos"
)

// Add RBAC rules to access cluster-api resources
//+kubebuilder:rbac:groups=cluster.k8s.io,resources=machines;machines/status;machinedeployments;machinedeployments/status;machinesets;machinesets/status;machineclasses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.k8s.io,resources=clusters;clusters/status,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=nodes;events,verbs=get;list;watch;create;update;patch;delete

// MachineActuator is responsible for performing machine reconciliation
type MachineActuator struct {
	Provisioner provisioners.Provisioner
	Clientset   *kubernetes.Clientset
}

// MachineActuatorParams holds parameter information for Actuator
type MachineActuatorParams struct {
	Provisioner string
}

// NewMachineActuator creates a new Actuator
func NewMachineActuator(params MachineActuatorParams) (*MachineActuator, error) {
	provisioner, err := provisioners.NewProvisioner(params.Provisioner)
	if err != nil {
		return nil, err
	}

	clientset, err := utils.CreateK8sClientSet()
	if err != nil {
		return nil, err
	}

	return &MachineActuator{Provisioner: provisioner, Clientset: clientset}, nil
}

// Create creates a machine and is invoked by the Machine Controller
func (a *MachineActuator) Create(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	log.Printf("Creating machine %v for cluster %v.", machine.Name, cluster.Name)

	err := a.Provisioner.Create(cluster, machine, a.Clientset)
	if err != nil {
		return err
	}

	return nil
}

// Delete deletes a machine and is invoked by the Machine Controller
func (a *MachineActuator) Delete(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	log.Printf("Creating machine %v for cluster %v.", machine.Name, cluster.Name)

	err := a.Provisioner.Delete(machine, a.Clientset)
	if err != nil {
		return err
	}

	return nil
}

// Update updates a machine and is invoked by the Machine Controller
func (a *MachineActuator) Update(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	log.Printf("Updating machine %v for cluster %v.", machine.Name, cluster.Name)

	err := a.Provisioner.Update(cluster, machine, a.Clientset)
	if err != nil {
		return err
	}

	return nil
}

// Exists tests for the existence of a machine and is invoked by the Machine Controller
func (a *MachineActuator) Exists(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) (bool, error) {
	log.Printf("Checking if machine %v for cluster %v exists.", machine.Name, cluster.Name)

	exists, err := a.Provisioner.Exists(machine, a.Clientset)
	if err != nil {
		return true, err
	}

	return exists, nil
}
