"use client";

import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "sonner";
import { goApiClient, Bet, Event, Market, Outcome } from "@/lib/go-api-client";

function formatDate(value: string) {
  if (!value) return "-";
  const date = new Date(value);
  return isNaN(date.getTime()) ? value : date.toLocaleString();
}

function getEventLabel(eventId: string, events: Event[]) {
  const event = events.find((e) => e.id === eventId);
  if (!event) return eventId;
  return `${event.homeTeam} vs ${event.awayTeam}`;
}

type SettlementOutcome = "won" | "lost" | "cancelled";

const SELECT_CLASSES =
  "flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2";

interface BetsData {
  bets: Bet[];
  events: Event[];
  marketsMap: Map<string, Market>;
  outcomesMap: Map<string, Outcome>;
  defaultOutcomes: Record<string, SettlementOutcome>;
}

async function fetchBetsData(): Promise<BetsData> {
  const [betsRes, eventsRes] = await Promise.all([
    goApiClient.listBets(),
    goApiClient.listEvents(),
  ]);

  if (betsRes.error || eventsRes.error) {
    throw new Error(
      betsRes.error || eventsRes.error || "Failed to load bets.",
    );
  }

  const loadedBets = betsRes.data?.bets ?? [];
  const loadedEvents = eventsRes.data?.events ?? [];

  const pendingEventIds = Array.from(
    new Set(
      loadedBets.filter((b) => b.status === "pending").map((b) => b.eventId),
    ),
  );

  const nextMarkets = new Map<string, Market>();
  const nextOutcomes = new Map<string, Outcome>();

  if (pendingEventIds.length > 0) {
    const marketResults = await Promise.all(
      pendingEventIds.map((id) => goApiClient.listMarkets(id)),
    );
    marketResults.forEach((res) => {
      if (!res.error) {
        res.data?.markets.forEach((market) => {
          nextMarkets.set(market.id, market);
          market.outcomes.forEach((outcome) => {
            nextOutcomes.set(outcome.id, outcome);
          });
        });
      }
    });
  }

  const defaultOutcomes: Record<string, SettlementOutcome> = {};
  loadedBets
    .filter((b) => b.status === "pending")
    .forEach((b) => {
      defaultOutcomes[b.id] = "won";
    });

  return {
    bets: loadedBets,
    events: loadedEvents,
    marketsMap: nextMarkets,
    outcomesMap: nextOutcomes,
    defaultOutcomes,
  };
}

export default function AdminSettlementsPage() {
  const [bets, setBets] = useState<Bet[]>([]);
  const [events, setEvents] = useState<Event[]>([]);
  const [marketsMap, setMarketsMap] = useState<Map<string, Market>>(new Map());
  const [outcomesMap, setOutcomesMap] = useState<Map<string, Outcome>>(
    new Map(),
  );
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [outcomes, setOutcomes] = useState<Record<string, SettlementOutcome>>(
    {},
  );
  const [settling, setSettling] = useState<Record<string, boolean>>({});

  function applyData(data: BetsData) {
    setBets(data.bets);
    setEvents(data.events);
    setMarketsMap(data.marketsMap);
    setOutcomesMap(data.outcomesMap);
    setOutcomes(data.defaultOutcomes);
  }

  async function load() {
    setLoading(true);
    setError(null);
    try {
      const data = await fetchBetsData();
      applyData(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load bets.");
    }
    setLoading(false);
  }

  useEffect(() => {
    async function init() {
      setLoading(true);
      setError(null);
      try {
        const data = await fetchBetsData();
        applyData(data);
      } catch (err) {
        setError(
          err instanceof Error ? err.message : "Failed to load bets.",
        );
      }
      setLoading(false);
    }

    init();
  }, []);

  async function settle(betId: string) {
    const outcome = outcomes[betId] ?? "won";
    setSettling((prev) => ({ ...prev, [betId]: true }));
    setError(null);
    const res = await goApiClient.settleBet({ bet_id: betId, outcome });

    if (res.error) {
      setError(res.error);
      toast.error(res.error);
    } else {
      toast.success(
        `Bet ${res.data?.bet.id ?? betId} settled as ${outcome}`,
      );
      await load();
    }
    setSettling((prev) => ({ ...prev, [betId]: false }));
  }

  const pendingBets = bets.filter((b) => b.status === "pending");

  return (
    <div className="container mx-auto max-w-4xl py-8 space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold">Settlements</h1>
        <Button onClick={load} disabled={loading} variant="outline">
          {loading ? "Loading..." : "Refresh"}
        </Button>
      </div>

      {error && <p className="text-red-500">{error}</p>}

      {loading ? (
        <div className="space-y-4">
          <Skeleton className="h-40 w-full" />
          <Skeleton className="h-40 w-full" />
        </div>
      ) : pendingBets.length === 0 ? (
        <p className="text-muted-foreground">No pending bets.</p>
      ) : (
        <div className="space-y-4">
          {pendingBets.map((bet) => (
            <Card key={bet.id}>
              <CardHeader>
                <CardTitle>{getEventLabel(bet.eventId, events)}</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 text-sm">
                  <div>
                    <span className="text-muted-foreground">Market</span>
                    <p className="font-medium">
                      {marketsMap.get(bet.marketId)?.name ?? bet.marketId}
                    </p>
                  </div>
                  <div>
                    <span className="text-muted-foreground">Outcome</span>
                    <p className="font-medium">
                      {outcomesMap.get(bet.outcomeId)?.name ?? bet.outcomeId}
                    </p>
                  </div>
                  <div>
                    <span className="text-muted-foreground">Odds</span>
                    <p className="font-mono">{bet.odds}</p>
                  </div>
                  <div>
                    <span className="text-muted-foreground">Stake</span>
                    <p className="font-medium">
                      {bet.stake} {bet.currency}
                    </p>
                  </div>
                  <div>
                    <span className="text-muted-foreground">Potential Payout</span>
                    <p className="font-medium">
                      {bet.potentialPayout} {bet.currency}
                    </p>
                  </div>
                  <div>
                    <span className="text-muted-foreground">Created</span>
                    <p>{formatDate(bet.createdAt)}</p>
                  </div>
                </div>
                <div className="flex flex-col sm:flex-row items-start sm:items-end gap-2">
                  <div className="w-full sm:w-48">
                    <Label htmlFor={`outcome-${bet.id}`}>Result</Label>
                    <select
                      id={`outcome-${bet.id}`}
                      value={outcomes[bet.id] ?? "won"}
                      onChange={(e) =>
                        setOutcomes((prev) => ({
                          ...prev,
                          [bet.id]: e.target.value as SettlementOutcome,
                        }))
                      }
                      className={SELECT_CLASSES}
                    >
                      <option value="won">Won</option>
                      <option value="lost">Lost</option>
                      <option value="cancelled">Cancelled</option>
                    </select>
                  </div>
                  <Button
                    onClick={() => settle(bet.id)}
                    disabled={settling[bet.id]}
                    className="w-full sm:w-auto"
                  >
                    {settling[bet.id] ? "Settling..." : "Settle"}
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
