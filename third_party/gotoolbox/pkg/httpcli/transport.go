package httpcli

import (
	"net/http"

	"github.com/wangweihong/gotoolbox/pkg/tls/httptls"
)

func NewTlsClientSkipVerifiedTransport() *http.Transport {
	creds := httptls.NewTlsClientSkipVerifiedCredentials()
	return &http.Transport{
		TLSClientConfig: creds,
	}
}


func NewTlsClientCATransport(ca string) *http.Transport {
	creds ,_:= httptls.NewTlsClientCredentials([]byte(ca))
	return &http.Transport{
		TLSClientConfig:creds,
	}
}
