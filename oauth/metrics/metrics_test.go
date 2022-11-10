package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	prometheusTest "github.com/prometheus/client_golang/prometheus/testutil"
)

func OkHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func TestMetricRequestTotal(t *testing.T) {
	reg := prometheus.NewRegistry()

	// Create a request to pass to our handler.
	req, err := http.NewRequest("GET", "github/authenticate?state=eyJ0b2tlbk5hbWUiOiJnZW5lcmF0ZWQtc3BpLWFjY2Vzcy10b2tlbi1rNHByaiIsInRva2VuTmFtZXNwYWNlIjoiZGVmYXVsdCIsInRva2VuS2NwV29ya3NwYWNlIjoiIiwic2NvcGVzIjpbInJlcG8iXSwic2VydmljZVByb3ZpZGVyVHlwZSI6IkdpdEh1YiIsInNlcnZpY2VQcm92aWRlclVybCI6Imh0dHBzOi8vZ2l0aHViLmNvbSJ9", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/107.0.0.0 Safari/537.36")

	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := OAuthServiceInstrumentMetricHandler(reg, http.HandlerFunc(OkHandler))

	// Our handlers satisfy http.Handler, so we can call their ServeHTTP method
	// directly and pass in our Request and ResponseRecorder.
	handler.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	expected := `
		# HELP redhat_appstudio_spi_oauth_service_requests_total The request counts to OAuth service categorized by HTTP method status code.
		# TYPE redhat_appstudio_spi_oauth_service_requests_total counter
		redhat_appstudio_spi_oauth_service_requests_total{code="200",method="get"} 1
`

	if err := prometheusTest.GatherAndCompare(reg, strings.NewReader(expected), "redhat_appstudio_spi_oauth_service_requests_total"); err != nil {
		t.Fatal(err)
	}
}
