package util

import (
	"context"
	"net"
	"net/http"
	"time"
)

func NewUnixHttpClient(sockFile string) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", sockFile)
			},
		},
		Timeout: 30 * time.Second,
	}
}
