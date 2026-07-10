"use client";

import { useEffect, useState, FormEvent } from "react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "sonner";
import { goApiClient, Event, Market, Outcome } from "@/lib/go-api-client";

const SELECT_CLASSES =
  "flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50";

export default function SportsbookPage() {
  const [events, setEvents] = useState<Event[]>([]);
  const [markets, setMarkets] = useState<Market[]>([]);
  const [loadingEvents, setLoadingEvents] = useState(true);
  const [loadingMarkets, setLoadingMarkets] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [selectedEventId, setSelectedEventId] = useState<string>("");
  const [selectedMarketId, setSelectedMarketId] = useState<string>("");
  const [selectedOutcomeId, setSelectedOutcomeId] = useState<string>("");
  const [stake, setStake] = useState("");
  const [currency, setCurrency] = useState("USDT");
  const [placing, setPlacing] = useState(false);

  useEffect(() => {
    async function loadEvents() {
      setLoadingEvents(true);
      setError(null);
      const res = await goApiClient.listEvents();
      if (res.error) {
        setError(res.error);
        setEvents([]);
      } else {
        const loaded = res.data?.events ?? [];
        setEvents(loaded);
        if (loaded.length > 0) {
          setSelectedEventId(loaded[0].id);
        }
      }
      setLoadingEvents(false);
    }

    loadEvents();
  }, []);

  useEffect(() => {
    async function loadMarkets() {
      if (!selectedEventId) {
        setMarkets([]);
        setSelectedMarketId("");
        setSelectedOutcomeId("");
        return;
      }
      setLoadingMarkets(true);
      setError(null);
      setSelectedMarketId("");
      setSelectedOutcomeId("");
      const res = await goApiClient.listMarkets(selectedEventId);
      if (res.error) {
        setError(res.error);
        setMarkets([]);
      } else {
        const loaded = res.data?.markets ?? [];
        setMarkets(loaded);
        if (loaded.length > 0) {
          setSelectedMarketId(loaded[0].id);
        }
      }
      setLoadingMarkets(false);
    }

    loadMarkets();
  }, [selectedEventId]);

  const selectedMarket = markets.find((m) => m.id === selectedMarketId);
  const selectedOutcome = selectedMarket?.outcomes.find(
    (o) => o.id === selectedOutcomeId,
  );

  async function handlePlaceBet(e: FormEvent) {
    e.preventDefault();
    if (!selectedEventId || !selectedMarketId || !selectedOutcomeId) {
      toast.error("Please select an event, market and outcome.");
      return;
    }
    if (!stake || Number.parseFloat(stake) <= 0) {
      toast.error("Enter a positive stake.");
      return;
    }
    if (!currency) {
      toast.error("Enter a currency.");
      return;
    }

    setPlacing(true);
    setError(null);
    const res = await goApiClient.placeBet({
      event_id: selectedEventId,
      market_id: selectedMarketId,
      outcome_id: selectedOutcomeId,
      stake,
      currency,
    });

    if (res.error) {
      setError(res.error);
      toast.error(res.error);
    } else {
      toast.success(
        `Bet placed: ${res.data?.bet.id ?? ""} (${res.data?.bet.status ?? ""})`,
      );
      setStake("");
      setSelectedOutcomeId("");
    }
    setPlacing(false);
  }

  return (
    <div className="container mx-auto max-w-2xl py-8 space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold">Sportsbook</h1>
        <Link href="/sportsbook/bets">
          <Button variant="outline">My Bets</Button>
        </Link>
      </div>

      {error && <p className="text-red-500">{error}</p>}

      <Card>
        <CardHeader>
          <CardTitle>Select Event</CardTitle>
        </CardHeader>
        <CardContent>
          {loadingEvents ? (
            <Skeleton className="h-10 w-full" />
          ) : events.length === 0 ? (
            <p className="text-muted-foreground">No events available.</p>
          ) : (
            <select
              id="event"
              value={selectedEventId}
              onChange={(e) => setSelectedEventId(e.target.value)}
              className={SELECT_CLASSES}
            >
              {events.map((event) => (
                <option key={event.id} value={event.id}>
                  {event.homeTeam} vs {event.awayTeam} ({event.status})
                </option>
              ))}
            </select>
          )}
        </CardContent>
      </Card>

      {selectedEventId && (
        <Card>
          <CardHeader>
            <CardTitle>Market & Outcome</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {loadingMarkets ? (
              <>
                <Skeleton className="h-10 w-full" />
                <Skeleton className="h-10 w-full" />
              </>
            ) : markets.length === 0 ? (
              <p className="text-muted-foreground">
                No markets for this event.
              </p>
            ) : (
              <>
                <div>
                  <Label htmlFor="market">Market</Label>
                  <select
                    id="market"
                    value={selectedMarketId}
                    onChange={(e) => {
                      setSelectedMarketId(e.target.value);
                      setSelectedOutcomeId("");
                    }}
                    className={SELECT_CLASSES}
                  >
                    {markets.map((market) => (
                      <option key={market.id} value={market.id}>
                        {market.name}
                      </option>
                    ))}
                  </select>
                </div>

                {selectedMarket && (
                  <div>
                    <Label>Outcome</Label>
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                      {selectedMarket.outcomes.map((outcome: Outcome) => (
                        <Button
                          key={outcome.id}
                          type="button"
                          variant={
                            outcome.id === selectedOutcomeId
                              ? "default"
                              : "outline"
                          }
                          onClick={() => setSelectedOutcomeId(outcome.id)}
                          className="justify-between"
                        >
                          <span>{outcome.name}</span>
                          <span className="font-mono">{outcome.odds}</span>
                        </Button>
                      ))}
                    </div>
                  </div>
                )}
              </>
            )}
          </CardContent>
        </Card>
      )}

      {selectedOutcome && (
        <Card>
          <CardHeader>
            <CardTitle>Bet Slip</CardTitle>
          </CardHeader>
          <CardContent>
            <form onSubmit={handlePlaceBet} className="space-y-4">
              <div className="rounded-lg border p-4 space-y-1">
                <p className="text-sm text-muted-foreground">Selection</p>
                <p className="font-medium">
                  {selectedOutcome.name} @ {selectedOutcome.odds}
                </p>
              </div>
              <div>
                <Label htmlFor="stake">Stake</Label>
                <Input
                  id="stake"
                  type="text"
                  inputMode="decimal"
                  value={stake}
                  onChange={(e) => setStake(e.target.value)}
                  placeholder="100.00"
                  required
                />
              </div>
              <div>
                <Label htmlFor="currency">Currency</Label>
                <Input
                  id="currency"
                  type="text"
                  value={currency}
                  onChange={(e) => setCurrency(e.target.value)}
                  placeholder="USDT"
                  required
                />
              </div>
              <Button
                type="submit"
                disabled={
                  placing ||
                  !selectedEventId ||
                  !selectedMarketId ||
                  !selectedOutcomeId
                }
              >
                {placing ? "Placing Bet..." : "Place Bet"}
              </Button>
            </form>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
