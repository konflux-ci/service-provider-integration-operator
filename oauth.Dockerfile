# Build the manager binary
FROM registry.access.redhat.com/ubi9/go-toolset:1.20.12 as builder
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
COPY static/ static/
COPY oauth/ oauth/

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o bin/oauth -a ./cmd/oauth/oauth.go

FROM registry.access.redhat.com/ubi9/ubi-minimal:9.4-1194 as spi-oauth

WORKDIR /

COPY --from=builder /opt/app-root/src/bin/oauth /spi-oauth
COPY --from=builder /opt/app-root/src/static/callback_success.html /static/callback_success.html
COPY --from=builder /opt/app-root/src/static/callback_error.html /static/callback_error.html
COPY --from=builder /opt/app-root/src/static/redirect_notice.html /static/redirect_notice.html

# It is mandatory to set these labels
LABEL description="RHTAP SPI OAuth service"
LABEL io.k8s.description="RHTAP OAuth service"
LABEL io.k8s.display-name="spi-oauth-service"
LABEL summary="RHTAP SPI OAuth service"
LABEL io.openshift.tags="rhtap"
LABEL com.redhat.component="spi-oauth-container"
LABEL name="spi-oauth"
USER 65532:65532

ENTRYPOINT ["/spi-oauth"]
