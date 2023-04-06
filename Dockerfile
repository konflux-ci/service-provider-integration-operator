# Build the manager binary
FROM golang:1.20 as builder

WORKDIR /workspace
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
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/ -a ./cmd/operator/operator.go

# Compose the final image of spi-operator.
# !!! This must be last one, because we want simple `docker build .` to build the operator image.
FROM registry.access.redhat.com/ubi8/ubi-minimal:8.7-1107 as spi-operator

# Install the 'shadow-utils' which contains `adduser` and `groupadd` binaries
RUN microdnf install shadow-utils \
	&& groupadd --gid 65532 nonroot \
	&& adduser \
		--no-create-home \
		--no-user-group \
		--uid 65532 \
		--gid 65532 \
		nonroot

WORKDIR /
COPY --from=builder /workspace/bin/operator .
USER 65532:65532

ENTRYPOINT ["/operator"]
