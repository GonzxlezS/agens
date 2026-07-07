package agens

import (
	"encoding/json"
	"fmt"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core"
)

// DefaultToolInput represents the default input structure for tools.
// Input is the text input to be processed by the tool.
type DefaultToolInput struct {
	Input string `json:"input" jsonschema:"description=The text input to be processed by the tool."`
}

var defaultInputSchema = DefaultToolInput{}

type AgentAsToolConfig struct {
	// Name overrides the default agent name when registered as a tool.
	Name string

	// Description provides the model with context on when and how to use this tool.
	Description string

	// InputSchema defines the expected JSON schema for the tool's input.
	InputSchema map[string]any

	// OutputSchema defines the expected JSON schema for the tool's output.
	OutputSchema map[string]any

	// EnableHistoryMemory allows the underlying agent to retain session history
	// across tool invocations. If false, execution is stateless.
	EnableHistoryMemory bool

	// HandlerToolInput transforms raw tool input from the orchestrating model into an Agent Input.
	HandlerToolInput func(toolContext *ai.ToolContext, rawIn any) (*Input, error)

	// HandlerModelResponse transforms the Agent's response back into a MultipartToolResponse.
	HandlerModelResponse func(toolContext *ai.ToolContext, resp *ai.ModelResponse) (*ai.MultipartToolResponse, error)
}

// WithInputType infers and sets the JSON input schema derived from the given value.
func (cfg *AgentAsToolConfig) WithInputType(input any) *AgentAsToolConfig {
	cfg.InputSchema = core.InferSchemaMap(input)
	return cfg
}

// WithOutputType infers and sets the JSON output schema derived from the given value.
func (cfg *AgentAsToolConfig) WithOutputType(output any) *AgentAsToolConfig {
	cfg.OutputSchema = core.InferSchemaMap(output)
	return cfg
}

// AsTool converts the Agent into a Genkit MultipartTool definition, enabling it
// to be called by other models or orchestrator agents.
func (agent *Agent) AsTool(cfg *AgentAsToolConfig) *ai.ToolDef[any, *ai.MultipartToolResponse] {
	if cfg == nil {
		cfg = new(AgentAsToolConfig)
	}

	if cfg.Name == "" {
		cfg.Name = agent.cfg.Name
	}

	if cfg.Description == "" {
		cfg.Description = agent.cfg.Description
	}

	if cfg.InputSchema == nil {
		cfg = cfg.WithInputType(defaultInputSchema)
	}

	if cfg.HandlerToolInput == nil {
		cfg.HandlerToolInput = defaultHandlerToolInput
	}

	if cfg.HandlerModelResponse == nil {
		cfg.HandlerModelResponse = defaultHandlerModelResponse
	}

	return ai.DefineMultipartTool(
		agent.reg,
		cfg.Name,
		cfg.Description,
		agentAsTool(agent, cfg),
		ai.WithInputSchema(cfg.InputSchema),
	)
}

func agentAsTool(agent *Agent, cfg *AgentAsToolConfig) ai.MultipartToolFunc[any] {
	return func(toolContext *ai.ToolContext, rawIn any) (*ai.MultipartToolResponse, error) {
		in, err := cfg.HandlerToolInput(toolContext, rawIn)
		if err != nil {
			return &ai.MultipartToolResponse{}, err
		}

		// output schema
		if cfg.OutputSchema != nil {
			in.OutputSchema = cfg.OutputSchema
		}

		// ctx
		in.GatewayID, _ = GatewayIDFromContext(toolContext)
		in.SessionID, _ = SessionIDFromContext(toolContext)

		if agentID, isAgent := AgentIDFromContext(toolContext); isAgent {
			in.SenderID = agentID
		} else {
			in.SenderID, _ = SenderIDFromContext(toolContext)
		}

		// tool ctx
		genCtx := ContextWithAgentAsTool(toolContext, true)
		genCtx = ContextWithStateless(genCtx, !cfg.EnableHistoryMemory)

		// generate
		resp, err := agent.Generate(genCtx, in)
		if err != nil {
			return &ai.MultipartToolResponse{}, err
		}

		return cfg.HandlerModelResponse(toolContext, resp)
	}
}

func defaultHandlerToolInput(toolContext *ai.ToolContext, rawIn any) (*Input, error) {
	inMap, ok := rawIn.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid input type: %v", rawIn)
	}

	text, ok := inMap["input"].(string)
	if !ok {
		jsonStr, err := json.Marshal(inMap)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal dynamic tool input: %w", err)
		}

		text = string(jsonStr)
	}

	in := new(Input)
	in = in.WithMessages(ai.NewUserTextMessage(text))
	return in, nil
}

func defaultHandlerModelResponse(toolContext *ai.ToolContext, resp *ai.ModelResponse) (*ai.MultipartToolResponse, error) {
	if (resp == nil) || (resp.Message == nil) {
		return &ai.MultipartToolResponse{}, nil
	}

	toolResp := &ai.MultipartToolResponse{
		Metadata: resp.Message.Metadata,
		Output:   resp.Text(),
	}

	// Media
	for _, part := range resp.Message.Content {
		if part.IsMedia() {
			toolResp.Content = append(toolResp.Content, part)
		}
	}

	return toolResp, nil
}
