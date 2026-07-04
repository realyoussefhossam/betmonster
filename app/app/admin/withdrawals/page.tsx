"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  goApiClient,
  WithdrawalRequest,
} from "@/lib/go-api-client";

export default function AdminWithdrawalsPage() {
  const [withdrawals, setWithdrawals] = useState<WithdrawalRequest[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [txHash, setTxHash] = useState("");

  async function load() {
    setLoading(true);
    setError(null);
    const res = await goApiClient.listPendingWithdrawals();
    if (res.error) {
      setError(res.error);
    } else {
      setWithdrawals(res.data?.withdrawals ?? []);
    }
    setLoading(false);
  }

  async function review(id: string, action: "approve" | "reject") {
    const res = await goApiClient.reviewWithdrawal({
      withdrawalId: id,
      action,
      txHash: action === "approve" ? txHash : undefined,
    });
    if (res.error) {
      setError(res.error);
    } else {
      await load();
    }
  }

  return (
    <div className="container mx-auto max-w-4xl py-8 space-y-6">
      <h1 className="text-3xl font-bold">Pending Withdrawals</h1>
      {error && <p className="text-red-500">{error}</p>}
      <div className="flex gap-4">
        <Button onClick={load} disabled={loading}>
          {loading ? "Loading..." : "Load Pending Withdrawals"}
        </Button>
      </div>
      <div className="mb-4">
        <Label htmlFor="txHash">Transaction Hash (for approve)</Label>
        <Input
          id="txHash"
          value={txHash}
          onChange={(e) => setTxHash(e.target.value)}
          placeholder="0x..."
        />
      </div>
      {loading ? (
        <p>Loading...</p>
      ) : withdrawals.length === 0 ? (
        <p className="text-muted-foreground">No pending withdrawals.</p>
      ) : (
        <ul className="space-y-4">
          {withdrawals.map((w) => (
            <li
              key={w.id}
              className="flex flex-col gap-2 rounded-lg border p-4"
            >
              <div className="flex justify-between">
                <span className="font-mono">{w.id}</span>
                <span>
                  {w.amount} {w.currency}
                </span>
              </div>
              <p className="text-sm text-muted-foreground break-all">
                {w.destinationAddress} ({w.chain})
              </p>
              <div className="flex gap-2">
                <Button
                  size="sm"
                  onClick={() => review(w.id, "approve")}
                  disabled={!txHash}
                >
                  Approve
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => review(w.id, "reject")}
                >
                  Reject
                </Button>
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
