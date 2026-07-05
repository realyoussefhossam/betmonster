"use client";

import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import {
  goApiClient,
  DepositAddressResponse,
  SupportedOptionsResponse,
} from "@/lib/go-api-client";

export default function DepositPage() {
  const [options, setOptions] = useState<SupportedOptionsResponse | null>(null);
  const [optionsError, setOptionsError] = useState<string | null>(null);
  const [currency, setCurrency] = useState("");
  const [chain, setChain] = useState("");
  const [address, setAddress] = useState<DepositAddressResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const validChainsForCurrency = (currency: string) => {
    if (!options) return [];
    return options.chains.filter((chain) =>
      options.pairs.includes(`${currency}:${chain}`),
    );
  };

  useEffect(() => {
    async function loadOptions() {
      const res = await goApiClient.getSupportedOptions();
      if (res.error) {
        setOptionsError(res.error);
      } else if (res.data) {
        const data = res.data;
        setOptions(data);
        const firstCurrency = data.currencies[0] ?? "";
        setCurrency(firstCurrency);
        setChain(
          data.chains.find((chain) =>
            data.pairs.includes(`${firstCurrency}:${chain}`),
          ) ?? data.chains[0] ?? "",
        );
      }
    }
    loadOptions();
  }, []);

  const validChains = validChainsForCurrency(currency);
  const currentChain = validChains.includes(chain) ? chain : validChains[0] ?? "";

  async function loadAddress() {
    if (!currency || !currentChain) return;
    setLoading(true);
    setError(null);
    const res = await goApiClient.getDepositAddress(currency, currentChain);
    if (res.error) {
      setError(res.error);
    } else if (res.data) {
      setAddress(res.data);
    }
    setLoading(false);
  }

  return (
    <div className="container mx-auto max-w-2xl py-8 space-y-6">
      <h1 className="text-3xl font-bold">Deposit</h1>
      <div className="space-y-4">
        <div>
          <Label htmlFor="currency">Currency</Label>
          <select
            id="currency"
            value={currency}
            onChange={(e) => {
              const nextCurrency = e.target.value;
              setCurrency(nextCurrency);
              const nextValid = validChainsForCurrency(nextCurrency);
              setChain(nextValid[0] ?? "");
            }}
            disabled={!options || options.currencies.length === 0}
            className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {options?.currencies.map((c) => (
              <option key={c} value={c}>
                {c}
              </option>
            )) ?? <option value="">Loading...</option>}
          </select>
        </div>
        <div>
          <Label htmlFor="chain">Chain</Label>
          <select
            id="chain"
            value={currentChain}
            onChange={(e) => setChain(e.target.value)}
            disabled={!options || validChains.length === 0}
            className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {validChains.map((c) => (
              <option key={c} value={c}>
                {c}
              </option>
            )) ?? <option value="">Loading...</option>}
          </select>
        </div>
        <Button onClick={loadAddress} disabled={loading || !currency || !currentChain}>
          {loading ? "Loading..." : "Get Address"}
        </Button>
      </div>
      {optionsError && <p className="text-red-500">{optionsError}</p>}
      {error && <p className="text-red-500">{error}</p>}
      {address && (
        <div className="rounded-lg border p-4 space-y-2">
          <p className="text-sm text-muted-foreground">Deposit address</p>
          <p className="font-mono break-all">{address.address}</p>
          <p className="text-sm text-muted-foreground">
            {address.currency} on {address.chain}
          </p>
        </div>
      )}
    </div>
  );
}
