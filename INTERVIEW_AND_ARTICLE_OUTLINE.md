# Senior Backend Engineer (Binance) — Project Interview Prep & Article Outline

## 🎯 Top 5 Interview Questions for This Project

### Q1: "Mengapa Anda memilih `pgx/v5` dengan `CopyFrom` untuk ingestion data tick, bukan `database/sql` standar atau TimescaleDB?"
**Jawaban Ideal**:  
"Pada frekuensi data market Binance WebSocket yang bisa mencapai ribuan trade per detik saat volatilitas tinggi, `database/sql` menimbulkan bottleneck karena alokasi memori query string, text protocol parsing, dan interface reflection. `pgx/v5` berkomunikasi langsung melalui PostgreSQL native binary protocol. Dengan metode `pool.CopyFrom`, kita memanfaatkan binary streaming protocol PostgreSQL (`COPY FROM STDIN`) yang langsung menulis data ke page file tanpa overhead parsing SQL statement per row. Kami mencapai throughput 50,000+ ticks/detik dengan konsumsi CPU rendah tanpa perlu menambah ketergantungan ekstensi TimescaleDB berbayar atau klaster database terpisah."

---

### Q2: "Mengapa Anda mengganti UUIDv4 menjadi UUIDv7 untuk entity `trades` dan `agent_decisions`?"
**Jawaban Ideal**:  
"UUIDv4 dibuat dari 128 bit acak murni. Di PostgreSQL, primary key index menggunakan struktur B-Tree. Ketika jutaan UUIDv4 di-insert pada sistem trading yang aktif, data baru jatuh di sembarang daun B-Tree secara acak, menyebabkan fenomena *random page split*, fragmentasi index, dan lonjakan I/O disk (cache thrashing). UUIDv7 (RFC 9562) menyematkan UNIX epoch 48-bit di awal identifier. Hal ini membuat ID selalu terurut secara kronologis (*time-ordered sequential*). Data selalu di-append di ujung kanan pohon B-Tree, menghasilkan kerapatan index 99% dan mencegah degradasi performa jangka panjang."

---

### Q3: "Bagaimana sistem Anda mencegah cascading failures ketika OpenAI atau Anthropic API mengalami rate limit 429 atau downtime 500?"
**Jawaban Ideal**:  
"Kami menerapkan arsitektur pertahanan berlapis:
1. **Exponential Backoff dengan Jitter**: Retry cerdas pada status code transient (429, 500, 503) agar tidak membanjiri upstream API.
2. **Circuit Breaker Pattern**: State-machine berbasis `sync.Mutex` yang melacak error window. Jika terjadi lebih dari 5 kegagalan berturut-turut dalam 1 menit, breaker berpindah ke status `OPEN`, menolak panggilan selama 5 menit (*cooldown*), dan sistem langsung beralih ke rule-based fallback decision (HOLD) tanpa memblokir perputaran goroutine loop.
3. **Redis Semantic Caching**: Hash `SHA256(prompt + context)` di-cache selama 5 menit. Jika kondisi pasar belum berubah drastis, respon LLM dilayani langsung dari Redis cache (latensi <2ms, biaya token $0)."

---

### Q4: "Bagaimana cara kerja Vector Search di arsitektur ini dan mengapa memakai `pgvector`?"
**Jawaban Ideal**:  
"Daripada membebani infrastruktur dengan database vector terpisah seperti Pinecone atau Qdrant yang menambah latency network hop dan biaya ekstra, kami memanfaatkan ekstensi `pgvector` langsung di dalam PostgreSQL 16. Setiap berita dan refleksi post-trade diubah menjadi vector embedding 1536-dimensi via `text-embedding-3-small`. Index yang digunakan adalah HNSW (*Hierarchical Navigable Small World*) dengan operator cosine distance (`vector_cosine_ops`). Saat agen melakukan fase `Think`, agen mencari 2 preseden pasar paling relevan di masa lalu dan menyuntikkannya ke prompt context (RAG). Keuntungan terbesarnya adalah konsistensi ACID transaksional—kami bisa memfilter metadata relasional dan semantic distance dalam 1 query SQL terpadu."

---

### Q5: "Bagaimana Anda memastikan goroutine tidak mengalami memory leak saat WebSocket stream terputus?"
**Jawaban Ideal**:  
"Seluruh goroutine terikat pada root `context.Context` yang mendengarkan OS signal (`SIGINT`, `SIGTERM`). Di layer WebSocket (`BinanceClient`), saat error koneksi terjadi atau context di-cancel, loop reader mengirim `CloseNormalClosure`, menutup network socket, dan menutup output channel (`defer close(c.tradeCh)`). Worker `TickBatchFlusher` mendengarkan `tradeCh`; ketika channel ditutup atau context selesai, ia melakukan graceful final flush untuk sisa buffer di memori menggunakan timeout terpisah 5 detik sebelum goroutine berakhir."

---

## 📝 Article Outline (Medium / Dev.to)

**Judul**: *Building an Institutional-Grade Autonomous Crypto Trading Agent in Go with pgvector, LLM Reasoning, and Clean Architecture*

1. **The Problem Statement**: Mengapa bot trading berbasis skrip Python sederhana sering gagal di pasar crypto frekuensi tinggi (GIL locking, overhead memory, kurangnya episodic memory).
2. **Clean Architecture in Go 1.23**: Memisahkan Domain, Ingestion, Agent Kognitif, dan Storage.
3. **High-Throughput Streaming**: Menangani WebSocket Binance dengan Exponential Backoff + Full Jitter & Postgres Binary `CopyFrom`.
4. **Giving LLMs an Episodic Memory (RAG with pgvector & HNSW)**: Mengapa model bahasa butuh preseden historis agar tidak impulsif.
5. **The Agentic Loop (Observe -> Think -> Act -> Reflect)**: Implementasi in-house TA (RSI, EMA, MACD) dan guardrails risiko.
6. **Hardcore Engineering Decisions**: Mengapa UUIDv7, Circuit Breaker, dan Redis Hash Caching krusial.
7. **Backtesting & Benchmark Results**: Replay 8,760 candle dalam 723ms.
8. **Conclusion & GitHub Repo Link**.
