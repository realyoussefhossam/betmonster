"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";

interface VerifyResponse {
  status: string;
  message: string;
  user?: {
    ID: string;
    Email: string;
    Name: string;
  };
}

interface ErrorResponse {
  error: string;
}

export const GoApiTest = () => {
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<VerifyResponse | ErrorResponse | null>(null);

  async function testVerify() {
    setLoading(true);
    setResult(null);
    try {
      const res = await fetch("/api/verify", { method: "GET" });
      const data = await res.json();
      setResult(data);
      if (res.ok) {
        toast.success("Go API verified the JWT");
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

  async function testMe() {
    setLoading(true);
    setResult(null);
    try {
      const res = await fetch("/api/me", { method: "GET" });
      const data = await res.json();
      setResult(data);
      if (res.ok) {
        toast.success("Go API returned user data");
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
        <Button onClick={testVerify} disabled={loading} size="sm">
          {loading ? "Testing..." : "Test /api/verify"}
        </Button>
        <Button onClick={testMe} disabled={loading} size="sm" variant="outline">
          {loading ? "Testing..." : "Test /api/me"}
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
