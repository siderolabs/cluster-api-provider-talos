/*
Copyright 2018 The Kubernetes Authors.

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

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/talos-systems/cluster-api-provider-talos/pkg/apis"
	"github.com/talos-systems/cluster-api-provider-talos/pkg/cloud/talos/actuators/cluster"
	"github.com/talos-systems/cluster-api-provider-talos/pkg/cloud/talos/actuators/machine"
	clusterapis "sigs.k8s.io/cluster-api/pkg/apis"
	"sigs.k8s.io/cluster-api/pkg/apis/cluster/common"
	capicluster "sigs.k8s.io/cluster-api/pkg/controller/cluster"
	capimachine "sigs.k8s.io/cluster-api/pkg/controller/machine"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

var provisioners = []string{"gce"}

func main() {
	cfg := config.GetConfigOrDie()
	if cfg == nil {
		panic(fmt.Errorf("GetConfigOrDie didn't die"))
	}

	flag.Parse()
	log := logf.Log.WithName("talos-controller-manager")
	logf.SetLogger(logf.ZapLogger(false))
	entryLog := log.WithName("entrypoint")

	// Setup a Manager
	mgr, err := manager.New(cfg, manager.Options{})
	if err != nil {
		entryLog.Error(err, "unable to set up overall controller manager")
		os.Exit(1)
	}

	clusterActuator, err := cluster.NewClusterActuator(mgr, cluster.ClusterActuatorParams{})
	if err != nil {
		panic(err)
	}

	//Create machine actuator for each provisioner type
	var machineActuators []*machine.MachineActuator
	for _, provisioner := range provisioners {
		newActuator, err := machine.NewMachineActuator(machine.MachineActuatorParams{Provisioner: provisioner})
		if err != nil {
			panic(err)
		}
		machineActuators = append(machineActuators, newActuator)
	}

	// Register our cluster deployer (the interface is in clusterctl and we define the Deployer interface on the actuator)
	common.RegisterClusterProvisioner("talos", clusterActuator)

	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		panic(err)
	}

	if err := clusterapis.AddToScheme(mgr.GetScheme()); err != nil {
		panic(err)
	}

	//Add all machine actuators
	for _, actuator := range machineActuators {
		capimachine.AddWithActuator(mgr, actuator)
	}

	capicluster.AddWithActuator(mgr, clusterActuator)

	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		entryLog.Error(err, "unable to run manager")
		os.Exit(1)
	}
}
