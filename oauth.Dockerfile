# Build the manager binary
FROM golang:1.20 as builder
ARG TARGETOS
ARG TARGETARCH

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
COPY static/ static/
COPY oauth/ oauth/


# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o bin/ -a ./cmd/oauth/oauth.go

# Compose the final image of spi-oauth service
FROM registry.access.redhat.com/ubi9/ubi-minimal:9.2-750 as spi-oauth

# Install the 'shadow-utils' which contains `adduser` and `groupadd` binaries
RUN microdnf -y install shadow-utils \
	&& groupadd --gid 65532 nonroot \
	&& adduser \
		--no-create-home \
		--no-user-group \
		--uid 65532 \
		--gid 65532 \
		nonroot

WORKDIR /

COPY --from=builder /workspace/bin/oauth /spi-oauth
COPY --from=builder /workspace/static/callback_success.html /static/callback_success.html
COPY --from=builder /workspace/static/callback_error.html /static/callback_error.html
COPY --from=builder /workspace/static/redirect_notice.html /static/redirect_notice.html

USER 65532:65532

ENTRYPOINT ["/spi-oauth"]
