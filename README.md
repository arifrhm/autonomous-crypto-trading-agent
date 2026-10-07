# Autonomous Crypto Trading Agent 🤖📈

[![Go Version](https://img.shields.io/badge/Go-1.23%2B-00ADD8?style=flat&logo=go)](https://golang.org)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16%20%2B%20pgvector-336791?style=flat&logo=postgresql)](https://github.com/pgvector/pgvector)
[![Redis](https://img.shields.io/badge/Redis-7-DC382D?style=flat&logo=redis)](https://redis.io)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

An institutional-grade, autonomous cryptocurrency paper trading agent written in **Go 1.23+**. Combines high-throughput market data ingestion from Binance WebSocket, episodic RAG memory via PostgreSQL `pgvector`, quantitative technical indicators (in-house RSI, EMA, MACD), LLM cognitive decision loops (GPT-4o-mini / Claude 3.5), simulated paper execution with slippage/fees, and Prometheus/Grafana mission control.

Engineered with **Clean Architecture**, deterministic concurrency patterns, and production-grade resilience (Circuit Breaker, time-ordered UUIDv7, exponential backoff with full jitter).

---

## 🏗️ Architecture Overview

```mermaid
flowchart TD
    subgraph MarketIngestion["Market Data Ingestion"]
        BWS["Binance WebSocket Stream"] --> Ingest["BinanceClient with Jitter Reconnect"]
        Ingest --> TradeChan["chan domain.Trade Buffer"]
        TradeChan --> Flusher["TickBatchFlusher: 100 ticks / 1s"]
        Flusher --> PG_Ticks[("PostgreSQL: price_ticks via CopyFrom")]
    end

    subgraph EpisodicMemory["News and Episodic Memory RAG"]
        CP["CryptoPanic News API"] --> SentFetcher["SentimentFetcher Worker"]
        SentFetcher --> Embedder["OpenAI text-embedding-3-small"]
        Embedder --> VecDB[("PostgreSQL 16: pgvector HNSW Index")]
        SentFetcher --> RedisCache[("Redis 7: sentiment:symbol")]
    end

    subgraph AgenticLoop["Agentic Cognitive Loop"]
        Obs["Observe: Market Price, In-House TA, Sentiment, Position"] --> Think["Think: Vector RAG Retrieval + LLM Brain"]
        Think --> Guardrails{"Risk Guardrails: RSI, Max Position, Min Confidence"}
        Guardrails -->|Approved| Act["Act: PaperEngine with Slippage and Fees"]
        Guardrails -->|Rejected| Hold["Action: HOLD with Audit Reason"]
        Act --> DecisionAudit[("PostgreSQL: agent_decisions")]
        Act --> TradeLog[("PostgreSQL: trades")]
        Act --> Reflect["Reflect: Meta-Learning and Self-Improvement"]
        Reflect --> VecDB
    end

    subgraph ObservabilityStack["Observability"]
        Agent["Agent Runtime"] --> Prom["Prometheus Metrics :8080/metrics"]
        Prom --> Grafana["Grafana Mission Control Dashboard"]
        Prom --> Alerts["Alerting Rules: Circuit Breaker and High Latency"]
    end
```

---

## ✨ Key Technical Highlights

1. **High-Throughput WebSocket Ingestion**:
   - Combined streams (`btcusdt@trade`, `ethusdt@trade`).
   - Auto-reconnect with **Exponential Backoff + Full Jitter** (1s to 30s) preventing thundering herds.
   - Non-blocking channel buffering with backpressure handling.
2. **PostgreSQL Binary COPY Protocol (`pgx/v5`)**:
   - Ingests up to 50,000+ ticks/sec using `pool.CopyFrom` instead of standard `INSERT`, yielding 5x lower CPU overhead.
3. **Episodic Memory & Semantic Search (`pgvector`)**:
   - PostgreSQL 16 `vector(1536)` with **HNSW cosine index** (`m=16, ef_construction=64`).
   - Retrieves historical market regimes and post-trade reflections to ground LLM decisions in empirical precedent.
4. **Resilient LLM Client**:
   - State-machine **Circuit Breaker** (trips after 5 failures in 1 min, halts calls for 5 min to prevent cascading outages).
   - Redis semantic caching based on `SHA256(prompt)` with 5-minute TTL, cutting LLM inference costs by up to 80%.
5. **Time-Ordered UUIDv7 Everywhere**:
   - Strictly conforms to RFC 9562 UUIDv7 for all `trades` and `agent_decisions`, preventing B-Tree index fragmentation.
6. **Ultra-Fast Quantitative Backtest Engine**:
   - Replays 1 full year of 1-hour candles (8,760 bars) in **under 750 milliseconds**.
   - Computes Sharpe ratio, Max Drawdown, Win Rate, and Profit Factor.

---

## ⚡ Quick Start

### Prerequisites
- Docker & Docker Compose
- Go 1.23+ (optional for local binary build)

### 1. Launch Infrastructure
```bash
docker compose up -d
```
Starts:
- PostgreSQL 16 + `pgvector` on port `5432`
- Redis 7 on port `6379`
- Prometheus on port `9090`
- Grafana on port `3000` (`admin`/`admin`)

### 2. Configure Environment
```bash
cp .env.example .env
# Edit OPENAI_API_KEY if testing live LLM reasoning
```

### 3. Run the Agent
```bash
go run cmd/agent/main.go
```

### 4. Run Quantitative Backtest CLI
```bash
go run cmd/backtest/main.go --symbol BTCUSDT --candles 8760
```
Outputs `backtest_report.json` and `backtest_trades.csv`.

---

## 📊 Observability & Metrics

Prometheus metrics exposed at `http://localhost:8080/metrics`:
- `llm_calls_total{provider, model, status}`
- `llm_latency_seconds{provider, model}`
- `llm_cost_usd_total{provider, model}`
- `sentiment_score{symbol}`
- `sentiment_fetch_total{symbol, status}`

Import the pre-configured Grafana dashboard from:
`deployments/grafana/dashboards/trading_dashboard.json`

---

## 🧪 Testing

```bash
# Run unit and integration tests with Go race detector
go test -v -race ./...

# Run benchmark tests
go test -bench=. ./internal/backtest/...
```
