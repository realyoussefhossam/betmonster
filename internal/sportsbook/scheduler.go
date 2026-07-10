package sportsbook

import (
	"context"
	"log/slog"
	"time"
)

type Scheduler struct {
	svc      *Service
	interval time.Duration
	logger   *slog.Logger
}

func NewScheduler(svc *Service, interval time.Duration, logger *slog.Logger) *Scheduler {
	if interval <= 0 {
		interval = 60 * time.Second
	}
	return &Scheduler{
		svc:      svc,
		interval: interval,
		logger:   logger,
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Run immediately on start, then on each tick.
	s.runOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

func (s *Scheduler) runOnce(ctx context.Context) {
	if err := s.svc.AutoSettleFromEvents(ctx); err != nil {
		s.logger.Error("auto-settlement failed", slog.String("error", err.Error()))
	}
}
