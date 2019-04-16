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
	stdlibx509 "crypto/x509"
	"encoding/base64"
	"encoding/pem"
	goerrors "errors"
	"net"
	"strconv"
	"strings"

	"github.com/talos-systems/cluster-api-provider-talos/pkg/cloud/talos/userdata"
	"github.com/talos-systems/cluster-api-provider-talos/pkg/cloud/talos/utils"
	"github.com/talos-systems/talos/pkg/crypto/x509"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// ClusterActuator is responsible for performing machine reconciliation
type ClusterActuator struct {
}

// ClusterActuatorParams holds parameter information for Actuator
type ClusterActuatorParams struct {
}

// NewClusterActuator creates a new Actuator
func NewClusterActuator(mgr manager.Manager, params ClusterActuatorParams) (*ClusterActuator, error) {
	return &ClusterActuator{}, nil
}

// Reconcile reconciles a cluster and is invoked by the Cluster Controller
// TODO: This needs to be idempotent. Check if these certs and stuff already exist
func (a *ClusterActuator) Reconcile(cluster *clusterv1.Cluster) error {

	// creates the clientset
	//It feels like I may want a global k8s client set. We use it everywhere in both actuators
	clientset, err := utils.CreateK8sClientSet()
	if err != nil {
		return err
	}

	//Gen tokens for kubeadm
	kubeadmTokens := &userdata.KubeadmTokens{
		BootstrapToken: utils.RandomString(6) + "." + utils.RandomString(16),
		CertKey:        utils.RandomString(26),
	}

	//Gen user/pass for trustd
	trustdInfo := &userdata.TrustdInfo{
		Username: utils.RandomString(14),
		Password: utils.RandomString(24),
	}

	err = createMasterConfigMaps(cluster, clientset, kubeadmTokens, trustdInfo)
	if err != nil {
		return err
	}
	err = createWorkerConfigMaps(cluster, clientset, kubeadmTokens, trustdInfo)
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

//createMasterConfigMaps generates certs and creates configmaps that define the userdata for each node
func createMasterConfigMaps(cluster *clusterv1.Cluster, clientset *kubernetes.Clientset, kubeadmTokens *userdata.KubeadmTokens, trustdInfo *userdata.TrustdInfo) error {

	spec, err := utils.ClusterProviderFromSpec(cluster.Spec.ProviderSpec)
	if err != nil {
		return err
	}

	//Gen k8s certs
	opts := []x509.Option{x509.RSA(true), x509.Organization("talos-k8s")}
	k8sCert, err := x509.NewSelfSignedCertificateAuthority(opts...)
	if err != nil {
		return err
	}

	//Gen os certs
	opts = []x509.Option{x509.RSA(false), x509.Organization("talos-os")}
	osCert, err := x509.NewSelfSignedCertificateAuthority(opts...)
	if err != nil {
		return err
	}

	//Gen admin certs
	adminKey, err := x509.NewKey()
	if err != nil {
		return err
	}
	pemBlock, _ := pem.Decode(adminKey.KeyPEM)
	if pemBlock == nil {
		return goerrors.New("Unable to decode admin key pem")
	}
	adminKeyEC, err := stdlibx509.ParseECPrivateKey(pemBlock.Bytes)
	if err != nil {
		return err
	}
	ips := []net.IP{net.ParseIP("127.0.0.1")}
	opts = []x509.Option{x509.IPAddresses(ips)}
	csr, err := x509.NewCertificateSigningRequest(adminKeyEC, opts...)
	if err != nil {
		return err
	}
	csrPemBlock, _ := pem.Decode(csr.X509CertificateRequestPEM)
	if csrPemBlock == nil {
		return goerrors.New("Unable to decode csr pem")
	}
	ccsr, err := stdlibx509.ParseCertificateRequest(csrPemBlock.Bytes)
	caPemBlock, _ := pem.Decode(osCert.CrtPEM)
	if caPemBlock == nil {
		return goerrors.New("Unable to decode ca cert pem")
	}
	caCrt, err := stdlibx509.ParseCertificate(caPemBlock.Bytes)
	caKeyPemBlock, _ := pem.Decode(osCert.KeyPEM)
	if caKeyPemBlock == nil {
		return goerrors.New("Unable to decode ca key pem")
	}
	caKey, err := stdlibx509.ParseECPrivateKey(caKeyPemBlock.Bytes)
	adminCrt, err := x509.NewCertificateFromCSR(caCrt, caKey, ccsr)
	if err != nil {
		return err
	}

	//Gather up all them there certs
	certs := &userdata.Certs{
		AdminCert: base64.StdEncoding.EncodeToString(adminCrt.X509CertificatePEM),
		AdminKey:  base64.StdEncoding.EncodeToString(adminKey.KeyPEM),
		OsCert:    base64.StdEncoding.EncodeToString(osCert.CrtPEM),
		OsKey:     base64.StdEncoding.EncodeToString(osCert.KeyPEM),
		K8sCert:   base64.StdEncoding.EncodeToString(k8sCert.CrtPEM),
		K8sKey:    base64.StdEncoding.EncodeToString(k8sCert.KeyPEM),
	}

	//For each master, gen userdata and push it into a configmap
	for index := range spec.Masters.IPs {
		cmName := cluster.ObjectMeta.Name + "-master-" + strconv.Itoa(index)

		//Check if cm exists
		cmExists := true
		_, err = clientset.CoreV1().ConfigMaps("cluster-api-provider-talos-system").Get(cmName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			cmExists = false
		}

		udType := "controlplane"
		if index == 0 {
			udType = "init"
		}
		udInput := &userdata.Input{
			UserdataType:  udType,
			Certs:         certs,
			MasterIPs:     spec.Masters.IPs,
			Index:         index,
			ClusterName:   cluster.ObjectMeta.Name,
			ServiceDomain: cluster.Spec.ClusterNetwork.ServiceDomain,
			PodNet:        cluster.Spec.ClusterNetwork.Pods.CIDRBlocks,
			ServiceNet:    cluster.Spec.ClusterNetwork.Services.CIDRBlocks,
			Endpoints:     strings.Join(spec.Masters.IPs[1:], ", "),
			KubeadmTokens: kubeadmTokens,
			TrustdInfo:    trustdInfo,
		}

		ud, err := userdata.GenUdata(udInput)
		if err != nil {
			return err
		}

		adminConf, err := userdata.GenAdmin(udInput)
		if err != nil {
			return err
		}

		cm := &v1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      cmName,
				Namespace: "cluster-api-provider-talos-system",
			},
			Data:       map[string]string{"userdata": ud, "admin.conf": adminConf},
			BinaryData: nil,
		}

		if cmExists {
			_, err = clientset.CoreV1().ConfigMaps("cluster-api-provider-talos-system").Update(cm)
		} else {
			_, err = clientset.CoreV1().ConfigMaps("cluster-api-provider-talos-system").Create(cm)

		}
		if err != nil {
			return err
		}

	}
	return nil
}

//createWorkerConfigMaps creates a configmap for a machineset of workers
func createWorkerConfigMaps(cluster *clusterv1.Cluster, clientset *kubernetes.Clientset, kubeadmTokens *userdata.KubeadmTokens, trustdInfo *userdata.TrustdInfo) error {

	spec, err := utils.ClusterProviderFromSpec(cluster.Spec.ProviderSpec)
	if err != nil {
		return err
	}

	cmName := cluster.ObjectMeta.Name + "-workers"

	//Check if cm exists
	cmExists := true
	_, err = clientset.CoreV1().ConfigMaps("cluster-api-provider-talos-system").Get(cmName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		cmExists = false
	}

	udInput := &userdata.Input{
		UserdataType:  "worker",
		MasterIPs:     spec.Masters.IPs,
		KubeadmTokens: kubeadmTokens,
		TrustdInfo:    trustdInfo,
	}

	ud, err := userdata.GenUdata(udInput)
	if err != nil {
		return err
	}

	cm := &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: "cluster-api-provider-talos-system",
		},
		Data:       map[string]string{"userdata": ud},
		BinaryData: nil,
	}

	if cmExists {
		_, err = clientset.CoreV1().ConfigMaps("cluster-api-provider-talos-system").Update(cm)
	} else {
		_, err = clientset.CoreV1().ConfigMaps("cluster-api-provider-talos-system").Create(cm)

	}
	if err != nil {
		return err
	}

	return nil
}

//deleteConfigMaps cleans up all configmaps associated with this cluster
func deleteConfigMaps(cluster *clusterv1.Cluster, clientset *kubernetes.Clientset) error {
	spec, err := utils.ClusterProviderFromSpec(cluster.Spec.ProviderSpec)
	if err != nil {
		return err
	}

	for index := range spec.Masters.IPs {
		cmName := cluster.ObjectMeta.Name + "-master-" + strconv.Itoa(index)
		err = clientset.CoreV1().ConfigMaps("cluster-api-provider-talos-system").Delete(cmName, nil)
		if err != nil {
			return err
		}
	}

	err = clientset.CoreV1().ConfigMaps("cluster-api-provider-talos-system").Delete(cluster.ObjectMeta.Name+"-workers", nil)
	if err != nil {
		return err
	}

	return nil
}
