"use client";

import { useEffect, useState } from "react";
import { goApiClient } from "@/lib/go-api-client";

export function RatesFooter() {
  const [rates, setRates] = useState<Record<string, string>>({});
  const [fiatCurrency, setFiatCurrency] = useState<string>("USD");

  useEffect(() => {
    async function load() {
      const res = await goApiClient.getRates();
      if (res.data) {
        setRates(res.data.rates);
        setFiatCurrency(res.data.fiat_currency);
      }
    }
    load();
    const id = setInterval(load, 60000);
    return () => clearInterval(id);
  }, []);

  const entries = Object.entries(rates);
  if (entries.length === 0) return null;

  return (
    <footer className="border-t bg-muted py-2">
      <div className="container mx-auto text-center text-sm text-muted-foreground">
        {entries.map(([currency, value], i) => (
          <span key={currency}>
            {currency} ${value} {fiatCurrency}
            {i < entries.length - 1 && " | "}
          </span>
        ))}
      </div>
    </footer>
  );
}
