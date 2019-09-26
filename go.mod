module github.com/talos-systems/cluster-api-provider-talos

go 1.12

require (
	github.com/Azure/azure-sdk-for-go v32.5.0+incompatible
	github.com/Azure/go-autorest/autorest v0.9.0
	github.com/Azure/go-autorest/autorest/azure/auth v0.3.0
	github.com/Azure/go-autorest/autorest/to v0.2.0
	github.com/aws/aws-sdk-go v1.20.21
	github.com/onsi/gomega v1.5.0
	github.com/packethost/packngo v0.0.0-20190507131943-1343be729ca2
	github.com/talos-systems/talos v0.3.0-alpha.0.0.20190925181307-607d68008c9b
	golang.org/x/net v0.0.0-20190813141303-74dc4d7220e7
	google.golang.org/api v0.7.0
	gopkg.in/yaml.v2 v2.2.2
	k8s.io/api v0.0.0
	k8s.io/apimachinery v0.0.0
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/code-generator v0.0.0
	sigs.k8s.io/cluster-api v0.1.9
	sigs.k8s.io/controller-runtime v0.1.12
	sigs.k8s.io/controller-tools v0.1.11
	sigs.k8s.io/testing_frameworks v0.1.2-0.20190130140139-57f07443c2d4
)

replace (
	git.apache.org/thrift.git => github.com/apache/thrift v0.12.0
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v12.4.1+incompatible
	k8s.io/api => k8s.io/api v0.0.0-20190222213804-5cb15d344471
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20190228180357-d002e88f6236
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190221213512-86fb29eff628
	k8s.io/apiserver => k8s.io/apiserver v0.0.0-20190122042701-b6aa1175dafa
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.0.0-20190822063658-442a64f3fed7
	k8s.io/client-go => k8s.io/client-go v10.0.0+incompatible
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.0.0-20190822065847-2058b41dfbb6
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.0.0-20190822065536-566e5fc137f7
	k8s.io/code-generator => k8s.io/code-generator v0.0.0-20181117043124-c2090bec4d9b
	k8s.io/component-base => k8s.io/component-base v0.0.0-20190822055535-1f6a258f5d89
	k8s.io/cri-api => k8s.io/cri-api v0.0.0-20190820110325-95eec93e2395
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.0.0-20190822070154-f51cd605b3ee
	k8s.io/klog => k8s.io/klog v0.3.1
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.0.0-20190822061015-a4f93a8219ed
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.0.0-20190822065235-826221481525
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.0.0-20190822064323-7e0495d8a3ff
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.0.0-20190822064931-4470440ed041
	k8s.io/kubectl => k8s.io/kubectl v0.0.0-20190822071625-14af4a63a1e1
	k8s.io/kubelet => k8s.io/kubelet v0.0.0-20190822064626-fa8f3d935631
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.0.0-20190822070624-3a30a18bba71
	k8s.io/metrics => k8s.io/metrics v0.0.0-20190822063337-6c03eb8600ee
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.0.0-20190822061642-ab22eab63834
	k8s.io/utils => k8s.io/utils v0.0.0-20190801114015-581e00157fb1
)
