# BetMonster Complete Microservices Roadmap

**Goal:** lay out every microservice and major feature from v1 to the long-term vision of the platform.

**Status:** living document — will be updated as the project evolves.

---

## v1 — Wallet Foundation

**Mission:** users can deposit, hold, and withdraw USDT/USDC. Admin can review withdrawals.

| Microservice | Responsibility |
|---|---|
| **Gateway** | Public API, Better Auth JWT verification, rate limiting, routing to internal services |
| **Wallet** | Per-currency balances, deposit addresses via xcash, deposit webhooks, manual withdrawals, transaction ledger |

**Infrastructure:** Postgres (auth DB + wallet DB), Redis, NATS, Docker Compose, helper scripts (`init_env.sh`, `dev-up.sh`, `migrate.sh`, `test.sh`, `upgrade.sh`).

---

## v2 — Sportsbook & Casino Core

**Mission:** users can bet on sports and play casino games.

| Microservice | Responsibility |
|---|---|
| **Sportsbook** | Events, odds, markets, bet types (single, multi, system), live betting hooks |
| **Casino** | Game engines, provably fair RNG, house edge, crash/slots/roulette |
| **Settlement** | Bet settlement, win/loss calculation, payouts, integration with Wallet |
| **Risk** | Deposit/withdrawal limits, bet limits, geolocation, basic fraud signals |
| **Notifications** | Webhooks, emails, SMS, in-app notifications |
| **Admin** | Operator dashboard, user management, withdrawal review, game/sports management |

**Money impact:** Settlement begins debiting/crediting wallets for bets. Risk service starts enforcing limits.

---

## v3 — Operations, Reporting, and Data Feeds

**Mission:** operators can run and monitor the platform.

| Microservice | Responsibility |
|---|---|
| **Reporting** | Analytics, audit logs, compliance reports, revenue/share reports |
| **Odds/Feed** | Ingest external sports data and odds feeds, normalize events |
| **Scheduler** | Cron jobs, event start/end triggers, settlement scheduling, maintenance windows |

---

## v4 — Advanced Money & Currencies

**Mission:** support more money flows and currencies.

| Feature | Description |
|---|---|
| **Automated withdrawals** | Hot-wallet payouts after admin approval rules or thresholds |
| **Multi-currency wallets** | BTC, ETH, and other stablecoins as separate wallets |
| **Currency conversion** | Optional USD-equivalent balance or conversion between currencies |
| **Fee engine** | Configurable deposit/withdrawal/bet fees per currency and operator |

---

## v5 — Compliance & Responsible Gambling

**Mission:** protect users and operators legally.

| Feature | Description |
|---|---|
| **KYC/AML integration** | Identity verification provider hooks (Sumsub, Onfido, etc.) |
| **Geolocation** | IP/country blocking, VPN detection, jurisdiction enforcement |
| **Self-exclusion** | User-initiated cool-off periods and permanent exclusion |
| **Deposit/loss limits** | Daily/weekly/monthly limits per user, with hard caps |
| **Audit trails** | Immutable logs for all admin actions and financial events |

---

## v6 — Multi-Tenant & Operator Platform

**Mission:** support multiple operators/merchants on a single deployment.

| Feature | Description |
|---|---|
| **Multi-tenancy** | Separate branding, currencies, limits, and users per operator |
| **Operator roles** | Owner, manager, finance, support, fraud analyst |
| **Revenue sharing** | Configurable revenue share and fee splits between operators and platform |
| **Sub-accounts** | Segregated wallets for affiliates, agents, or VIP managers |

---

## v7 — Player Experience

**Mission:** modern, mobile-first user experience.

| Feature | Description |
|---|---|
| **PWA** | Installable progressive web app with offline support for static pages |
| **Mobile apps** | Optional native wrappers (React Native, Capacitor) |
| **Live chat/support** | In-app support ticketing and chat |
| **Bonuses/promotions** | Bonus wallets, wagering requirements, free bets, rakeback |

---

## v8 — Advanced Risk & Fraud

**Mission:** protect against abuse and coordinated attacks.

| Feature | Description |
|---|---|
| **Real-time risk scoring** | Behavior-based scoring for deposits, withdrawals, and bets |
| **Sybil detection** | Multi-account detection, device fingerprinting, collusion rings |
| **Anomaly detection** | ML-based alerts for unusual bet patterns or deposit flows |
| **Automated compliance actions** | Auto-freeze, auto-review, auto-ban based on risk thresholds |

---

## v9 — Liquidity & Ecosystem

**Mission:** scale beyond a single operator.

| Feature | Description |
|---|---|
| **Liquidity pool** | Shared pool for automated payouts across operators |
| **Exchange integrations** | On/off ramps via exchanges, OTC desks, or third-party providers |
| **Affiliate network** | Referral tracking, commission engine, affiliate dashboards |
| **API marketplace** | Public APIs for third-party bots, tipsters, and traders |

---

## v10 — Enterprise & White-Label

**Mission:** become the "xcash for betting" — a fully deployable platform anyone can run.

| Feature | Description |
|---|---|
| **White-label deployments** | One-click branded deployments for new operators |
| **Marketplace** | Pre-built casino games, sports feeds, and compliance plugins |
| **Custody options** | Self-custody, MPC custody, or third-party custody integrations |
| **Governance/forking** | Clean module boundaries so operators can fork and customize safely |

---

## v11+ — Long-Term Vision

- **Decentralized betting primitives** — smart-contract-based settlement oracles for transparency.
- **Cross-chain support** — beyond EVM and Tron to Solana, Bitcoin Lightning, etc.
- **AI-powered personalization** — personalized odds, game recommendations, and risk profiles.
- **Global licensing framework** — built-in compliance modules for major jurisdictions.
- **Open-source marketplace** — community-contributed games, sports feeds, and risk models.

---

## End Goal

BetMonster aims to be an open-source, self-hosted alternative to the leading crypto sportsbook/casino platforms (e.g., 1xBet, Stake, Roobet, Shuffle, Rainbet). The end state includes:

- Full sportsbook: live events, pre-match, in-play, multiple bet types, cash-out, live odds feeds.
- Full casino: slots, table games, crash, live dealer, provably fair games, house-edge management.
- Player features: bonuses, rakeback, VIP levels, leaderboards, tournaments, PWA/mobile.
- Operator features: multi-tenant white-label, revenue sharing, analytics, compliance, risk management.
- Enterprise features: liquidity pools, custody integrations, fiat on/off ramps, cross-chain support.

## Summary Table

| Version | Focus | New Services |
|---|---|---|
| v1 | Wallet foundation | Gateway, Wallet |
| v2 | Sportsbook & casino | Sportsbook, Casino, Settlement, Risk, Notifications, Admin |
| v3 | Operations & data | Reporting, Odds/Feed, Scheduler |
| v4 | Advanced money | Automated withdrawals, multi-currency, fee engine |
| v5 | Compliance | KYC/AML, geolocation, self-exclusion, limits |
| v6 | Multi-tenant | Multi-tenancy, operator roles, revenue sharing |
| v7 | Player UX | PWA, mobile apps, bonuses, support |
| v8 | Risk & fraud | Real-time scoring, sybil detection, anomaly detection |
| v9 | Liquidity & ecosystem | Liquidity pool, exchange integrations, affiliate network, APIs |
| v10 | Enterprise white-label | White-label deployments, marketplace, custody options |
| v11+ | Decentralization & scale | Cross-chain, smart-contract settlement, AI personalization |
