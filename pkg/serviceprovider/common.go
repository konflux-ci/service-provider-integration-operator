package serviceprovider

import "net/url"

func getHostWithScheme(repoUrl string) (string, error) {
	u, err := url.Parse(repoUrl)
	if err != nil {
		return "", err
	}
	return u.Scheme + "://" + u.Host, nil
}
