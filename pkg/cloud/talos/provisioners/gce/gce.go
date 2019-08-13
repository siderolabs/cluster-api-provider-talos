package gce

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/talos-systems/cluster-api-provider-talos/pkg/cloud/talos/utils"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

// GCE represents a provider for GCE.
type GCE struct {
}

// ClusterInfo holds data about desired config in cluster object
type ClusterInfo struct {
	Region  string
	Project string
}

// MachineInfo holds data about desired config in machine object
type MachineInfo struct {
	Zone      string
	Project   string
	Instances InstanceInfo
}

// InstanceInfo holds data about the instances we'll create
type InstanceInfo struct {
	Type  string
	Image string
	Disks DiskInfo
}

// DiskInfo holds disk info data
type DiskInfo struct {
	Size int
}

//NewGCE returns an instance of the GCE provisioner
func NewGCE() (*GCE, error) {
	return &GCE{}, nil
}

// Create creates an instance in GCE.
func (gce *GCE) Create(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine, clientset *kubernetes.Clientset) error {

	machineSpec, err := utils.MachineProviderFromSpec(machine.Spec.ProviderSpec)
	if err != nil {
		return err
	}

	gceConfig := &MachineInfo{}
	yaml.Unmarshal([]byte(machineSpec.Platform.Config), gceConfig)

	computeService, err := client(clientset)
	if err != nil {
		return err
	}

	//fetch userdata based on machine name
	udConfigMap := &v1.ConfigMap{}
	natIP := ""
	if strings.Contains(machine.ObjectMeta.Name, "worker") {
		udConfigMap, err = clientset.CoreV1().ConfigMaps("cluster-api-provider-talos-system").Get(cluster.ObjectMeta.Name+"-workers", metav1.GetOptions{})
		if k8serrors.IsNotFound(err) {
			return err
		}
	} else {
		udConfigMap, err = clientset.CoreV1().ConfigMaps("cluster-api-provider-talos-system").Get(machine.ObjectMeta.Name, metav1.GetOptions{})
		if k8serrors.IsNotFound(err) {
			return err
		}

		// Parse the region out of zone
		zoneSlice := strings.Split(gceConfig.Zone, "-")
		regionSlice := zoneSlice[:len(zoneSlice)-1]
		region := strings.Join(regionSlice, "-")

		// Find public ip
		address, err := getPublicIPByName(computeService, machine.ObjectMeta.Name+"-ip", gceConfig.Project, region)
		if err != nil {
			return err
		}
		natIP = address.Address
	}
	ud := udConfigMap.Data["userdata"]

	//create instance with userdata
	_, err = computeService.Instances.Insert(gceConfig.Project, gceConfig.Zone, &compute.Instance{
		Name:         machine.ObjectMeta.Name,
		MachineType:  fmt.Sprintf("zones/%s/machineTypes/%s", gceConfig.Zone, gceConfig.Instances.Type),
		CanIpForward: true,
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				Network: "global/networks/default",
				AccessConfigs: []*compute.AccessConfig{
					{
						Type:  "ONE_TO_ONE_NAT",
						Name:  "External NAT",
						NatIP: natIP,
					},
				},
			},
		},
		Disks: []*compute.AttachedDisk{
			{
				AutoDelete: true,
				Boot:       true,
				InitializeParams: &compute.AttachedDiskInitializeParams{
					DiskSizeGb:  int64(gceConfig.Instances.Disks.Size),
					SourceImage: gceConfig.Instances.Image,
				},
			},
		},
		Metadata: &compute.Metadata{Items: []*compute.MetadataItems{&compute.MetadataItems{Key: "user-data", Value: &ud}}},
	},
	).Do()

	if err != nil {
		return err
	}

	return nil
}

//Update updates a given GCE instance.
func (gce *GCE) Update(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine, clientset *kubernetes.Clientset) error {

	return nil
}

// Delete deletes a GCE instance.
func (gce *GCE) Delete(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine, clientset *kubernetes.Clientset) error {

	// If instance isn't found by name, assume we no longer need to delete
	exists, err := gce.Exists(ctx, cluster, machine, clientset)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	machineSpec, err := utils.MachineProviderFromSpec(machine.Spec.ProviderSpec)
	if err != nil {
		return err
	}

	gceConfig := &MachineInfo{}
	yaml.Unmarshal([]byte(machineSpec.Platform.Config), gceConfig)

	computeService, err := client(clientset)
	if err != nil {
		return err
	}

	_, err = computeService.Instances.Delete(gceConfig.Project, gceConfig.Zone, machine.ObjectMeta.Name).Do()
	if err != nil {
		return err
	}
	return nil
}

// Exists returns whether or not an instance is present in GCE.
func (gce *GCE) Exists(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine, clientset *kubernetes.Clientset) (bool, error) {
	machineSpec, err := utils.MachineProviderFromSpec(machine.Spec.ProviderSpec)
	if err != nil {
		return false, err
	}

	gceConfig := &MachineInfo{}
	yaml.Unmarshal([]byte(machineSpec.Platform.Config), gceConfig)

	computeService, err := client(clientset)
	if err != nil {
		return true, err
	}

	_, err = computeService.Instances.Get(gceConfig.Project, gceConfig.Zone, machine.ObjectMeta.Name).Do()
	if err != nil && strings.Contains(err.Error(), "notFound") {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}

// AllocateExternalIPs creates IPs for the control plane nodes
func (gce *GCE) AllocateExternalIPs(cluster *clusterv1.Cluster, clientset *kubernetes.Clientset) ([]string, error) {
	clusterSpec, err := utils.ClusterProviderFromSpec(cluster.Spec.ProviderSpec)
	if err != nil {
		return nil, err
	}

	gceConfig := &ClusterInfo{}
	yaml.Unmarshal([]byte(clusterSpec.Platform.Config), gceConfig)

	computeService, err := client(clientset)
	if err != nil {
		return nil, err
	}

	floatingIPs := []string{}
	for i := 0; i < clusterSpec.ControlPlane.Count; i++ {

		//Check for address existence and return early if we can
		address, err := getPublicIPByName(computeService, cluster.ObjectMeta.Name+"-master-"+strconv.Itoa(i)+"-ip", gceConfig.Project, gceConfig.Region)
		if err != nil {
			return nil, err
		}
		if address != nil {
			floatingIPs = append(floatingIPs, address.Address)
			continue
		}

		// Insert the address and wait for it to be ready
		op, err := computeService.Addresses.Insert(gceConfig.Project, gceConfig.Region, &compute.Address{Name: cluster.ObjectMeta.Name + "-master-" + strconv.Itoa(i) + "-ip"}).Do()
		if err != nil {
			return nil, err
		}

		opStatus := &compute.Operation{}
		for opStatus.Status != "DONE" {
			opStatus, err = computeService.RegionOperations.Get(gceConfig.Project, gceConfig.Region, op.Name).Do()
			time.Sleep(5 * time.Second)
		}

		address, err = getPublicIPByName(computeService, cluster.ObjectMeta.Name+"-master-"+strconv.Itoa(i)+"-ip", gceConfig.Project, gceConfig.Region)
		if err != nil {
			return nil, err
		}
		floatingIPs = append(floatingIPs, address.Address)
	}

	return floatingIPs, nil
}

// DeAllocateExternalIPs cleans IPs for the control plane nodes
func (gce *GCE) DeAllocateExternalIPs(cluster *clusterv1.Cluster, clientset *kubernetes.Clientset) error {

	clusterSpec, err := utils.ClusterProviderFromSpec(cluster.Spec.ProviderSpec)
	if err != nil {
		return err
	}

	gceConfig := &ClusterInfo{}
	yaml.Unmarshal([]byte(clusterSpec.Platform.Config), gceConfig)

	computeService, err := client(clientset)
	if err != nil {
		return err
	}

	for i := 0; i < clusterSpec.ControlPlane.Count; i++ {
		_, err := computeService.Addresses.Delete(gceConfig.Project, gceConfig.Region, cluster.ObjectMeta.Name+"-master-"+strconv.Itoa(i)+"-ip").Do()
		if err != nil && !strings.Contains(err.Error(), "notFound") {
			return err
		}
	}

	return nil
}

func getPublicIPByName(computeService *compute.Service, name string, project string, region string) (*compute.Address, error) {
	addressList, err := computeService.Addresses.List(project, region).Do()
	if err != nil {
		return nil, err
	}

	for _, address := range addressList.Items {
		if address.Name == name {
			return address, nil
		}
	}

	// Nothing found
	return nil, nil

}

func client(clientset *kubernetes.Clientset) (*compute.Service, error) {
	creds, err := clientset.CoreV1().Secrets("cluster-api-provider-talos-system").Get("gce-credentials", metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return nil, err
	}

	//create client
	ctx := context.Background()
	return compute.NewService(ctx, option.WithCredentialsJSON(creds.Data["service-account.json"]))
}
