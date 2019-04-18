
# Image URL to use all building/pushing image targets
IMG ?= autonomy/cluster-api-provider-talos:latest
KUBEBUILDER_VER ?= 1.0.8
all: test manager

test-prep:
	wget https://github.com/kubernetes-sigs/kubebuilder/releases/download/v${KUBEBUILDER_VER}/kubebuilder_${KUBEBUILDER_VER}_linux_amd64.tar.gz
	tar -xvf kubebuilder_${KUBEBUILDER_VER}_linux_amd64.tar.gz
	mv kubebuilder_${KUBEBUILDER_VER}_linux_amd64 kubebuilder
	mv kubebuilder /usr/local


# Run tests
test: generate fmt vet
	go test ./pkg/... ./cmd/... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager github.com/talos-systems/cluster-api-provider-talos/cmd/manager

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	go run ./cmd/manager/main.go

# Install CRDs into a cluster
install: manifests
	kubectl apply -f config/crds

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	cat provider-components.yaml | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests:
	go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go all
	sed -i'' -e 's@^- manager_auth_proxy_patch.yaml.*@#&@' config/default/kustomization.yaml
	kustomize build config/default/ > provider-components.yaml
	echo "---" >> provider-components.yaml
	kustomize build vendor/sigs.k8s.io/cluster-api/config/default/ >> provider-components.yaml

# Run go fmt against code
fmt:
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
vet:
	go vet ./pkg/... ./cmd/...

# Generate code
generate:
ifndef GOPATH
	$(error GOPATH not defined, please define GOPATH. Run "go help gopath" to learn more about GOPATH)
endif
	go generate ./pkg/... ./cmd/...

# Build the docker image
docker-build:
	docker build . -t ${IMG}
	
	# @echo "updating kustomize image patch file for manager resource"
	# sed -i'' -e 's@image: .*@image: '"${IMG}"'@' ./config/default/manager_image_patch.yaml

# Push the docker image
docker-push:
	docker push ${IMG}
