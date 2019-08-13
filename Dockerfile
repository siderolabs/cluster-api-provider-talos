ARG KUBEBUILDER_VERSION=1.0.8
ARG KUSTOMIZE_VERSION=1.0.11

FROM golang:1.10.3 as vendor
COPY ./ /go/src/github.com/talos-systems/cluster-api-provider-talos/
RUN go get github.com/golang/dep/cmd/dep
WORKDIR /go/src/github.com/talos-systems/cluster-api-provider-talos
RUN dep ensure -v -vendor-only

FROM vendor AS generate
RUN go generate ./pkg/... ./cmd/...

FROM generate AS test
RUN mkdir -p /usr/local/kubebuilder/bin
ARG KUBEBUILDER_VERSION
RUN curl -L https://github.com/kubernetes-sigs/kubebuilder/releases/download/v${KUBEBUILDER_VERSION}/kubebuilder_${KUBEBUILDER_VERSION}_linux_amd64.tar.gz | tar -xvz --strip-components=2 -C /usr/local/kubebuilder/bin
RUN go fmt ./pkg/... ./cmd/...
RUN go vet ./pkg/... ./cmd/...
#TODO: Troubleshoot issues here
#RUN go test ./pkg/... ./cmd/... -coverprofile cover.out

FROM test AS manifests
ARG IMG
ARG KUSTOMIZE_VERSION
RUN mkdir -p /tmp/manifests
RUN wget -O /usr/local/bin/kustomize https://github.com/kubernetes-sigs/kustomize/releases/download/v${KUSTOMIZE_VERSION}/kustomize_${KUSTOMIZE_VERSION}_linux_amd64
RUN chmod +x /usr/local/bin/kustomize
RUN	go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go all
RUN	sed -i'' -e 's@image: .*@image: '"${IMG}"'@' ./config/default/manager_image_patch.yaml
RUN	sed -i'' -e 's@^- manager_auth_proxy_patch.yaml.*@#&@' config/default/kustomization.yaml
RUN	kustomize build config/default/ > /tmp/manifests/provider-components.yaml
RUN	echo "---" >> /tmp/manifests/provider-components.yaml
RUN	kustomize build vendor/sigs.k8s.io/cluster-api/config/default/ >> /tmp/manifests/provider-components.yaml

# Build the manager binary
FROM manifests AS build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager github.com/talos-systems/cluster-api-provider-talos/cmd/manager

# Copy the controller-manager into a thin image
FROM scratch
WORKDIR /
COPY --from=build /go/src/github.com/talos-systems/cluster-api-provider-talos/manager .
ENTRYPOINT ["/manager"]
