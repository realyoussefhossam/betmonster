"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { goApiClient } from "@/lib/go-api-client";

export default function WithdrawPage() {
  const [currency, setCurrency] = useState("USDT");
  const [amount, setAmount] = useState("");
  const [chain, setChain] = useState("base");
  const [destinationAddress, setDestinationAddress] = useState("");
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
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
          <Input
            id="currency"
            value={currency}
            onChange={(e) => setCurrency(e.target.value)}
          />
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
          <Input
            id="chain"
            value={chain}
            onChange={(e) => setChain(e.target.value)}
          />
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
        <Button type="submit" disabled={loading}>
          {loading ? "Submitting..." : "Request Withdrawal"}
        </Button>
      </form>
      {error && <p className="text-red-500">{error}</p>}
      {result && <p className="text-green-600">{result}</p>}
    </div>
  );
}
