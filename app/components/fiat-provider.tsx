"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useReducer,
  useSyncExternalStore,
  ReactNode,
} from "react";
import { goApiClient } from "@/lib/go-api-client";

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

interface FiatContextValue {
  fiat: string;
  setFiat: (value: string) => void;
  rates: Record<string, string> | null;
  ratesLoading: boolean;
}

const FiatContext = createContext<FiatContextValue>({
  fiat: "USD",
  setFiat: () => {},
  rates: null,
  ratesLoading: false,
});

function getStoredFiat(): string {
  if (typeof window === "undefined") {
    return "USD";
  }
  const saved = localStorage.getItem(STORAGE_KEY);
  return saved && SUPPORTED_FIAT.includes(saved) ? saved : "USD";
}

function subscribe(callback: () => void) {
  function handleStorage(e: StorageEvent) {
    if (e.key === STORAGE_KEY) {
      callback();
    }
  }
  window.addEventListener("storage", handleStorage);
  return () => window.removeEventListener("storage", handleStorage);
}

interface RatesState {
  rates: Record<string, string> | null;
  loading: boolean;
}

type RatesAction =
  | { type: "START" }
  | { type: "SUCCESS"; rates: Record<string, string> | null }
  | { type: "ERROR" };

function ratesReducer(state: RatesState, action: RatesAction): RatesState {
  switch (action.type) {
    case "START":
      return { ...state, loading: true };
    case "SUCCESS":
      return { rates: action.rates, loading: false };
    case "ERROR":
      return { ...state, loading: false };
    default:
      return state;
  }
}

export function FiatProvider({ children }: { children: ReactNode }) {
  const fiat = useSyncExternalStore(subscribe, getStoredFiat, () => "USD");
  const [ratesState, dispatch] = useReducer(ratesReducer, {
    rates: null,
    loading: false,
  });

  useEffect(() => {
    let cancelled = false;
    dispatch({ type: "START" });
    goApiClient
      .getRates(fiat)
      .then((res) => {
        if (!cancelled) {
          dispatch({ type: "SUCCESS", rates: res.data?.rates ?? null });
        }
      })
      .catch(() => {
        if (!cancelled) {
          dispatch({ type: "ERROR" });
        }
      });
    return () => {
      cancelled = true;
    };
  }, [fiat]);

  const setFiat = useCallback((value: string) => {
    if (typeof window !== "undefined") {
      localStorage.setItem(STORAGE_KEY, value);
      window.dispatchEvent(
        new StorageEvent("storage", { key: STORAGE_KEY, newValue: value }),
      );
    }
  }, []);

  const value = useMemo(
    () => ({
      fiat,
      setFiat,
      rates: ratesState.rates,
      ratesLoading: ratesState.loading,
    }),
    [fiat, setFiat, ratesState],
  );

  return (
    <FiatContext.Provider value={value}>{children}</FiatContext.Provider>
  );
}

export function useFiatCurrency() {
  return useContext(FiatContext);
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
