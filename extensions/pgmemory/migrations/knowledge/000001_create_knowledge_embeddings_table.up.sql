CREATE EXTENSION IF NOT EXISTS vector;

-- 384 (MiniLM), 768 (Gemini/Vertex), 1024 (BGE), 1536 (OpenAI)
DO $$ 
DECLARE 
    dims INTEGER[] := ARRAY[384, 768, 1024, 1536];
    d INTEGER;
BEGIN 
    FOREACH d IN ARRAY dims LOOP
        EXECUTE format('
            CREATE TABLE IF NOT EXISTS knowledge_embeddings_%s (
                id SERIAL PRIMARY KEY,
                agent_name TEXT NOT NULL,
                embedder_name TEXT NOT NULL,
                label TEXT NOT NULL,
                content TEXT NOT NULL,
                content_hash TEXT NOT NULL,
                embedding vector(%s) NOT NULL,
                created_at TIMESTAMP DEFAULT NOW()
            );

            CREATE INDEX IF NOT EXISTS idx_knowledge_agent_model_label_%s 
                ON knowledge_embeddings_%s (agent_name, embedder_name, label);
            
            CREATE INDEX IF NOT EXISTS idx_knowledge_lookup_%s 
                ON knowledge_embeddings_%s (agent_name, embedder_name, content_hash);
            
            CREATE INDEX IF NOT EXISTS idx_knowledge_embedding_ivfflat_%s 
                ON knowledge_embeddings_%s USING ivfflat (embedding vector_cosine_ops) 
                WITH (lists = 100);
        ', d, d, d, d, d, d, d, d);
    END LOOP;
END $$;