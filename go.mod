module github.com/talos-systems/cluster-api-provider-talos

go 1.12

require (
	github.com/Azure/azure-sdk-for-go v34.0.0+incompatible
	github.com/Azure/go-autorest/autorest v0.9.1
	github.com/Azure/go-autorest/autorest/azure/auth v0.3.0
	github.com/Azure/go-autorest/autorest/to v0.3.0
	github.com/Azure/go-autorest/autorest/validation v0.2.0 // indirect
	github.com/aws/aws-sdk-go v1.25.8
	github.com/onsi/gomega v1.5.0
	github.com/packethost/packngo v0.2.0
	github.com/talos-systems/talos v0.3.0-alpha.0.0.20191009201711-edc21ea9109e
	golang.org/x/net v0.0.0-20190813141303-74dc4d7220e7
	google.golang.org/api v0.4.0
	gopkg.in/yaml.v2 v2.2.2
	k8s.io/api v0.0.0-20190918155943-95b840bb6a1f
	k8s.io/apimachinery v0.0.0-20190913080033-27d36303b655
	k8s.io/client-go v10.0.0+incompatible
	k8s.io/code-generator v0.0.0-20181117043124-c2090bec4d9b
	sigs.k8s.io/cluster-api v0.1.9
	sigs.k8s.io/controller-runtime v0.1.12
	sigs.k8s.io/controller-tools v0.1.11
	sigs.k8s.io/testing_frameworks v0.1.1
)

replace (
	git.apache.org/thrift.git => github.com/apache/thrift v0.12.0
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v12.4.1+incompatible
	github.com/kubernetes-incubator/bootkube => github.com/andrewrynhard/bootkube v0.14.1-0.20191009160759-890e418c7b1d
	k8s.io/api => k8s.io/api v0.0.0-20190222213804-5cb15d344471
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20190228180357-d002e88f6236
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190221213512-86fb29eff628
	k8s.io/apiserver => k8s.io/apiserver v0.0.0-20190122042701-b6aa1175dafa
	k8s.io/client-go => k8s.io/client-go v10.0.0+incompatible
	k8s.io/cri-api => k8s.io/cri-api v0.0.0-20190820110325-95eec93e2395
)
