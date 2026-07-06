package oddsfeed

import (
	"context"
	"log/slog"
	"time"
)

type WebSocketWorker struct {
	service   *Service
	providers map[string]FeedProvider
	logger    *slog.Logger
}

func NewWebSocketWorker(service *Service, providers []FeedProvider, logger *slog.Logger) *WebSocketWorker {
	pm := make(map[string]FeedProvider, len(providers))
	for _, p := range providers {
		pm[p.Name()] = p
	}
	return &WebSocketWorker{service: service, providers: pm, logger: logger}
}

func (w *WebSocketWorker) Start(ctx context.Context) {
	for name, p := range w.providers {
		go w.runProvider(ctx, name, p)
	}
}

func (w *WebSocketWorker) runProvider(ctx context.Context, name string, p FeedProvider) {
	for {
		w.logger.Info("websocket subscribing", slog.String("provider", name))
		updates := make(chan Update, 100)
		if err := p.SubscribeLive(ctx, "", updates); err != nil {
			w.logger.Error("websocket error", slog.String("provider", name), slog.String("error", err.Error()))
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}
