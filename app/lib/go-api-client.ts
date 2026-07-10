import { authClient } from "@/lib/auth-client";
import { decodeJwt } from "jose";

const GO_API_URL =
  process.env.NEXT_PUBLIC_GO_API_URL || "http://localhost:8080";

export interface ApiResponse<T = unknown> {
  data?: T;
  error?: string;
  status: number;
}

export interface UserProfile {
  id: string;
  email: string;
  name: string;
}

export interface BalanceResponse {
  currency: string;
  balance: string;
  fiatCurrency?: string;
  fiatValue?: string;
}

export interface RatesResponse {
  fiat_currency: string;
  rates: Record<string, string>;
}

export interface Transaction {
  id: string;
  userId: string;
  walletId: string;
  type: string;
  currency: string;
  amount: string;
  balanceBefore: string;
  balanceAfter: string;
  status: string;
  referenceId: string;
  metadata: string;
  createdAt: string;
  fiatValue?: string;
}

export interface TransactionsResponse {
  transactions: Transaction[];
}

export interface DepositAddressResponse {
  address: string;
  chain: string;
  currency: string;
}

export interface WithdrawalResponse {
  withdrawalId: string;
  status: string;
}

export interface SupportedOptionsResponse {
  currencies: string[];
  chains: string[];
  pairs: string[];
}

export interface WithdrawalRequest {
  id: string;
  userId: string;
  currency: string;
  amount: string;
  destinationAddress: string;
  chain: string;
  status: string;
  txHash: string;
  createdAt: string;
}

export interface PendingWithdrawalsResponse {
  withdrawals: WithdrawalRequest[];
}

export interface ReviewWithdrawalResponse {
  status: string;
}

export interface Sport {
  id: string;
  name: string;
}

export interface League {
  id: string;
  name: string;
}

export interface Event {
  id: string;
  sportId: string;
  leagueId: string;
  homeTeam: string;
  awayTeam: string;
  startTime: string;
  status: string;
  liveMinute?: number;
}

export interface Outcome {
  id: string;
  name: string;
  odds: string;
  status: string;
}

export interface Market {
  id: string;
  eventId: string;
  name: string;
  status: string;
  outcomes: Outcome[];
}

export interface Bet {
  id: string;
  userId: string;
  eventId: string;
  marketId: string;
  outcomeId: string;
  odds: string;
  stake: string;
  potentialPayout: string;
  currency: string;
  status: string;
  referenceId: string;
  debitTransactionId?: string;
  creditTransactionId?: string;
  createdAt: string;
  settledAt?: string;
}

export interface EventsResponse {
  events: Event[];
}

export interface MarketsResponse {
  markets: Market[];
}

export interface BetsResponse {
  bets: Bet[];
}

export interface PlaceBetRequest {
  event_id: string;
  market_id: string;
  outcome_id: string;
  stake: string;
  currency: string;
}

export interface PlaceBetResponse {
  bet: Bet;
}

export interface SettleBetRequest {
  bet_id: string;
  outcome: "won" | "lost" | "cancelled";
}

export interface SettleBetResponse {
  bet: Bet;
}

class GoApiClient {
  private baseUrl: string;
  private cachedToken: string | null;

  constructor(baseUrl: string = GO_API_URL) {
    this.baseUrl = baseUrl;
    this.cachedToken = null;
  }

  private isTokenValid(): boolean {
    if (!this.cachedToken) {
      return false;
    }

    const jwt = decodeJwt(this.cachedToken);
    if (!jwt.exp) {
      return false;
    }

    const currentTimeInSeconds = Math.floor(Date.now() / 1000);
    return jwt.exp > currentTimeInSeconds + 10;
  }

  private async getToken(): Promise<string | null> {
    if (this.isTokenValid()) {
      return this.cachedToken;
    }

    const token = (await authClient.token().then((x) => x.data?.token)) || null;

    this.cachedToken = token;

    return token;
  }

  private async request<T = unknown>(
    endpoint: string,
    options: RequestInit = {},
  ): Promise<ApiResponse<T>> {
    try {
      const token = await this.getToken();

      // Send request
      const response = await fetch(`${this.baseUrl}${endpoint}`, {
        ...options,
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
      });

      const data = await response.json();

      if (!response.ok) {
        return {
          error:
            data.message ||
            `API Error: ${response.status} ${response.statusText}`,
          status: response.status,
        };
      }

      return {
        data,
        status: response.status,
      };
    } catch (error) {
      return {
        error:
          error instanceof Error ? error.message : "Unknown error occurred",
        status: 0,
      };
    }
  }

  async getSupportedOptions(): Promise<ApiResponse<SupportedOptionsResponse>> {
    return this.request("/api/wallet/supported", { method: "GET" });
  }

  async getRates(fiatCurrency?: string): Promise<ApiResponse<RatesResponse>> {
    const params = new URLSearchParams();
    if (fiatCurrency) params.set("fiat_currency", fiatCurrency);
    const query = params.toString() ? `?${params.toString()}` : "";
    return this.request(`/api/wallet/rates${query}`, { method: "GET" });
  }

  async getBalance(currency: string = "USDT", fiatCurrency?: string): Promise<ApiResponse<BalanceResponse>> {
    const params = new URLSearchParams({ currency });
    if (fiatCurrency) params.set("fiat_currency", fiatCurrency);
    return this.request(`/api/wallet/balance?${params.toString()}`, { method: "GET" });
  }

  async getTransactions(fiatCurrency?: string): Promise<ApiResponse<TransactionsResponse>> {
    const params = new URLSearchParams();
    if (fiatCurrency) params.set("fiat_currency", fiatCurrency);
    const query = params.toString() ? `?${params.toString()}` : "";
    return this.request(`/api/wallet/transactions${query}`, { method: "GET" });
  }

  async getDepositAddress(
    currency: string = "USDT",
    chain: string = "polygon",
  ): Promise<ApiResponse<DepositAddressResponse>> {
    return this.request(`/api/wallet/deposit-address?currency=${currency}&chain=${chain}`, {
      method: "GET",
    });
  }

  async requestWithdrawal(body: {
    currency: string;
    amount: string;
    destinationAddress: string;
    chain: string;
  }): Promise<ApiResponse<WithdrawalResponse>> {
    return this.request("/api/wallet/withdraw", {
      method: "POST",
      body: JSON.stringify(body),
    });
  }

  async listPendingWithdrawals(): Promise<ApiResponse<PendingWithdrawalsResponse>> {
    return this.request("/api/admin/withdrawals", { method: "GET" });
  }

  async reviewWithdrawal(body: {
    withdrawalId: string;
    action: "approve" | "reject";
    txHash?: string;
  }): Promise<ApiResponse<ReviewWithdrawalResponse>> {
    return this.request("/api/admin/withdrawals/review", {
      method: "POST",
      body: JSON.stringify({ ...body, reviewedBy: "admin" }),
    });
  }

  async listSports(): Promise<ApiResponse<{ sports: Sport[] }>> {
    return this.request("/api/sports", { method: "GET" });
  }

  async listEvents(): Promise<ApiResponse<EventsResponse>> {
    return this.request("/api/events", { method: "GET" });
  }

  async listMarkets(eventId: string): Promise<ApiResponse<MarketsResponse>> {
    return this.request(`/api/events/${encodeURIComponent(eventId)}/markets`, { method: "GET" });
  }

  async listOutcomes(marketId: string): Promise<ApiResponse<{ outcomes: Outcome[] }>> {
    return this.request(`/api/markets/${encodeURIComponent(marketId)}/outcomes`, { method: "GET" });
  }

  async placeBet(body: PlaceBetRequest): Promise<ApiResponse<PlaceBetResponse>> {
    return this.request("/api/bets", {
      method: "POST",
      body: JSON.stringify(body),
    });
  }

  async listBets(): Promise<ApiResponse<BetsResponse>> {
    return this.request("/api/bets", { method: "GET" });
  }

  async getBet(betId: string): Promise<ApiResponse<{ bet: Bet }>> {
    return this.request(`/api/bets/${encodeURIComponent(betId)}`, { method: "GET" });
  }

  async settleBet(body: SettleBetRequest): Promise<ApiResponse<SettleBetResponse>> {
    return this.request("/api/admin/bets/settle", {
      method: "POST",
      body: JSON.stringify(body),
    });
  }
}

export const goApiClient = new GoApiClient();
export default goApiClient;
