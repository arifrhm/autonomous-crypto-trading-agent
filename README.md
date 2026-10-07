# Autonomous Crypto Trading Agent 🤖📈

An autonomous crypto paper trading agent built in **Go 1.23+**, featuring real-time Binance WebSocket ingestion, technical analysis, LLM-powered decision making (OpenAI / Claude), paper trading execution engine, and Prometheus/Grafana observability.

Designed with clean architecture and production-grade concurrency patterns for high-frequency market streaming.

---

## 🏗️ Architecture Overview

- **Ingestion Layer**: High-resilience Binance WebSocket client streaming combined trade data with exponential backoff & keep-alive ping/pong.
- **Storage Layer**: PostgreSQL 16 for persistent trade & tick records (via `pgx/v5` batching) + Redis 7 for high-speed state cache & Pub/Sub.
- **Agentic Loop**: Observe (indicators + sentiment) → Think (structured LLM inference with guardrails) → Act (paper execution with simulated slippage & fees) → Reflect (periodic self-improvement).
- **Observability**: Native Prometheus metrics + Grafana dashboards for latency, P&L, throughput, and LLM cost tracking.

---

## 🚀 Quick Start (Local Development)

### 1. Spin up Local Infrastructure
```bash
make docker-up
```
This starts PostgreSQL (5432), Redis (6379), Prometheus (9090), and Grafana (3000).

### 2. Copy Environment Variables
```bash
cp .env.example .env
```

### 3. Run Agent
```bash
make run
```
