package network

import (
	"context"
	"net"
	"net/http"
	"time"
)

// NativeDependencies returns real system resolvers, TCP dialer, HTTP client and
// the platform ICMP implementation. Nothing here changes system state.
func NativeDependencies() Dependencies {
	return Dependencies{
		Resolver:       systemResolver{},
		ServerResolver: serverResolver{},
		Dialer:         &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 15 * time.Second},
		Pinger:         newPinger(),
		HTTP: &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(request *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return http.ErrUseLastResponse
				}
				return nil
			},
		},
	}
}

type systemResolver struct{}

func (systemResolver) LookupHost(ctx context.Context, host string) ([]string, error) {
	return net.DefaultResolver.LookupHost(ctx, host)
}

type serverResolver struct{}

// LookupWithServer sends the query directly to the configured DNS server over
// UDP, so the check validates the specific server rather than the system cache.
func (serverResolver) LookupWithServer(ctx context.Context, server, host string) ([]string, error) {
	address := net.JoinHostPort(server, "53")
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "udp", address)
		},
	}
	return resolver.LookupHost(ctx, host)
}
