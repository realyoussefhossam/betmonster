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

export interface AuthVerifyResponse {
  valid: boolean;
  userId?: string;
  email?: string;
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

  async verifyAuth(): Promise<ApiResponse<AuthVerifyResponse>> {
    return this.request("/api/verify", {
      method: "GET",
    });
  }
}

export const goApiClient = new GoApiClient();
export default goApiClient;
