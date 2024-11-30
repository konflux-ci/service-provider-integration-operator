# Build the manager binary
FROM registry.access.redhat.com/ubi9/go-toolset:1.21.10 as builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /opt/app-root/src
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/ cmd/
COPY api/ api/
COPY pkg/ pkg/
COPY controllers/ controllers/

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o bin/operator -a ./cmd/operator/operator.go

FROM registry.access.redhat.com/ubi9/ubi-minimal:9.3 as spi-operator

WORKDIR /
COPY --from=builder /opt/app-root/src/bin/operator .
# It is mandatory to set these labels
LABEL description="RHTAP SPI Operator"
LABEL io.k8s.description="RHTAP SPI Operator"
LABEL io.k8s.display-name="spi-operator"
LABEL summary="RHTAP SPI Operator"
LABEL io.openshift.tags="rhtap"
LABEL com.redhat.component="spi-operator-container"
LABEL name="spi-operator"
USER 65532:65532

ENTRYPOINT ["/operator"]
