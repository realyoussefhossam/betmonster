"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { WalletCard } from "@/components/wallet-card";
import { Button } from "@/components/ui/button";
import {
  goApiClient,
  BalanceResponse,
  Transaction,
} from "@/lib/go-api-client";

export default function WalletPage() {
  const [balance, setBalance] = useState<BalanceResponse | null>(null);
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    async function load() {
      const [balanceRes, txRes] = await Promise.all([
        goApiClient.getBalance("USDT"),
        goApiClient.getTransactions(),
      ]);
      if (balanceRes.error) {
        setError(balanceRes.error);
      } else if (balanceRes.data) {
        setBalance(balanceRes.data);
      }
      if (txRes.data?.transactions) {
        setTransactions(txRes.data.transactions);
      }
      setLoading(false);
    }
    load();
  }, []);

  return (
    <div className="container mx-auto max-w-2xl py-8 space-y-6">
      <h1 className="text-3xl font-bold">Wallet</h1>
      {error && <p className="text-red-500">{error}</p>}
      <WalletCard
        currency={balance?.currency ?? "USDT"}
        balance={balance?.balance ?? "0"}
        loading={loading}
      />
      <div className="flex gap-4">
        <Link href="/wallet/deposit">
          <Button>Deposit</Button>
        </Link>
        <Link href="/wallet/withdraw">
          <Button variant="outline">Withdraw</Button>
        </Link>
      </div>
      <div>
        <h2 className="text-xl font-semibold mb-4">Transactions</h2>
        {transactions.length === 0 ? (
          <p className="text-muted-foreground">No transactions yet.</p>
        ) : (
          <ul className="space-y-2">
            {transactions.map((tx) => (
              <li
                key={tx.id}
                className="flex justify-between rounded-lg border p-4"
              >
                <span className="capitalize">{tx.type}</span>
                <span>
                  {tx.amount} {tx.status}
                </span>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}
