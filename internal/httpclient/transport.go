package httpclient

import (
	"crypto/tls"
	"net/http"
)

// newBaseTransport clones the default transport, optionally disabling TLS
// verification. --insecure is intended only for self-hosted/proxy debugging and
// the command layer prints a loud warning when it is set.
func newBaseTransport(insecure bool) http.RoundTripper {
	t := http.DefaultTransport.(*http.Transport).Clone()
	if insecure {
		// Reachable only via the opt-in --insecure flag (self-hosted/proxy
		// debugging), for which the command layer prints a loud warning.
		if t.TLSClientConfig == nil {
			t.TLSClientConfig = &tls.Config{}
		}
		t.TLSClientConfig.InsecureSkipVerify = true
	}
	return t
}
