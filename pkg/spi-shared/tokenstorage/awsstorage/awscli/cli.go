package awscli

type AWSCliArgs struct {
	ConfigFile      string `arg:"--aws-config-filepath, env: AWS_CONFIG_FILE" default:"/etc/spi/aws/config" help:""`
	CredentialsFile string `arg:"--aws-credentials-filepath, env: AWS_CREDENTIALS_FILE" default:"/etc/spi/aws/credentials" help:""`
}
