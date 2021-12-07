package serviceprovider

import "net/http"

type serviceProviderProbe interface {
	Probe(cl *http.Client, url string) (string, error)
}
