package util

import "net"

func GetIPV4() string {
	ips := allIPV4()
	if ips == nil || len(ips) == 0 {
		return ""
	}
	return ips[0]
}

func allIPV4() (ipv4s []string) {
	adders, err := net.InterfaceAddrs()
	if err != nil {
		return
	}
	for _, addr := range adders {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				ipv4 := ipNet.IP.String()
				if ipv4 == "127.0.0.1" || ipv4 == "localhost" {
					continue
				}
				ipv4s = append(ipv4s, ipv4)
			}
		}
	}
	return
}
