module github.com/redhat-appstudio/service-provider-integration-operator

go 1.16

require (
	github.com/google/go-cmp v0.5.6
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.17.0
	github.com/redhat-appstudio/service-provider-integration-oauth v0.0.0
	github.com/stretchr/testify v1.7.0
	k8s.io/api v0.22.3
	k8s.io/apimachinery v0.22.3
	k8s.io/client-go v0.22.3
	sigs.k8s.io/controller-runtime v0.10.3
)

replace github.com/redhat-appstudio/service-provider-integration-oauth => github.com/metlos/service-provider-integration-oauth v0.0.0-20211207004008-6b629d70f06e
