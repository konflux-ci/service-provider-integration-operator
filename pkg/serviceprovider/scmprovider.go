package serviceprovider

//ScmProvider is a marker interface for service providers, indicating that given SP is able to
// perform all listed SCM-specific operations, such as file retrieving or others.
type ScmProvider interface {
	// GetDownloadFileCapability returns download file capability
	GetDownloadFileCapability() DownloadFileCapability
}
