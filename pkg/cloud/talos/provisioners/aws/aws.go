package aws

import (
	"context"
	"encoding/base64"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	awspkg "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/talos-systems/cluster-api-provider-talos/pkg/cloud/talos/utils"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

// AWS represents a provider for AWS.
type AWS struct {
}

// ClusterInfo holds data about desired config in cluster object
type ClusterInfo struct {
	Region string
}

// MachineInfo holds data about desired config in cluster object
type MachineInfo struct {
	Region    string
	Instances InstanceInfo
}

// InstanceInfo holds data about the instances we'll create
type InstanceInfo struct {
	Type           string
	AMI            string
	Keypair        string
	SecurityGroups []string
	Disks          DiskInfo
}

// DiskInfo holds disk info data
type DiskInfo struct {
	Size int
}

//NewAWS returns an instance of the AWS provisioner
func NewAWS() (*AWS, error) {
	return &AWS{}, nil
}

// Create creates an instance in AWS.
func (aws *AWS) Create(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine, clientset *kubernetes.Clientset) error {

	machineSpec, err := utils.MachineProviderFromSpec(machine.Spec.ProviderSpec)
	if err != nil {
		return err
	}

	awsConfig := &MachineInfo{}
	yaml.Unmarshal([]byte(machineSpec.Platform.Config), awsConfig)

	ec2client, err := client(awsConfig.Region)
	if err != nil {
		return err
	}

	//fetch floating IP if master and userdata based on machine name
	natIP := ""
	if strings.Contains(machine.ObjectMeta.Name, "master") {

		// Find public ip
		address, err := getPublicIPByName(ec2client, machine.ObjectMeta.Name+"-ip")
		if err != nil {
			return err
		}
		if address == nil {
			return errors.New("IP not ready")
		}
		natIP = *address.PublicIp
	}

	udConfigMap, err := utils.FetchConfigMap(cluster, machine, clientset)
	if err != nil {
		return err
	}
	ud := udConfigMap.Data["userdata"]
	udb64 := base64.StdEncoding.EncodeToString([]byte(ud))

	// Create our ec2 instance and wait for it to be running
	instanceInput := &ec2.RunInstancesInput{
		ImageId:      awspkg.String(awsConfig.Instances.AMI),
		InstanceType: awspkg.String(awsConfig.Instances.Type),
		MinCount:     awspkg.Int64(1),
		MaxCount:     awspkg.Int64(1),
		KeyName:      awspkg.String(awsConfig.Instances.Keypair),
		UserData:     awspkg.String(udb64),
	}

	res, err := ec2client.RunInstances(instanceInput)
	if err != nil {
		return err
	}
	instanceID := *res.Instances[0].InstanceId

	//Wait for instance to be running, then fetch and associate pre-existing Elastic IP if needed
	if natIP != "" {
		//TODO: Should probably attempt to call delete if running status fails
		err = waitForStatus(instanceID, "running", ec2client)
		if err != nil {
			return err
		}
		descAddrInput := &ec2.DescribeAddressesInput{PublicIps: []*string{awspkg.String(natIP)}}
		addresses, err := ec2client.DescribeAddresses(descAddrInput)
		if err != nil {
			return err
		}

		assocAddrInput := &ec2.AssociateAddressInput{
			AllocationId:       addresses.Addresses[0].AllocationId,
			NetworkInterfaceId: res.Instances[0].NetworkInterfaces[0].NetworkInterfaceId,
		}
		_, err = ec2client.AssociateAddress(assocAddrInput)
		if err != nil {
			return err
		}
	}

	// Add tags to the created instance
	_, err = ec2client.CreateTags(&ec2.CreateTagsInput{
		Resources: []*string{res.Instances[0].InstanceId},
		Tags: []*ec2.Tag{
			{
				Key:   awspkg.String("Name"),
				Value: awspkg.String(machine.ObjectMeta.Name),
			},
			{
				Key:   awspkg.String("TalosClusterName"),
				Value: awspkg.String(cluster.ObjectMeta.Name),
			},
		},
	})
	if err != nil {
		return err
	}

	log.Println("[AWS] Instance created with ID:" + instanceID)
	return nil
}

//Update updates a given AWS instance.
func (aws *AWS) Update(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine, clientset *kubernetes.Clientset) error {
	return nil
}

// Delete deletes a AWS instance.
func (aws *AWS) Delete(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine, clientset *kubernetes.Clientset) error {

	// Fish out configs and create an ec2 client
	machineSpec, err := utils.MachineProviderFromSpec(machine.Spec.ProviderSpec)
	if err != nil {
		return err
	}

	awsConfig := &MachineInfo{}
	yaml.Unmarshal([]byte(machineSpec.Platform.Config), awsConfig)

	ec2client, err := client(awsConfig.Region)
	if err != nil {
		return err
	}

	//Make sure instance exists before attempting delete
	exists, err := aws.Exists(ctx, cluster, machine, clientset)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	//Find instance id
	id, err := fetchInstanceID(cluster.ObjectMeta.Name, machine.ObjectMeta.Name, ec2client)
	if err != nil {
		return err
	}

	//Stop Instance
	input := &ec2.TerminateInstancesInput{
		InstanceIds: []*string{id},
	}
	_, err = ec2client.TerminateInstances(input)
	if err != nil {
		return err
	}
	return nil
}

// Exists returns whether or not an instance is present in AWS.
func (aws *AWS) Exists(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine, clientset *kubernetes.Clientset) (bool, error) {
	// Fish out configs and create an ec2 client
	machineSpec, err := utils.MachineProviderFromSpec(machine.Spec.ProviderSpec)
	if err != nil {
		return false, err
	}

	awsConfig := &MachineInfo{}
	yaml.Unmarshal([]byte(machineSpec.Platform.Config), awsConfig)

	ec2client, err := client(awsConfig.Region)
	if err != nil {
		return false, err
	}

	id, err := fetchInstanceID(cluster.ObjectMeta.Name, machine.ObjectMeta.Name, ec2client)
	if err != nil {
		return false, err
	}

	if id == nil {
		return false, nil
	}
	return true, nil
}

// AllocateExternalIPs creates IPs for the control plane nodes
func (aws *AWS) AllocateExternalIPs(cluster *clusterv1.Cluster, clientset *kubernetes.Clientset) ([]string, error) {
	// Fish out configs and create an ec2 client
	clusterSpec, err := utils.ClusterProviderFromSpec(cluster.Spec.ProviderSpec)
	if err != nil {
		return nil, err
	}

	awsConfig := &ClusterInfo{}
	yaml.Unmarshal([]byte(clusterSpec.Platform.Config), awsConfig)

	ec2client, err := client(awsConfig.Region)
	if err != nil {
		return nil, err
	}

	floatingIPs := []string{}
	for i := 0; i < clusterSpec.ControlPlane.Count; i++ {

		// Check if ips already exist and add to list early if so
		flip, err := getPublicIPByName(ec2client, cluster.ObjectMeta.Name+"-master-"+strconv.Itoa(i)+"-ip")
		if err != nil {
			return nil, err
		}
		if flip != nil {
			floatingIPs = append(floatingIPs, *flip.PublicIp)
			continue
		}

		// Allocate elastic ip
		allocRes, err := ec2client.AllocateAddress(&ec2.AllocateAddressInput{
			Domain: awspkg.String("vpc"),
		})
		if err != nil {
			return nil, err
		}

		// Add tags to the created instance
		_, err = ec2client.CreateTags(&ec2.CreateTagsInput{
			Resources: []*string{allocRes.AllocationId},
			Tags: []*ec2.Tag{
				{
					Key:   awspkg.String("Name"),
					Value: awspkg.String(cluster.ObjectMeta.Name + "-master-" + strconv.Itoa(i) + "-ip"),
				},
			},
		})
		if err != nil {
			return nil, err
		}

		floatingIPs = append(floatingIPs, *allocRes.PublicIp)
	}

	return floatingIPs, nil
}

// DeAllocateExternalIPs cleans IPs for the control plane nodes
func (aws *AWS) DeAllocateExternalIPs(cluster *clusterv1.Cluster, clientset *kubernetes.Clientset) error {
	// Fish out configs and create an ec2 client
	clusterSpec, err := utils.ClusterProviderFromSpec(cluster.Spec.ProviderSpec)
	if err != nil {
		return err
	}

	awsConfig := &ClusterInfo{}
	yaml.Unmarshal([]byte(clusterSpec.Platform.Config), awsConfig)

	ec2client, err := client(awsConfig.Region)
	if err != nil {
		return err
	}

	for i := 0; i < clusterSpec.ControlPlane.Count; i++ {

		// Check if ips already exist and add to list early if so
		flip, err := getPublicIPByName(ec2client, cluster.ObjectMeta.Name+"-master-"+strconv.Itoa(i)+"-ip")
		if err != nil {
			return err
		}

		// Ignore if not found
		if flip == nil {
			return nil
		}

		_, err = ec2client.ReleaseAddress(&ec2.ReleaseAddressInput{
			AllocationId: flip.AllocationId,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// getPublicIPbyName finds the public IP object from a list of all IP objects
func getPublicIPByName(ec2client *ec2.EC2, name string) (*ec2.Address, error) {
	result, err := ec2client.DescribeAddresses(&ec2.DescribeAddressesInput{
		Filters: []*ec2.Filter{
			{
				Name:   awspkg.String("tag:Name"),
				Values: awspkg.StringSlice([]string{name}),
			},
		},
	})
	if err != nil {
		return nil, err
	}

	if len(result.Addresses) > 0 {
		return result.Addresses[0], nil
	}

	// Not found
	return nil, nil
}

// client generates an ec2 client to use
func client(region string) (*ec2.EC2, error) {
	sess, err := session.NewSession(&awspkg.Config{
		Region: awspkg.String(region)},
	)
	if err != nil {
		return nil, err
	}

	ec2client := ec2.New(sess)
	return ec2client, nil
}

//fetchInstance ID searches AWS for instance name and the cluster name tags that we add during instance creation. Returns the instance ID.
func fetchInstanceID(clusterName string, instanceName string, client *ec2.EC2) (*string, error) {

	instanceFilters := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			&ec2.Filter{
				Name: awspkg.String("tag:Name"),
				Values: []*string{
					awspkg.String(instanceName),
				},
			},
			&ec2.Filter{
				Name: awspkg.String("tag:TalosClusterName"),
				Values: []*string{
					awspkg.String(clusterName),
				},
			},
			&ec2.Filter{
				Name:   awspkg.String("instance-state-name"),
				Values: []*string{awspkg.String("running"), awspkg.String("pending"), awspkg.String("stopped")},
			},
		},
	}
	res, err := client.DescribeInstances(instanceFilters)
	if err != nil {
		return nil, err
	}
	if len(res.Reservations) == 0 {
		return nil, nil
	}
	if len(res.Reservations[0].Instances) > 1 {
		return nil, errors.New("[AWS] Multiple instances with same filter info")
	}

	return res.Reservations[0].Instances[0].InstanceId, nil

}

//waitForStatus polls the AWS api for a certain instance status
//needed for attaching elastic IP after boot
func waitForStatus(instanceID string, desiredState string, client *ec2.EC2) error {

	timeout := time.After(120 * time.Second)
	tick := time.Tick(3 * time.Second)

	for {
		select {
		case <-timeout:
			return errors.New("[AWS] Timed out waiting for running instance: " + instanceID)
		case <-tick:
			instanceInput := &ec2.DescribeInstancesInput{
				InstanceIds: []*string{awspkg.String(instanceID)},
			}
			res, err := client.DescribeInstances(instanceInput)
			if err != nil {
				return err
			}

			if *res.Reservations[0].Instances[0].State.Name == desiredState {
				return nil
			}
		}
	}
}
