"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
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

export default function MyBetsPage() {
  const [bets, setBets] = useState<Bet[]>([]);
  const [events, setEvents] = useState<Event[]>([]);
  const [marketsMap, setMarketsMap] = useState<Map<string, Market>>(new Map());
  const [outcomesMap, setOutcomesMap] = useState<Map<string, Outcome>>(
    new Map(),
  );
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    async function load() {
      setLoading(true);
      setError(null);
      const [betsRes, eventsRes] = await Promise.all([
        goApiClient.listBets(),
        goApiClient.listEvents(),
      ]);

      if (betsRes.error || eventsRes.error) {
        setError(betsRes.error || eventsRes.error || "Failed to load bets.");
        setLoading(false);
        return;
      }

      const loadedBets = betsRes.data?.bets ?? [];
      const loadedEvents = eventsRes.data?.events ?? [];
      setBets(loadedBets);
      setEvents(loadedEvents);

      const eventIds = Array.from(new Set(loadedBets.map((b) => b.eventId)));
      if (eventIds.length > 0) {
        const marketResults = await Promise.all(
          eventIds.map((id) => goApiClient.listMarkets(id)),
        );
        const nextMarkets = new Map<string, Market>();
        const nextOutcomes = new Map<string, Outcome>();
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
        setMarketsMap(nextMarkets);
        setOutcomesMap(nextOutcomes);
      }

      setLoading(false);
    }

    load();
  }, []);

  return (
    <div className="container mx-auto max-w-2xl py-8 space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold">My Bets</h1>
        <Link href="/sportsbook">
          <Button variant="outline">Back to Sportsbook</Button>
        </Link>
      </div>

      {error && <p className="text-red-500">{error}</p>}

      {loading ? (
        <div className="space-y-4">
          <Skeleton className="h-32 w-full" />
          <Skeleton className="h-32 w-full" />
        </div>
      ) : bets.length === 0 ? (
        <p className="text-muted-foreground">No bets yet.</p>
      ) : (
        <div className="space-y-4">
          {bets.map((bet) => (
            <Card key={bet.id}>
              <CardHeader>
                <CardTitle>{getEventLabel(bet.eventId, events)}</CardTitle>
              </CardHeader>
              <CardContent className="space-y-2">
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Market</span>
                  <span className="font-medium">
                    {marketsMap.get(bet.marketId)?.name ?? bet.marketId}
                  </span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Outcome</span>
                  <span className="font-medium">
                    {outcomesMap.get(bet.outcomeId)?.name ?? bet.outcomeId}
                  </span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Odds</span>
                  <span className="font-mono">{bet.odds}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Stake</span>
                  <span className="font-medium">
                    {bet.stake} {bet.currency}
                  </span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Potential Payout</span>
                  <span className="font-medium">
                    {bet.potentialPayout} {bet.currency}
                  </span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Status</span>
                  <span className="capitalize font-medium">{bet.status}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Created</span>
                  <span className="text-sm">{formatDate(bet.createdAt)}</span>
                </div>
                {bet.settledAt && (
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Settled</span>
                    <span className="text-sm">{formatDate(bet.settledAt)}</span>
                  </div>
                )}
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
