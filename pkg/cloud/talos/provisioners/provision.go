package provisioners

import (
	"context"
	"errors"

	"github.com/talos-systems/cluster-api-provider-talos/pkg/cloud/talos/provisioners/aws"
	"github.com/talos-systems/cluster-api-provider-talos/pkg/cloud/talos/provisioners/azure"
	"github.com/talos-systems/cluster-api-provider-talos/pkg/cloud/talos/provisioners/gce"
	"github.com/talos-systems/cluster-api-provider-talos/pkg/cloud/talos/provisioners/packet"
	"k8s.io/client-go/kubernetes"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

type Provisioner interface {
	Create(context.Context, *clusterv1.Cluster, *clusterv1.Machine, *kubernetes.Clientset) error
	Update(context.Context, *clusterv1.Cluster, *clusterv1.Machine, *kubernetes.Clientset) error

	Delete(context.Context, *clusterv1.Cluster, *clusterv1.Machine, *kubernetes.Clientset) error
	Exists(context.Context, *clusterv1.Cluster, *clusterv1.Machine, *kubernetes.Clientset) (bool, error)
}

func NewProvisioner(id string) (Provisioner, error) {
	switch id {
	case "aws":
		return aws.NewAWS()
	case "azure":
		return azure.NewAz()
	case "gce":
		return gce.NewGCE()
	case "packet":
		return packet.NewPacket()
	}
	return nil, errors.New("Unknown provisioner")
}
