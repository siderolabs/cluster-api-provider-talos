package packet

import (
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/packethost/packngo"
	"github.com/talos-systems/cluster-api-provider-talos/pkg/cloud/talos/utils"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

// Packet represents a provider for Packet.
type Packet struct {
	client *packngo.Client
}

// ClusterInfo holds data about desired config in cluster object
type ClusterInfo struct {
	ProjectID string
	Instances InstanceInfo
}

// InstanceInfo holds data about the instances we'll create
type InstanceInfo struct {
	Plan     string
	Facility string
	PXEUrl   string
	Install  map[string]interface{}
}

// Userdata holds userdata in struct form
type Userdata struct {
	Version    string
	Security   interface{}
	Services   interface{}
	Install    map[string]interface{}
	Networking *Network
}

type Network struct {
	OS *OS `yaml:"os,omitempty"`
}

type OS struct {
	Devices []map[string]interface{} `yaml:"devices,omitempty"`
}

//NewPacket returns an instance of the Packet provisioner
func NewPacket() (*Packet, error) {
	c, err := packngo.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	return &Packet{client: c}, nil
}

// Create creates an instance in Packet.
func (packet *Packet) Create(cluster *clusterv1.Cluster, machine *clusterv1.Machine, clientset *kubernetes.Clientset) error {

	clusterSpec, err := utils.ClusterProviderFromSpec(cluster.Spec.ProviderSpec)
	if err != nil {
		return err
	}

	machineSpec, err := utils.MachineProviderFromSpec(machine.Spec.ProviderSpec)
	if err != nil {
		return err
	}

	packetConfig := &ClusterInfo{}
	yaml.Unmarshal([]byte(machineSpec.Platform.Config), packetConfig)

	// Here we pull down the userdata config map and add the install section to the end if it's defined in the machine
	udConfigMap, err := utils.FetchConfigMap(cluster, machine, clientset)
	if err != nil {
		return err
	}
	udStruct := &Userdata{}
	yaml.Unmarshal([]byte(udConfigMap.Data["userdata"]), udStruct)

	if packetConfig.Instances.Install != nil {
		udStruct.Install = packetConfig.Instances.Install
	}

	//Add network tweaks for elastic IPs to userdata
	var floatingIP string
	isMaster := strings.Contains(machine.ObjectMeta.Name, "master")
	if isMaster {
		nameSlice := strings.Split(machine.ObjectMeta.Name, "-")
		indexString := nameSlice[len(nameSlice)-1]
		index, err := strconv.Atoi(indexString)
		if err != nil {
			return err
		}
		if index > len(clusterSpec.Masters.IPs) {
			return errors.New("No available floating IP")
		}
		floatingIP = clusterSpec.Masters.IPs[index]

		//Haxx on haxx b/c dhcp value needs to be a bool so the whole networking.os.devices block needs to be a map of interfaces
		if udStruct.Networking == nil {
			udStruct.Networking = &Network{OS: &OS{Devices: make([]map[string]interface{}, 0)}}
		}
		udStruct.Networking.OS.Devices = append(udStruct.Networking.OS.Devices, map[string]interface{}{"interface": "lo", "cidr": floatingIP + "/32"})
		udStruct.Networking.OS.Devices = append(udStruct.Networking.OS.Devices, map[string]interface{}{"interface": "eth0", "dhcp": true})
	}

	udBytes, err := yaml.Marshal(udStruct)
	if err != nil {
		return err
	}
	//TODO(rsmitty): Shebang no longer needed once talos alpha 28 is cut.
	ud := "#!talos\n" + string(udBytes)

	devCreateReq := &packngo.DeviceCreateRequest{
		Hostname:      machine.ObjectMeta.Name,
		Plan:          packetConfig.Instances.Plan,
		Facility:      []string{packetConfig.Instances.Facility},
		OS:            "custom_ipxe",
		BillingCycle:  "hourly",
		ProjectID:     packetConfig.ProjectID,
		UserData:      ud,
		IPXEScriptURL: packetConfig.Instances.PXEUrl,
	}

	dev, _, err := packet.client.Devices.Create(devCreateReq)
	if err != nil {
		return err
	}

	//Wait for masters to be active, attach floating ip
	if isMaster {
		err = packet.waitForStatus(machine, "active")
		if err != nil {
			return err
		}
		ipReq := &packngo.AddressStruct{Address: floatingIP + "/32"}
		_, _, err = packet.client.DeviceIPs.Assign(dev.ID, ipReq)
		if err != nil {
			return err
		}
	}

	log.Println("[Packet] Instance created with id: " + dev.ID)

	return nil
}

//Update updates a given Packet instance.
func (packet *Packet) Update(cluster *clusterv1.Cluster, machine *clusterv1.Machine, clientset *kubernetes.Clientset) error {
	return nil
}

// Delete deletes a Packet instance.
func (packet *Packet) Delete(cluster *clusterv1.Cluster, machine *clusterv1.Machine, clientset *kubernetes.Clientset) error {

	dev, err := packet.fetchDevice(machine)
	if err != nil {
		return err
	}

	if dev != nil {
		_, err = packet.client.Devices.Delete(dev.ID)
		if err != nil {
			return err
		}
	}
	return nil
}

// Exists returns whether or not an instance is present in AWS.
func (packet *Packet) Exists(cluster *clusterv1.Cluster, machine *clusterv1.Machine, clientset *kubernetes.Clientset) (bool, error) {

	dev, err := packet.fetchDevice(machine)
	if err != nil {
		return false, err
	}

	if dev == nil {
		return false, nil
	}
	return true, nil
}

func (packet *Packet) fetchDevice(machine *clusterv1.Machine) (*packngo.Device, error) {
	machineSpec, err := utils.MachineProviderFromSpec(machine.Spec.ProviderSpec)
	if err != nil {
		return nil, err
	}
	packetConfig := &ClusterInfo{}
	yaml.Unmarshal([]byte(machineSpec.Platform.Config), packetConfig)

	devList, _, err := packet.client.Devices.List(packetConfig.ProjectID, &packngo.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, dev := range devList {
		if dev.Hostname == machine.ObjectMeta.Name {
			return &dev, nil
		}
	}
	return nil, nil
}

//waitForStatus polls the Packet api for a certain instance status
//needed for attaching elastic IP after boot
func (packet *Packet) waitForStatus(machine *clusterv1.Machine, desiredState string) error {

	timeout := time.After(600 * time.Second)
	tick := time.Tick(3 * time.Second)

	for {
		select {
		case <-timeout:
			return errors.New("[Packet] Timed out waiting for running instance")
		case <-tick:
			dev, err := packet.fetchDevice(machine)
			if err != nil {
				return err
			}
			if dev.State == desiredState {
				return nil
			}
		}
	}
}
