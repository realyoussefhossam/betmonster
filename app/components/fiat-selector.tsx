"use client";

import { useState } from "react";

const SUPPORTED_FIAT = [
  "USD", "EUR", "JPY", "INR", "CAD", "CNY", "IDR", "KRW", "PHP", "RUB",
  "MXN", "PLN", "TRY", "VND", "ARS", "PEN", "CLP", "NGN", "AED", "BHD",
  "CRC", "KWD", "MAD", "MYR", "QAR", "SAR", "SGD", "TND", "TWD", "GHS",
  "KES", "BOB", "XOF", "PKR", "NZD", "ISK", "BAM", "TZS", "EGP", "LKR",
  "UGX", "KZT", "BDT", "UAH", "GEL", "MNT", "GTQ", "KGS", "ZAR", "TMT",
  "ZMW", "TJS", "MRU", "TTD", "GMD", "MGA", "JMD", "NIO", "HNL", "MZN",
  "XAF", "RWF", "GNF", "BWP", "KMF", "LSL", "ERN", "BIF", "MWK", "PGK",
];

const STORAGE_KEY = "betmonster-fiat";

function getInitialFiat(): string {
  if (typeof window === "undefined") {
    return "USD";
  }
  const saved = localStorage.getItem(STORAGE_KEY);
  return saved && SUPPORTED_FIAT.includes(saved) ? saved : "USD";
}

export function useFiatCurrency() {
  const [fiat, setFiatState] = useState<string>(getInitialFiat);

  const setFiat = (value: string) => {
    setFiatState(value);
    if (typeof window !== "undefined") {
      localStorage.setItem(STORAGE_KEY, value);
    }
  };

  return { fiat, setFiat };
}

export function FiatSelector({
  value,
  onChange,
}: {
  value: string;
  onChange: (v: string) => void;
}) {
  return (
    <select
      value={value}
      onChange={(e) => onChange(e.target.value)}
      className="rounded border bg-background px-2 py-1 text-sm"
      aria-label="Display currency"
    >
      {SUPPORTED_FIAT.map((c) => (
        <option key={c} value={c}>
          {c}
        </option>
      ))}
    </select>
  );
}
