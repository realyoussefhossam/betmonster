# BetMonster Wallet Microservice Roadmap

**Scope:** how the Wallet microservice evolves from the simple v1 slice to a production-ready, top-level wallet service.

**Status:** living document — updated as the wallet service grows.

---

## v1 — Foundation

**Mission:** users can deposit, hold, and withdraw USDT and USDC.

- Per-currency wallets (USDT, USDC).
- Deposits via xcash per-user deposit address.
- Deposit webhook handling with HMAC validation and idempotency.
- Manual admin withdrawals (debit on request, admin supplies tx hash).
- Auditable transaction ledger (`balance_before`, `balance_after`, `reference_id`).
- Optimistic locking on wallet rows.
- gRPC API for internal services.
- Admin dashboard for withdrawals.

---

## v2 — Multi-Currency & Limits

**Mission:** support more currencies and basic risk controls.

- New currencies: BTC, ETH, and other stablecoins (each as a separate wallet).
- Currency conversion engine (optional USD-equivalent view or actual conversion).
- Deposit limits per user and globally.
- Withdrawal limits per user and globally.
- Address book / saved withdrawal addresses.
- Pending withdrawal queue with priority.
- Transaction history pagination and filtering.
- Balance snapshots for audit.

---

## v3 — Automated Withdrawals

**Mission:** reduce manual operator work for payouts.

- Hot wallet integration for automated withdrawals.
- Cold wallet / treasury management.
- Withdrawal approval workflows (single admin, multi-admin, thresholds).
- Address allowlists and blocklists.
- Withdrawal scheduling and batching.
- Fee engine (withdrawal fees, network fees, dynamic fee estimation).
- Gas management for EVM/Tron withdrawals.

---

## v4 — Compliance & Risk

**Mission:** protect users and operators from fraud and legal issues.

- KYC/AML status integration (wallet respects KYC level).
- Source-of-funds tagging.
- Sanctions and risk-address screening (e.g., OFAC, Chainalysis-style lists).
- Deposit holds for high-risk transactions.
- Withdrawal holds for suspicious activity.
- Travel rule / FATF compliance hooks.

---

## v5 — Advanced Ledger & Sub-Wallets

**Mission:** support complex operator business models.

- Bonus / promo wallets (wagering requirements).
- Sub-accounts for affiliates, agents, or VIP managers.
- Commission and revenue-share calculations.
- Internal transfers between users or sub-accounts.
- Refunds and adjustments with full audit trail.
- Immutable ledger exports for accounting.

---

## v6 — Reconciliation & Operations

**Mission:** operator confidence and auditability.

- Automated reconciliation: wallet balances vs. transactions vs. on-chain balances.
- Daily / weekly / monthly reconciliation reports.
- Discrepancy alerts.
- Operator dashboards for balances, deposits, withdrawals, and fees.
- Immutable audit logs for all admin actions.
- Backup and disaster-recovery procedures.

---

## v7 — Real-Time & Notifications

**Mission:** keep users and operators informed.

- Real-time balance updates via WebSocket or SSE.
- Deposit confirmation notifications.
- Withdrawal status notifications.
- Webhook events for external systems (`deposit.confirmed`, `withdrawal.completed`, etc.).
- Email / SMS notifications for withdrawals.

---

## v8 — Multi-Tenant Wallet

**Mission:** separate wallets per operator on a shared deployment.

- Tenant isolation at the database or schema level.
- Per-tenant currencies, limits, and fees.
- Operator-level reporting.
- Cross-tenant transfers (if enabled).

---

## v9 — Fiat & On/Off Ramps

**Mission:** bridge crypto and fiat.

- Fiat deposit / withdrawal provider integrations.
- Bank transfer tracking.
- Currency exchange rate service.
- Stablecoin mint/redeem hooks.

---

## v10 — Enterprise Custody

**Mission:** support institutional-grade custody.

- MPC (multi-party computation) wallet integration.
- Third-party custody provider integrations (Fireblocks, Copper, etc.).
- Multi-signature withdrawal policies.
- Hardware Security Module (HSM) support.
- Insurance and cold-storage reporting.

---

## End Goal

The Wallet microservice must eventually support the depth and reliability of leading crypto sportsbook/casino platforms (e.g., 1xBet, Stake, Roobet, Shuffle, Rainbet). That means:

- Instant deposits and fast withdrawals across many cryptocurrencies.
- Robust fee engine, automated payouts, and hot/cold wallet management.
- Full compliance and risk controls (KYC/AML, address screening, limits).
- Sub-accounts, bonus wallets, and affiliate commission tracking.
- Immutable audit trail, reconciliation, and operator reporting.
- Enterprise custody options (MPC, multi-sig, third-party custody) for operators.

## Summary Table

| Version | Focus | Key Additions |
|---|---|---|
| v1 | Foundation | USDT/USDC, xcash deposits, manual withdrawals, ledger |
| v2 | Multi-currency & limits | BTC/ETH, conversion, deposit/withdrawal limits |
| v3 | Automated withdrawals | Hot wallet, fee engine, approval workflows |
| v4 | Compliance | KYC/AML, risk screening, holds |
| v5 | Advanced ledger | Bonus wallets, sub-accounts, commissions |
| v6 | Reconciliation | Balance reconciliation, audit reports, alerts |
| v7 | Real-time | WebSocket, webhooks, notifications |
| v8 | Multi-tenant | Tenant isolation, per-tenant configs |
| v9 | Fiat on/off ramps | Fiat providers, exchange rates |
| v10 | Enterprise custody | MPC, custody providers, HSM, multi-sig |
