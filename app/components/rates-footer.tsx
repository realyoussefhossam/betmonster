"use client";

import { useEffect } from "react";
import { goApiClient } from "@/lib/go-api-client";
import { useFiatCurrency } from "@/components/fiat-provider";

export function RatesFooter() {
  const { fiat, rates, ratesLoading } = useFiatCurrency();

  useEffect(() => {
    async function load() {
      await goApiClient.getRates(fiat);
    }
    const id = setInterval(load, 60000);
    return () => clearInterval(id);
  }, [fiat]);

  const entries = rates ? Object.entries(rates) : [];
  if (entries.length === 0 && !ratesLoading) return null;

  return (
    <footer className="border-t bg-muted py-2">
      <div className="container mx-auto text-center text-sm text-muted-foreground">
        {entries.length === 0 ? (
          <span>Loading rates...</span>
        ) : (
          entries.map(([currency, value], i) => (
            <span key={currency}>
              {currency} {value} {fiat}
              {i < entries.length - 1 && " | "}
            </span>
          ))
        )}
      </div>
    </footer>
  );
}
