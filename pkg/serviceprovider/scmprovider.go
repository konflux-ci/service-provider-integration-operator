package serviceprovider

//ScmProvider is a marker interface for service providers, indicating that given SP is able to
// perform an SCM-specific operations, such as file retrieving etc.
type ScmProvider interface {
	// GetDownloadFileCapability non-nil returned value indicates given SCM provider is able to perform file download operation
	GetDownloadFileCapability() DownloadFileCapability
}
