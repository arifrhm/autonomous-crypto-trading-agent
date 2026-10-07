-- 000002_add_pgvector_and_memories.up.sql

-- Enable pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- Table: market_memories (Vector embeddings for news, market events, and post-trade reflections)
CREATE TABLE IF NOT EXISTS market_memories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    symbol VARCHAR(20) NOT NULL,
    category VARCHAR(50) NOT NULL, -- 'NEWS', 'TRADE_REFLECTION', 'REGIME_CHANGE'
    content TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    embedding vector(1536) NOT NULL, -- OpenAI text-embedding-3-small dimension
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexing for Cosine Distance using HNSW (Hierarchical Navigable Small World) for ultra-fast ANN search
CREATE INDEX IF NOT EXISTS idx_market_memories_embedding_hnsw 
    ON market_memories USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 64);

CREATE INDEX IF NOT EXISTS idx_market_memories_symbol_category 
    ON market_memories (symbol, category, created_at DESC);
