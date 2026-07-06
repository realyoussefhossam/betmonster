package oddsfeed

import (
	"context"
	"log/slog"
	"time"
)

type Scheduler struct {
	service   *Service
	interval  time.Duration
	logger    *slog.Logger
	providers []string
}

func NewScheduler(service *Service, providers []string, interval time.Duration, logger *slog.Logger) *Scheduler {
	if interval <= 0 {
		interval = time.Minute
	}
	return &Scheduler{service: service, providers: providers, interval: interval, logger: logger}
}

func (sch *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(sch.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, p := range sch.providers {
				if err := sch.service.SyncProvider(ctx, p); err != nil {
					sch.logger.Error("sync provider failed", slog.String("provider", p), slog.String("error", err.Error()))
				}
			}
		}
	}
}
