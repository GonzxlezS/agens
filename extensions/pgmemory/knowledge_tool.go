package pgmemory

import (
	"errors"
	"fmt"
	"strings"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

const ToolNameFormat = "%s_%s_tool"

var ErrKnowledgeMemoryFailure = fmt.Errorf("pgmemory: knowledge memory failure")

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

func (m *KnowledgeMemory) AsTool(agentID string, limit int) ai.Tool {
	toolName := fmt.Sprintf(ToolNameFormat, agentID, m.cfg.Name)

	f := func(ctx *ai.ToolContext, query KnowledgeQuery) (KnowledgeResponse, error) {
		resp, err := m.RetrieveKnowledge(ctx, agentID, query.Query, limit)
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
			label, _ := doc.Metadata[LabelKey].(string)
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

	return genkit.DefineTool(m.g, toolName, m.cfg.Description, f)
}

func documentToText(doc *ai.Document) string {
	var b strings.Builder
	for _, part := range doc.Content {
		b.WriteString(part.Text)
		b.WriteString("\n")
	}
	return b.String()
}
