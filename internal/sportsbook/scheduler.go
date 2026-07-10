package sportsbook

import (
	"context"
	"log/slog"
	"time"
)

const schedulerPageSize = 100

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
	page := 1
	for {
		bets, err := s.svc.ListPendingBets(ctx, page, schedulerPageSize)
		if err != nil {
			s.logger.Error("list pending bets", slog.String("error", err.Error()))
			return
		}
		if len(bets) == 0 {
			return
		}

		for _, bet := range bets {
			outcome, err := s.svc.resolveOutcomeStatus(ctx, bet.EventID, bet.MarketID, bet.OutcomeID)
			if err != nil {
				s.logger.Error("resolve outcome status",
					slog.String("bet_id", bet.ID),
					slog.String("event_id", bet.EventID),
					slog.String("error", err.Error()),
				)
				continue
			}
			if outcome == "" {
				continue
			}
			if _, err := s.svc.SettleBet(ctx, bet.ID, outcome); err != nil {
				s.logger.Error("auto settle bet",
					slog.String("bet_id", bet.ID),
					slog.String("outcome", outcome),
					slog.String("error", err.Error()),
				)
			}
		}

		if len(bets) < schedulerPageSize {
			return
		}
		page++
	}
}
