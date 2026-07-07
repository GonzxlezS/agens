package pgmemory

import (
	"errors"
	"fmt"

	"github.com/firebase/genkit/go/ai"
)

const ToolNameFormat = "%s_%s_tool"

var ErrKnowledgeMemoryFailure = fmt.Errorf("pgmemory: knowledge memory failure")

type (
	KnowledgeQuery struct {
		Query string `json:"query" jsonschema_description:"The specific search query or keywords to retrieve relevant information from the knowledge base. Should be clear and focused on the topic."`
	}

	KnowledgeResponse struct {
		Documents []*ai.Document `json:"documents" jsonschema_description:"List of relevant documents found."`
		Count     int            `json:"count" jsonschema_description:"Number of documents retrieved. 0 if nothing was found."`
	}
)

func (m *KnowledgeMemory) AsTool(agentID string, limit int) ai.Tool {
	if m.tool != nil {
		return m.tool
	}

	fn := func(ctx *ai.ToolContext, query KnowledgeQuery) (KnowledgeResponse, error) {
		resp := KnowledgeResponse{}

		docs, err := m.RetrieveKnowledge(ctx, agentID, query.Query, limit)
		if err != nil {
			return resp, errors.Join(ErrKnowledgeMemoryFailure, err)
		}

		resp.Documents = docs
		resp.Count = len(docs)
		return resp, nil
	}

	m.tool = ai.DefineTool(m.reg, fmt.Sprintf(ToolNameFormat, agentID, m.cfg.Name), m.cfg.Description, fn)
	return m.tool
}
