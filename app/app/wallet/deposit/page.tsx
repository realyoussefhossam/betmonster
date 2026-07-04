"use client";

import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  goApiClient,
  DepositAddressResponse,
} from "@/lib/go-api-client";

export default function DepositPage() {
  const [currency, setCurrency] = useState("USDT");
  const [chain, setChain] = useState("base");
  const [address, setAddress] = useState<DepositAddressResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function loadAddress() {
    setLoading(true);
    setError(null);
    const res = await goApiClient.getDepositAddress(currency, chain);
    if (res.error) {
      setError(res.error);
    } else if (res.data) {
      setAddress(res.data);
    }
    setLoading(false);
  }

  useEffect(() => {
    loadAddress();
  }, []);

  return (
    <div className="container mx-auto max-w-2xl py-8 space-y-6">
      <h1 className="text-3xl font-bold">Deposit</h1>
      <div className="space-y-4">
        <div>
          <Label htmlFor="currency">Currency</Label>
          <Input
            id="currency"
            value={currency}
            onChange={(e) => setCurrency(e.target.value)}
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
        <Button onClick={loadAddress} disabled={loading}>
          {loading ? "Loading..." : "Get Address"}
        </Button>
      </div>
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
