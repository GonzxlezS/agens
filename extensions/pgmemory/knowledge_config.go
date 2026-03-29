package pgmemory

import "github.com/firebase/genkit/go/ai"

type KnowledgeMemoryConfig struct {
	Name        string
	Description string

	Embedder   ai.EmbedderArg
	Dimensions int

	RetrieverOptions *ai.RetrieverOptions
	EmbedderOptions  []ai.EmbedderOption
}

func (cfg *KnowledgeMemoryConfig) embedderOptions(additionalOptions ...ai.EmbedderOption) []ai.EmbedderOption {
	embedderOpts := make([]ai.EmbedderOption, 0, len(cfg.EmbedderOptions)+len(additionalOptions)+1)

	embedderOpts = append(embedderOpts, cfg.EmbedderOptions...)
	embedderOpts = append(embedderOpts, additionalOptions...)

	if cfg.Embedder != nil {
		embedderOpts = append(embedderOpts, ai.WithEmbedder(cfg.Embedder))
	}
	return embedderOpts
}
