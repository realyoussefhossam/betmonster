"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";

interface ErrorResponse {
  error: string;
}

export const GoApiTest = () => {
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<Record<string, unknown> | ErrorResponse | null>(null);

  async function testBalance() {
    setLoading(true);
    setResult(null);
    try {
      const res = await fetch("/api/wallet/balance?currency=USDT", { method: "GET" });
      const data = await res.json();
      setResult(data);
      if (res.ok) {
        toast.success("Wallet balance endpoint OK");
      } else {
        toast.error(data.error ?? "Request failed");
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unknown error";
      setResult({ error: message });
      toast.error(message);
    } finally {
      setLoading(false);
    }
  }

  async function testTransactions() {
    setLoading(true);
    setResult(null);
    try {
      const res = await fetch("/api/wallet/transactions", { method: "GET" });
      const data = await res.json();
      setResult(data);
      if (res.ok) {
        toast.success("Transactions endpoint OK");
      } else {
        toast.error(data.error ?? "Request failed");
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unknown error";
      setResult({ error: message });
      toast.error(message);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex gap-2">
        <Button onClick={testBalance} disabled={loading} size="sm">
          {loading ? "Testing..." : "Test balance"}
        </Button>
        <Button onClick={testTransactions} disabled={loading} size="sm" variant="outline">
          {loading ? "Testing..." : "Test transactions"}
        </Button>
      </div>

      {result && (
        <pre className="text-xs bg-muted p-4 rounded-md overflow-auto">
          {JSON.stringify(result, null, 2)}
        </pre>
      )}
    </div>
  );
};
