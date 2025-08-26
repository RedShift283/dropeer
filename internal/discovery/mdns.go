package discovery

import (
	"context"
	"fmt"
	"log"
	"time"

	"dropeer/internal/common"

	"github.com/grandcat/zeroconf"
)

// PublishService publishes the tracker service on the network.
func PublishService(port int) (*zeroconf.Server, error) {
	server, err := zeroconf.Register("GoP2PLAN-Tracker", common.ServiceName, common.ServiceDomain, port, []string{"txtv=0", "lo=1", "la=2"}, nil)
	if err != nil {
		return nil, fmt.Errorf("could not register service: %w", err)
	}
	return server, nil
}

// DiscoverTracker finds the tracker on the LAN using mDNS.
func DiscoverTracker() (string, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return "", fmt.Errorf("failed to initialize resolver: %w", err)
	}

	entries := make(chan *zeroconf.ServiceEntry)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func(results <-chan *zeroconf.ServiceEntry) {
		for entry := range results {
			log.Printf("Discovered tracker: %s at %s:%d", entry.Instance, entry.AddrIPv4[0], entry.Port)
			entries <- entry
		}
	}(entries)

	if err := resolver.Browse(ctx, common.ServiceName, common.ServiceDomain, entries); err != nil {
		return "", fmt.Errorf("failed to browse: %w", err)
	}

	select {
	case <-ctx.Done():
		return "", fmt.Errorf("tracker discovery timed out")
	case entry := <-entries:
		if len(entry.AddrIPv4) == 0 {
			return "", fmt.Errorf("discovered tracker but no IPv4 address found")
		}
		return fmt.Sprintf("http://%s:%d", entry.AddrIPv4[0].String(), entry.Port), nil
	}
}
