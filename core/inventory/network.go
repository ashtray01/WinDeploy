package inventory

import "context"

type AdapterConfig struct {
	Index                 uint32   `json:"index"`
	IPv6Index             uint32   `json:"ipv6_index"`
	ID                    string   `json:"id"`
	Name                  string   `json:"name"`
	Operational           bool     `json:"operational"`
	IPv4Enabled           bool     `json:"ipv4_enabled"`
	DHCPv4Enabled         bool     `json:"dhcp_v4_enabled"`
	DNSSuffix             string   `json:"dns_suffix,omitempty"`
	DNSServers            []string `json:"dns_servers"`
	Gateways              []string `json:"gateways"`
	TransmitBitsPerSecond uint64   `json:"transmit_bits_per_second"`
	ReceiveBitsPerSecond  uint64   `json:"receive_bits_per_second"`
}
type NetworkProvider interface {
	NetworkConfig(context.Context) ([]AdapterConfig, error)
}
