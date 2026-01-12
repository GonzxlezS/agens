DO $$ 
DECLARE 
    dims INTEGER[] := ARRAY[384, 768, 1024, 1536];
    d INTEGER;
BEGIN 
    FOREACH d IN ARRAY dims LOOP
        EXECUTE format('DROP TABLE IF EXISTS knowledge_embeddings_%s CASCADE', d);
    END LOOP;
END $$;