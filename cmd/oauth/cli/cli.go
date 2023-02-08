package cli

import (
	"github.com/redhat-appstudio/service-provider-integration-operator/cmd"
)

type OAuthServiceCliArgs struct {
	cmd.CommonCliArgs
	cmd.LoggingCliArgs
	ServiceAddr     string `arg:"--service-addr, env" default:"0.0.0.0:8000" help:"Service address to listen on"`
	AllowedOrigins  string `arg:"--allowed-origins, env" default:"https://console.dev.redhat.com,https://prod.foo.redhat.com:1337" help:"Comma-separated list of domains allowed for cross-domain requests"`
	KubeConfig      string `arg:"--kubeconfig, env" default:"" help:""`
	KubeInsecureTLS bool   `arg:"--kube-insecure-tls, env" default:"false" help:"Whether is allowed or not insecure kubernetes tls connection."`
	ApiServer       string `arg:"--api-server, env:API_SERVER" default:"" help:"host:port of the Kubernetes API server to use when handling HTTP requests"`
	ApiServerCAPath string `arg:"--ca-path, env:API_SERVER_CA_PATH" default:"" help:"the path to the CA certificate to use when connecting to the Kubernetes API server"`
}
