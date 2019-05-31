package provisioners

import (
	"errors"

	"github.com/talos-systems/cluster-api-provider-talos/pkg/cloud/talos/provisioners/aws"
	"github.com/talos-systems/cluster-api-provider-talos/pkg/cloud/talos/provisioners/gce"
	"github.com/talos-systems/cluster-api-provider-talos/pkg/cloud/talos/provisioners/packet"
	"k8s.io/client-go/kubernetes"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

type Provisioner interface {
	Create(*clusterv1.Cluster, *clusterv1.Machine, *kubernetes.Clientset) error
	Update(*clusterv1.Cluster, *clusterv1.Machine, *kubernetes.Clientset) error

	Delete(*clusterv1.Cluster, *clusterv1.Machine, *kubernetes.Clientset) error
	Exists(*clusterv1.Cluster, *clusterv1.Machine, *kubernetes.Clientset) (bool, error)
}

func NewProvisioner(id string) (Provisioner, error) {
	switch id {
	case "gce":
		return gce.NewGCE()
	case "aws":
		return aws.NewAWS()
	case "packet":
		return packet.NewPacket()
	}
	return nil, errors.New("Unknown provisioner")
}
