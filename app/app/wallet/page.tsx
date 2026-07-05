"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { WalletCard } from "@/components/wallet-card";
import { Button } from "@/components/ui/button";
import { FiatSelector, useFiatCurrency } from "@/components/fiat-provider";
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

function isZeroBalance(balance: string): boolean {
  return balance === "0" || balance === "0.00000000";
}

export default function WalletPage() {
  const [balances, setBalances] = useState<BalanceResponse[]>([]);
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showZero, setShowZero] = useState(false);
  const { fiat, setFiat, isReady } = useFiatCurrency();

  useEffect(() => {
    if (!isReady) return;

    async function load() {
      setLoading(true);
      setError(null);

      const supportedRes = await goApiClient.getSupportedOptions();
      if (supportedRes.error) {
        setError(supportedRes.error);
        setBalances([]);
        setTransactions([]);
        setLoading(false);
        return;
      }

      const currencies = supportedRes.data?.currencies ?? [];
      const [txRes, ...balanceResults] = await Promise.all([
        goApiClient.getTransactions(fiat),
        ...currencies.map((currency) => goApiClient.getBalance(currency, fiat)),
      ]);

      const loadedBalances = balanceResults
        .filter((res) => res.data)
        .map((res) => res.data!);

      setBalances(loadedBalances);
      setTransactions(txRes.data?.transactions ?? []);
      setLoading(false);
    }

    load();
  }, [fiat, showZero, isReady]);

  const visibleBalances = balances.filter(
    (b) => showZero || !isZeroBalance(b.balance),
  );
  const zeroCount = balances.filter((b) => isZeroBalance(b.balance)).length;

  return (
    <div className="container mx-auto max-w-2xl py-8 space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold">Wallet</h1>
        <FiatSelector value={fiat} onChange={setFiat} />
      </div>
      {error && <p className="text-red-500">{error}</p>}
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-semibold">Balances</h2>
        {!loading && zeroCount > 0 && (
          <button
            onClick={() => setShowZero((v) => !v)}
            className="text-sm text-primary underline-offset-4 hover:underline"
            type="button"
          >
            {showZero
              ? "Hide zero balances"
              : `Show ${zeroCount} zero balance${zeroCount === 1 ? "" : "s"}`}
          </button>
        )}
      </div>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        {loading ? (
          <>
            <WalletCard currency="..." balance="0" loading />
            <WalletCard currency="..." balance="0" loading />
          </>
        ) : visibleBalances.length === 0 ? (
          <p className="text-muted-foreground">
            {showZero ? "No balances found." : "No non-zero balances."}
          </p>
        ) : (
          visibleBalances.map((b) => (
            <WalletCard
              key={b.currency}
              currency={b.currency}
              balance={b.balance}
              fiatValue={b.fiatValue}
              fiatCurrency={b.fiatCurrency}
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
        {loading ? (
          <p className="text-muted-foreground">Loading transactions...</p>
        ) : transactions.length === 0 ? (
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
                  {tx.amount} {tx.currency}{" "}
                  <span className="text-xs text-muted-foreground">
                    {tx.status}
                  </span>
                  {tx.fiatValue && (
                    <div className="text-xs text-muted-foreground">
                      ≈ {tx.fiatValue} {fiat}
                    </div>
                  )}
                </span>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}
