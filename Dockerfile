ARG KUBEBUILDER_VERSION=1.0.8

# Build the manager binary
FROM golang:1.10.3 as vendor
RUN go get github.com/golang/dep/cmd/dep
WORKDIR /go/src/github.com/talos-systems/cluster-api-provider-talos
COPY Gopkg.lock .
COPY Gopkg.toml .
RUN dep ensure -v -vendor-only

FROM vendor AS generate
COPY config/    config/
COPY hack/    hack/
COPY pkg/    pkg/
COPY cmd/    cmd/
RUN go generate ./pkg/... ./cmd/...

FROM generate AS test
RUN mkdir -p /usr/local/kubebuilder/bin
ARG KUBEBUILDER_VERSION
RUN curl -L https://github.com/kubernetes-sigs/kubebuilder/releases/download/v${KUBEBUILDER_VERSION}/kubebuilder_${KUBEBUILDER_VERSION}_linux_amd64.tar.gz | tar -xvz --strip-components=2 -C /usr/local/kubebuilder/bin
RUN go fmt ./pkg/... ./cmd/...
RUN go vet ./pkg/... ./cmd/...
RUN go test ./pkg/... ./cmd/... -coverprofile cover.out

# Build the manager binary
FROM generate AS build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager github.com/talos-systems/cluster-api-provider-talos/cmd/manager

# Copy the controller-manager into a thin image
FROM scratch
WORKDIR /
COPY --from=build /go/src/github.com/talos-systems/cluster-api-provider-talos/manager .
ENTRYPOINT ["/manager"]
