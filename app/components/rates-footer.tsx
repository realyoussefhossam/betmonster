"use client";

import { useEffect, useState } from "react";
import { goApiClient } from "@/lib/go-api-client";
import { useFiatCurrency } from "@/components/fiat-provider";

export function RatesFooter() {
  const { fiat } = useFiatCurrency();
  const [rates, setRates] = useState<Record<string, string>>({});

  useEffect(() => {
    async function load() {
      const res = await goApiClient.getRates(fiat);
      if (res.data) {
        setRates(res.data.rates);
      }
    }
    load();
    const id = setInterval(load, 60000);
    return () => clearInterval(id);
  }, [fiat]);

  const entries = Object.entries(rates);
  if (entries.length === 0) return null;

  return (
    <footer className="border-t bg-muted py-2">
      <div className="container mx-auto text-center text-sm text-muted-foreground">
        {entries.map(([currency, value], i) => (
          <span key={currency}>
            {currency} {value} {fiat}
            {i < entries.length - 1 && " | "}
          </span>
        ))}
      </div>
    </footer>
  );
}
