package azuro

import (
	"context"
	"fmt"

	"github.com/realyoussefhossam/betmonster/internal/oddsfeed"
)

const ProviderName = "azuro"

type Provider struct {
	graphURL string
	wsURL    string
}

func New(graphURL, wsURL string) *Provider { return &Provider{graphURL: graphURL, wsURL: wsURL} }

func (p *Provider) Name() string { return ProviderName }

func (p *Provider) FetchSnapshot(ctx context.Context, sport string, params map[string]string) (*oddsfeed.Snapshot, error) {
	if p.graphURL == "" {
		return nil, fmt.Errorf("azuro graph URL not configured")
	}
	// TODO: query Azuro Graph API and normalize into oddsfeed.Snapshot.
	return &oddsfeed.Snapshot{Provider: ProviderName}, nil
}

func (p *Provider) SubscribeLive(ctx context.Context, sport string, updates chan<- oddsfeed.Update) error {
	if p.wsURL == "" {
		return fmt.Errorf("azuro websocket URL not configured")
	}
	// TODO: connect to Azuro WebSocket and push oddsfeed.Update messages.
	<-ctx.Done()
	return ctx.Err()
}

func (p *Provider) ValidateConfig(cfg oddsfeed.ProviderConfig) error {
	if cfg.GraphURL == "" {
		return fmt.Errorf("graph URL required")
	}
	return nil
}
