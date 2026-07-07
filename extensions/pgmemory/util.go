package pgmemory

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/firebase/genkit/go/ai"
)

const StoredIDKey = "stored_id"

const KnowledgeTableNameFormat = "knowledge_embeddings_%d"

const LockKeyFormat = "%s:%s:%s"

const DefaultTimeout = 30 * time.Second

var (
	ErrStoredIDNotAnInt64 = errors.New("pgmemory: stored ID is not of type int64")

	ErrDBNotInitialized = errors.New("pgmemory: database connection not initialized")

	ErrEmptyTableName = errors.New("pgmemory: table name cannot be empty")

	ErrInvalidDimension = errors.New("pgmemory: invalid dimension")
)

// GetStoredID retrieves the unique stored identifier from a message's metadata.
func GetStoredID(msg *ai.Message) (int64, error) {
	if (msg == nil) || (msg.Metadata == nil) {
		return 0, nil
	}

	v := msg.Metadata[StoredIDKey]
	switch id := v.(type) {
	case int64:
		return id, nil
	case float64:
		return int64(id), nil
	case nil:
		return 0, nil
	default:
		return 0, ErrStoredIDNotAnInt64
	}
}

// SetStoredID sets the unique stored identifier in a message's metadata.
func SetStoredID(msg *ai.Message, id int64) *ai.Message {
	if msg.Metadata == nil {
		msg.Metadata = make(map[string]any)
	}

	msg.Metadata[StoredIDKey] = id
	return msg
}

func generateLockKey(agentID, gatewayID, sessionID string) string {
	return fmt.Sprintf(LockKeyFormat, agentID, gatewayID, sessionID)
}

func knowledgeTableName(dim int) (string, error) {
	if dim <= 0 {
		return "", ErrInvalidDimension
	}

	tableName := fmt.Sprintf(KnowledgeTableNameFormat, dim)
	return tableName, nil
}

func historyTableName(name string) (string, error) {
	tableName := strings.TrimSpace(name)

	if tableName == "" {
		return "", ErrEmptyTableName
	}

	return tableName, nil
}

func calculateHash(content string) string {
	h := sha256.New()
	h.Write([]byte(content))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func documentToText(doc *ai.Document) string {
	var b strings.Builder
	for _, part := range doc.Content {
		b.WriteString(part.Text)
		b.WriteString("\n")
	}
	return b.String()
}
