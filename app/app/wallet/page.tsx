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

function formatDate(value: string) {
  if (!value) return "-";
  const date = new Date(value);
  return isNaN(date.getTime()) ? value : date.toLocaleString();
}

export default function WalletPage() {
  const [balances, setBalances] = useState<BalanceResponse[]>([]);
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    async function load() {
      const [supportedRes, txRes] = await Promise.all([
        goApiClient.getSupportedOptions(),
        goApiClient.getTransactions(),
      ]);

      if (supportedRes.error) {
        setError(supportedRes.error);
      } else if (supportedRes.data?.currencies) {
        const balanceResults = await Promise.all(
          supportedRes.data.currencies.map((currency) =>
            goApiClient.getBalance(currency),
          ),
        );
        const loadedBalances = balanceResults
          .filter((res) => res.data)
          .map((res) => res.data!);
        setBalances(loadedBalances);
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
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        {loading ? (
          <WalletCard currency="..." balance="0" loading />
        ) : (
          balances.map((b) => (
            <WalletCard
              key={b.currency}
              currency={b.currency}
              balance={b.balance}
              loading={false}
            />
          ))
        )}
      </div>
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
                className="flex justify-between items-center rounded-lg border p-4"
              >
                <div className="space-y-1">
                  <span className="block capitalize">{tx.type}</span>
                  <span className="block text-xs text-muted-foreground">
                    {formatDate(tx.createdAt)}
                  </span>
                </div>
                <span className="text-right">
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
