TAG = $(shell gitmeta image tag)
REPO = autonomy/cluster-api-provider-talos
# Image URL to use all building/pushing image targets
IMG ?= $(REPO):$(TAG)

all: test manager

# Run tests
test:
	docker build . --target $@ -t $(REPO):test

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager github.com/talos-systems/cluster-api-provider-talos/cmd/manager

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate
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

# Generate code
generate:
ifndef GOPATH
	$(error GOPATH not defined, please define GOPATH. Run "go help gopath" to learn more about GOPATH)
endif
	go generate ./pkg/... ./cmd/...

# Build the docker image
docker-build:
	docker build . -t $(IMG)

	# @echo "updating kustomize image patch file for manager resource"
	# sed -i'' -e 's@image: .*@image: '"${IMG}"'@' ./config/default/manager_image_patch.yaml

# Push the docker image
docker-push:
	@docker tag $(REPO):$(TAG) $(REPO):latest
	@docker push $(REPO):$(TAG)
	@docker push $(REPO):latest

.PHONY: login
login:
	@docker login --username "$(DOCKER_USERNAME)" --password "$(DOCKER_PASSWORD)"
