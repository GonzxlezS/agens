package pgmemory

import (
	"errors"
	"fmt"
	"strings"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

type (
	KnowledgeQuery struct {
		Query string `json:"query" jsonschema_description:"The specific search query or keywords to retrieve relevant information from the knowledge base. Should be clear and focused on the topic."`
	}

	DocumentResult struct {
		Label   string `json:"label" jsonschema_description:"The category or source label of the retrieved document."`
		Content string `json:"content" jsonschema_description:"The text content of the retrieved document."`
	}

	KnowledgeResponse struct {
		Results []DocumentResult `json:"results" jsonschema_description:"List of relevant documents found."`
		Count   int              `json:"count" jsonschema_description:"Number of documents retrieved. 0 if nothing was found."`
	}
)

var ErrKnowledgeMemoryFailure = fmt.Errorf("pgmemory: knowledge memory failure")

func defineTool(g *genkit.Genkit, retriever ai.Retriever, cfg *KnowledgeMemoryConfig, agentID string, limit int) ai.Tool {
	toolName := fmt.Sprintf("%s_%s_tool", agentID, cfg.Name)

	f := func(ctx *ai.ToolContext, query KnowledgeQuery) (KnowledgeResponse, error) {
		resp, err := genkit.Retrieve(
			ctx, g,
			ai.WithRetriever(retriever),
			ai.WithConfig(&RetrieveOptions{
				AgentID: agentID,
				Limit:   limit,
			}),
			ai.WithTextDocs(query.Query),
		)
		if err != nil {
			return KnowledgeResponse{}, errors.Join(ErrKnowledgeMemoryFailure, err)
		}

		kResponse := KnowledgeResponse{
			Count: len(resp.Documents),
		}

		if kResponse.Count < 1 {
			return kResponse, nil
		}

		for _, doc := range resp.Documents {
			label, _ := doc.Metadata[labelKey].(string)
			if label == "" {
				label = "unlabeled"
			}

			kResponse.Results = append(
				kResponse.Results,
				DocumentResult{
					Label:   label,
					Content: documentToText(doc),
				},
			)
		}

		return kResponse, nil
	}

	return genkit.DefineTool(g, toolName, cfg.Description, f)
}

func documentToText(doc *ai.Document) string {
	var b strings.Builder
	for _, part := range doc.Content {
		b.WriteString(part.Text)
		b.WriteString("\n")
	}
	return b.String()
}
