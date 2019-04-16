package utils

import (
	"math/rand"

	talosv1 "github.com/talos-systems/cluster-api-provider-talos/pkg/apis/talos/v1alpha1"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

//RandomString simply returns a string of length n
func RandomString(n int) string {
	var letter = []rune("abcdefghijklmnopqrstuvwxy0123456789")

	b := make([]rune, n)
	for i := range b {
		b[i] = letter[rand.Intn(len(letter))]
	}
	return string(b)
}

//ClusterProviderFromSpec parses out and returns provider specific cluster spec
func ClusterProviderFromSpec(providerSpec clusterv1.ProviderSpec) (*talosv1.TalosClusterProviderSpec, error) {
	var config talosv1.TalosClusterProviderSpec
	if err := yaml.Unmarshal(providerSpec.Value.Raw, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

//MachineProviderFromSpec parses out and returns provider specific machine spec
func MachineProviderFromSpec(providerSpec clusterv1.ProviderSpec) (*talosv1.TalosMachineProviderSpec, error) {
	var config talosv1.TalosMachineProviderSpec
	if err := yaml.Unmarshal(providerSpec.Value.Raw, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

//CreateK8sClientSet returns a kube client to use for calls to the api server
func CreateK8sClientSet() (*kubernetes.Clientset, error) {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientset, nil
}
