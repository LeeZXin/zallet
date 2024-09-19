package process

import (
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type ProbeType string

const (
	HttpProbeType ProbeType = "http"
	TcpProbeType  ProbeType = "tcp"
)

type TcpProbe struct {
	Host string `json:"host" yaml:"host"`
}

func (t *TcpProbe) IsValid() bool {
	return regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}:\d+$`).MatchString(t.Host)
}

type HttpProbe struct {
	Url string `json:"url" yaml:"url"`
}

func (t *HttpProbe) IsValid() bool {
	parsed, err := url.Parse(t.Url)
	if err != nil {
		return false
	}
	return strings.HasPrefix(parsed.Scheme, "http")
}

type Probe struct {
	Delay    string     `json:"delay" yaml:"delay"`
	Interval string     `json:"interval" yaml:"interval"`
	Type     ProbeType  `json:"type" yaml:"type"`
	Tcp      *TcpProbe  `json:"tcp,omitempty" yaml:"tcp,omitempty"`
	Http     *HttpProbe `json:"http,omitempty" yaml:"http,omitempty"`
}

func (p *Probe) IsValid() bool {
	switch p.Type {
	case HttpProbeType:
		return p.Http != nil && p.Http.IsValid()
	case TcpProbeType:
		return p.Tcp != nil && p.Tcp.IsValid()
	default:
		return false
	}
}

func (p *Probe) run() bool {
	switch p.Type {
	case HttpProbeType:
		if p.Http != nil {
			resp, err := http.Get(p.Http.Url)
			if err != nil {
				return false
			}
			defer resp.Body.Close()
			return resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusBadRequest
		}
	case TcpProbeType:
		if p.Tcp != nil {
			conn, err := net.DialTimeout("tcp", p.Tcp.Host, time.Second)
			if err != nil {
				return false
			}
			defer conn.Close()
			return true
		}
	}
	return false
}
