package azure

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-03-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-04-01/network"
	"github.com/Azure/go-autorest/autorest"
	azuresdk "github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/talos-systems/cluster-api-provider-talos/pkg/cloud/talos/utils"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

// Az represents a provider for Azure.
type Az struct {
}

// ClusterInfo holds data about desired config in cluster object
type ClusterInfo struct {
	Location      string
	ResourceGroup string
}

// MachineInfo holds data about desired config in machine object
type MachineInfo struct {
	Location      string
	ResourceGroup string
	Instances     InstanceInfo
}

// InstanceInfo holds data about the instances we'll create
type InstanceInfo struct {
	Type    string
	Image   string
	Network string
	Subnet  string
	Disks   DiskInfo
}

// DiskInfo holds disk info data
type DiskInfo struct {
	Size int
}

// Session is an object representing session for subscription
type Session struct {
	SubscriptionID string
	Authorizer     autorest.Authorizer
}

// NewAz returns an instance of the Azure provisioner
func NewAz() (*Az, error) {
	return &Az{}, nil
}

// Create creates an instance in Azure.
func (azure *Az) Create(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine, clientset *kubernetes.Clientset) error {

	machineSpec, err := utils.MachineProviderFromSpec(machine.Spec.ProviderSpec)
	if err != nil {
		return err
	}

	azureConfig := &MachineInfo{}
	yaml.Unmarshal([]byte(machineSpec.Platform.Config), azureConfig)

	// Dig out the subnet for our nic
	subnet, err := getSubnetByName(ctx, azureConfig)
	if err != nil {
		return err
	}

	// Begin crafting the config for the nic
	nicIPConfigProperties := &network.InterfaceIPConfigurationPropertiesFormat{
		PrivateIPAllocationMethod: network.Dynamic,
		Subnet:                    subnet,
	}

	// Find the public IP we want to use if necessary
	if !strings.Contains(machine.ObjectMeta.Name, "worker") {
		publicIPObject, err := getPublicIPByName(ctx, machine.ObjectMeta.Name+"-ip", azureConfig.ResourceGroup)
		if err != nil {
			return err
		}

		// If we don't receive an IP object, the public IP wasn't found
		// even though we're expecting it to be there. This logic may
		// eventually change once we create IPs for the user.
		if publicIPObject == nil {
			return errors.New("expected public IP not found")
		}
		nicIPConfigProperties.PublicIPAddress = publicIPObject
	}

	// Create a network interface for our VM to use (with flip attached if necessary)
	nic := network.Interface{
		Name:     to.StringPtr(machine.ObjectMeta.Name + "-nic"),
		Location: to.StringPtr(azureConfig.Location),
		InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
			IPConfigurations: &[]network.InterfaceIPConfiguration{
				{
					Name:                                     to.StringPtr(machine.ObjectMeta.Name + "-ip-config"),
					InterfaceIPConfigurationPropertiesFormat: nicIPConfigProperties,
				},
			},
		},
	}
	nicClient, err := nicclient()
	if err != nil {
		return err
	}
	nicfuture, err := nicClient.CreateOrUpdate(ctx, azureConfig.ResourceGroup, *nic.Name, nic)
	if err != nil {
		return err
	}
	err = nicfuture.WaitForCompletionRef(ctx, nicClient.Client)
	if err != nil {
		return err
	}
	nicObject, err := nicfuture.Result(*nicClient)
	if err != nil {
		return err
	}

	// Pull down userdata and b64 encode it
	udConfigMap, err := utils.FetchConfigMap(cluster, machine, clientset)
	if err != nil {
		return err
	}
	ud := udConfigMap.Data["userdata"]
	udb64 := base64.StdEncoding.EncodeToString([]byte(ud))

	// Specify dummy val pass. We don't it anyways but it's required.
	user := "talosuser"
	password := utils.RandomString(15)

	// Draft and create VM
	vm := compute.VirtualMachine{
		Location: to.StringPtr(azureConfig.Location),
		VirtualMachineProperties: &compute.VirtualMachineProperties{
			HardwareProfile: &compute.HardwareProfile{
				VMSize: compute.VirtualMachineSizeTypes(azureConfig.Instances.Type),
			},
			StorageProfile: &compute.StorageProfile{
				ImageReference: &compute.ImageReference{ID: to.StringPtr(azureConfig.Instances.Image)},
				OsDisk: &compute.OSDisk{
					OsType:       compute.Linux,
					Name:         to.StringPtr(machine.ObjectMeta.Name + "-os-disk"),
					CreateOption: compute.DiskCreateOptionTypesFromImage,
					DiskSizeGB:   to.Int32Ptr(int32(azureConfig.Instances.Disks.Size)),
				},
			},
			OsProfile: &compute.OSProfile{
				ComputerName:  to.StringPtr(machine.ObjectMeta.Name),
				AdminUsername: to.StringPtr(user),
				AdminPassword: to.StringPtr(password),
				CustomData:    to.StringPtr(udb64),
			},
			NetworkProfile: &compute.NetworkProfile{
				NetworkInterfaces: &[]compute.NetworkInterfaceReference{
					{
						ID: nicObject.ID,
						NetworkInterfaceReferenceProperties: &compute.NetworkInterfaceReferenceProperties{
							Primary: to.BoolPtr(true),
						},
					},
				},
			},
		},
	}

	vmClient, err := vmclient()
	if err != nil {
		return err
	}

	_, err = vmClient.CreateOrUpdate(ctx, azureConfig.ResourceGroup, machine.ObjectMeta.Name, vm)
	if err != nil {
		return err
	}

	log.Println("[Azure] Instance created: " + machine.ObjectMeta.Name)

	return nil
}

//Update updates a given Azure instance.
func (azure *Az) Update(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine, clientset *kubernetes.Clientset) error {
	return nil
}

// Delete deletes a Azure instance.
func (azure *Az) Delete(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine, clientset *kubernetes.Clientset) error {
	machineSpec, err := utils.MachineProviderFromSpec(machine.Spec.ProviderSpec)
	if err != nil {
		return err
	}

	azureConfig := &MachineInfo{}
	yaml.Unmarshal([]byte(machineSpec.Platform.Config), azureConfig)

	// Cleanup VM
	vmClient, err := vmclient()
	if err != nil {
		return err
	}
	vmfuture, err := vmClient.Delete(ctx, azureConfig.ResourceGroup, machine.ObjectMeta.Name)
	if err != nil {
		return err
	}

	// If VM is still deleting, return error so the rest of the cleanup is requeued
	// Azure returns a 204 when the VM isn't found
	if vmfuture.Response().StatusCode != http.StatusNoContent {
		return errors.New("[Azure] Waiting for VM completion to be completed")
	}

	// Cleanup os disk
	disksClient, err := disksclient()
	if err != nil {
		return err
	}
	_, err = disksClient.Delete(ctx, azureConfig.ResourceGroup, machine.ObjectMeta.Name+"-os-disk")
	if err != nil {
		return err
	}

	// Cleanup nic
	nicClient, err := nicclient()
	if err != nil {
		return err
	}
	_, err = nicClient.Delete(ctx, azureConfig.ResourceGroup, machine.ObjectMeta.Name+"-nic")
	if err != nil {
		return err
	}

	log.Println("[Azure] Instance deleted: " + machine.ObjectMeta.Name)

	return nil
}

// Exists returns whether or not an instance is present in Azure.
func (azure *Az) Exists(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine, clientset *kubernetes.Clientset) (bool, error) {
	machineSpec, err := utils.MachineProviderFromSpec(machine.Spec.ProviderSpec)
	if err != nil {
		return false, err
	}

	azureConfig := &MachineInfo{}
	yaml.Unmarshal([]byte(machineSpec.Platform.Config), azureConfig)

	vmClient, err := vmclient()
	if err != nil {
		return true, err
	}

	// If there's an error from retrieving the VM by name, assume it doesn't exist
	_, err = getVMByName(ctx, vmClient, azureConfig.ResourceGroup, machine.ObjectMeta.Name)
	if err != nil {
		return false, nil
	}

	return true, nil
}

// AllocateExternalIPs creates IPs for the control plane nodes
func (azure *Az) AllocateExternalIPs(cluster *clusterv1.Cluster, clientset *kubernetes.Clientset) ([]string, error) {

	clusterSpec, err := utils.ClusterProviderFromSpec(cluster.Spec.ProviderSpec)
	if err != nil {
		return nil, err
	}

	azureConfig := &ClusterInfo{}
	yaml.Unmarshal([]byte(clusterSpec.Platform.Config), azureConfig)
	client, err := ipclient()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	floatingIPs := []string{}
	for i := 0; i < clusterSpec.ControlPlane.Count; i++ {

		// Check if ips already exist and add to list early if so
		flip, err := getPublicIPByName(ctx, cluster.ObjectMeta.Name+"-master-"+strconv.Itoa(i)+"-ip", azureConfig.ResourceGroup)
		if err != nil {
			return nil, err
		}
		if flip != nil {
			floatingIPs = append(floatingIPs, *flip.PublicIPAddressPropertiesFormat.IPAddress)
			continue
		}

		// Create IP if needed
		result, err := client.CreateOrUpdate(
			ctx,
			azureConfig.ResourceGroup,
			cluster.ObjectMeta.Name+"-master-"+strconv.Itoa(i)+"-ip",
			network.PublicIPAddress{
				Sku: &network.PublicIPAddressSku{
					Name: network.PublicIPAddressSkuNameBasic,
				},
				PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
					PublicIPAllocationMethod: network.Static,
					PublicIPAddressVersion:   network.IPv4,
				},
				Location: to.StringPtr(azureConfig.Location),
			},
		)

		err = result.WaitForCompletionRef(ctx, client.Client)
		if err != nil {
			return nil, err
		}

		ip, err := result.Result(*client)
		if err != nil {
			return nil, err
		}
		floatingIPs = append(floatingIPs, *ip.PublicIPAddressPropertiesFormat.IPAddress)
	}
	return floatingIPs, nil
}

// DeAllocateExternalIPs cleans IPs for the control plane nodes
func (azure *Az) DeAllocateExternalIPs(cluster *clusterv1.Cluster, clientset *kubernetes.Clientset) error {

	clusterSpec, err := utils.ClusterProviderFromSpec(cluster.Spec.ProviderSpec)
	if err != nil {
		return err
	}

	azureConfig := &ClusterInfo{}
	yaml.Unmarshal([]byte(clusterSpec.Platform.Config), azureConfig)
	client, err := ipclient()
	if err != nil {
		return err
	}

	ctx := context.Background()

	for i := 0; i < clusterSpec.ControlPlane.Count; i++ {
		_, err := client.Delete(ctx, azureConfig.ResourceGroup, cluster.ObjectMeta.Name+"-master-"+strconv.Itoa(i)+"-ip")
		if err != nil {
			return err
		}
	}
	return nil
}

// getSubnetByName finds a given subnet (required for input along with network)
func getSubnetByName(ctx context.Context, azureConfig *MachineInfo) (*network.Subnet, error) {
	client, err := subnetclient()
	if err != nil {
		return nil, err
	}

	subnet, err := client.Get(ctx, azureConfig.ResourceGroup, azureConfig.Instances.Network, azureConfig.Instances.Subnet, "")
	if err != nil {
		return nil, err
	}

	return &subnet, nil
}

// getPublicIPbyName finds the public IP object from a list of all IP objects
func getPublicIPByName(ctx context.Context, name string, resourceGroup string) (*network.PublicIPAddress, error) {
	client, err := ipclient()
	if err != nil {
		return nil, err
	}

	//Dump all public IPs created and iterate to find our desired IP
	list, err := client.ListComplete(ctx, resourceGroup)
	if err != nil {
		return nil, err
	}
	for list.NotDone() {
		result := list.Value()
		if *result.Name == name {
			return &result, nil
		}
		err = list.NextWithContext(ctx)
		if err != nil {
			return nil, err
		}
	}

	// IP not found
	return nil, nil
}

// getPublicIPbyIP finds the public IP object from a list of all IP objects
func getPublicIPbyIP(ctx context.Context, ipAddress string, resourceGroup string) (*network.PublicIPAddress, error) {
	client, err := ipclient()
	if err != nil {
		return nil, err
	}

	//Dump all public IPs created and iterate to find our desired IP
	list, err := client.ListComplete(ctx, resourceGroup)
	if err != nil {
		return nil, err
	}
	for list.NotDone() {
		result := list.Value()
		if *result.PublicIPAddressPropertiesFormat.IPAddress == ipAddress {
			return &result, nil
		}
		err = list.NextWithContext(ctx)
		if err != nil {
			return nil, err
		}
	}

	// IP not found
	return nil, nil
}

// getVMByName finds the VM given a machine name
func getVMByName(ctx context.Context, vmClient *compute.VirtualMachinesClient, resourceGroup string, vmName string) (*compute.VirtualMachine, error) {
	vm, err := vmClient.Get(ctx, resourceGroup, vmName, "")
	if err != nil {
		return nil, err
	}

	return &vm, nil
}

// fetchCreds creates a new authorizer and parses out the credentials file. Used by various clients for creation
func fetchCreds() (autorest.Authorizer, map[string]interface{}, error) {
	authorizer, err := auth.NewAuthorizerFromFile(azuresdk.PublicCloud.ResourceManagerEndpoint)
	if err != nil {
		return nil, nil, err
	}

	credFile, err := ioutil.ReadFile(os.Getenv("AZURE_AUTH_LOCATION"))
	if err != nil {
		return nil, nil, err
	}

	credMap := make(map[string]interface{})
	err = json.Unmarshal(credFile, &credMap)
	if err != nil {
		return nil, nil, err
	}
	return authorizer, credMap, nil
}

// Creates client for use in disk ops
func disksclient() (*compute.DisksClient, error) {
	authorizer, credMap, err := fetchCreds()
	if err != nil {
		return nil, err
	}
	disksClient := compute.NewDisksClient(credMap["subscriptionId"].(string))
	disksClient.Authorizer = authorizer
	return &disksClient, nil
}

// Creates client for use in public IP ops
func ipclient() (*network.PublicIPAddressesClient, error) {
	authorizer, credMap, err := fetchCreds()
	if err != nil {
		return nil, err
	}
	ipClient := network.NewPublicIPAddressesClient(credMap["subscriptionId"].(string))
	ipClient.Authorizer = authorizer
	return &ipClient, nil
}

// Creates client for use in NIC ops
func nicclient() (*network.InterfacesClient, error) {
	authorizer, credMap, err := fetchCreds()
	if err != nil {
		return nil, err
	}
	nicClient := network.NewInterfacesClient(credMap["subscriptionId"].(string))
	nicClient.Authorizer = authorizer
	return &nicClient, nil
}

// Creates client for use in subnet ops
func subnetclient() (*network.SubnetsClient, error) {
	authorizer, credMap, err := fetchCreds()
	if err != nil {
		return nil, err
	}
	subnetClient := network.NewSubnetsClient(credMap["subscriptionId"].(string))
	subnetClient.Authorizer = authorizer
	return &subnetClient, nil
}

// Creates client for use in VM ops
func vmclient() (*compute.VirtualMachinesClient, error) {
	authorizer, credMap, err := fetchCreds()
	if err != nil {
		return nil, err
	}
	vmClient := compute.NewVirtualMachinesClient(credMap["subscriptionId"].(string))
	vmClient.Authorizer = authorizer
	return &vmClient, nil
}
