"use client";

import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { goApiClient, SupportedOptionsResponse } from "@/lib/go-api-client";

export default function WithdrawPage() {
  const [options, setOptions] = useState<SupportedOptionsResponse | null>(null);
  const [optionsError, setOptionsError] = useState<string | null>(null);
  const [currency, setCurrency] = useState("");
  const [amount, setAmount] = useState("");
  const [chain, setChain] = useState("");
  const [destinationAddress, setDestinationAddress] = useState("");
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    async function loadOptions() {
      const res = await goApiClient.getSupportedOptions();
      if (res.error) {
        setOptionsError(res.error);
      } else if (res.data) {
        setOptions(res.data);
        setCurrency(res.data.currencies[0] ?? "");
        setChain(res.data.chains[0] ?? "");
      }
    }
    loadOptions();
  }, []);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!currency || !chain) return;
    setLoading(true);
    setError(null);
    setResult(null);
    const res = await goApiClient.requestWithdrawal({
      currency,
      amount,
      destinationAddress,
      chain,
    });
    if (res.error) {
      setError(res.error);
    } else {
      setResult(`Withdrawal request ${res.data?.withdrawalId} is ${res.data?.status}`);
    }
    setLoading(false);
  }

  return (
    <div className="container mx-auto max-w-2xl py-8 space-y-6">
      <h1 className="text-3xl font-bold">Withdraw</h1>
      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <Label htmlFor="currency">Currency</Label>
          <select
            id="currency"
            value={currency}
            onChange={(e) => setCurrency(e.target.value)}
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
          <Label htmlFor="amount">Amount</Label>
          <Input
            id="amount"
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
            placeholder="100.00"
          />
        </div>
        <div>
          <Label htmlFor="chain">Chain</Label>
          <select
            id="chain"
            value={chain}
            onChange={(e) => setChain(e.target.value)}
            disabled={!options || options.chains.length === 0}
            className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {options?.chains.map((c) => (
              <option key={c} value={c}>
                {c}
              </option>
            )) ?? <option value="">Loading...</option>}
          </select>
        </div>
        <div>
          <Label htmlFor="destination">Destination Address</Label>
          <Input
            id="destination"
            value={destinationAddress}
            onChange={(e) => setDestinationAddress(e.target.value)}
            placeholder="0x..."
          />
        </div>
        <Button type="submit" disabled={loading || !currency || !chain}>
          {loading ? "Submitting..." : "Request Withdrawal"}
        </Button>
      </form>
      {optionsError && <p className="text-red-500">{optionsError}</p>}
      {error && <p className="text-red-500">{error}</p>}
      {result && <p className="text-green-600">{result}</p>}
    </div>
  );
}
