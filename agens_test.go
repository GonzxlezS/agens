package agens

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core"
	"github.com/firebase/genkit/go/core/api"
	"github.com/firebase/genkit/go/genkit"
)

var _ KnowledgeMemory = &mockMemory{}
var _ HistoryMemory = &mockMemory{}
var _ HistoryManager = &mockMemory{}
var _ KnowledgeQueryer = &mockMemory{}

// TESTS
func TestNewAgent(t *testing.T) {
	t.Run("ErrAgentIDEmpty", func(t *testing.T) {
		_, err := NewAgent(AgentConfig{})
		if !errors.Is(err, ErrAgentIDEmpty) {
			t.Fatal(err)
		}
	})

	t.Run("ErrAgentNameEmpty", func(t *testing.T) {
		_, err := NewAgent(AgentConfig{ID: "021"})
		if !errors.Is(err, ErrAgentNameEmpty) {
			t.Fatal(err)
		}
	})

	t.Run("ErrAgentDescriptionEmpty", func(t *testing.T) {
		_, err := NewAgent(AgentConfig{ID: "021", Name: "e21"})
		if !errors.Is(err, ErrAgentDescriptionEmpty) {
			t.Fatal(err)
		}
	})

	t.Run("ErrAgentInstructionsEmpty", func(t *testing.T) {
		_, err := NewAgent(AgentConfig{ID: "021", Name: "e21", Description: "ai"})
		if !errors.Is(err, ErrAgentInstructionsEmpty) {
			t.Fatal(err)
		}
	})

	t.Run("ErrAgentInstructionsEmptyStr", func(t *testing.T) {
		_, err := NewAgent(AgentConfig{
			ID:          "021",
			Name:        "e21",
			Description: "ai",
			Instructions: []string{
				"",
			},
		})

		if !errors.Is(err, ErrAgentInstructionsEmpty) {
			t.Fatal(err)
		}
	})

	cfg := AgentConfig{
		ID:          " 021 ",
		Name:        "e21  ",
		Description: "   ai",
		Instructions: []string{
			"test",
		},
	}

	agent, err := NewAgent(cfg)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("id", func(t *testing.T) {
		if agent.ID() != "021" {
			t.Fatal("ID must be 021")
		}
	})

	t.Run("systemPrompt", func(t *testing.T) {
		var (
			in  = &Input{}
			ctx = context.Background()
		)

		agentSystem, err := agent.cfg.SystemPrompt(context.Background(), in)
		if err != nil {
			t.Fatal(err)
		}

		system, err := DefaultSystemPromptFn(ctx, agent.cfg, in)
		if err != nil {
			t.Fatal(err)
		}

		if agentSystem != system {
			t.Fatal("invalid system prompt")
		}
	})

	t.Run("ErrAgentRegistryNil", func(t *testing.T) {
		_, err := agent.Generate(context.Background(), &Input{})
		if !errors.Is(err, ErrAgentRegistryNil) {
			t.Fatal(err)
		}
	})

	t.Run("register", func(t *testing.T) {
		var (
			ctx = context.Background()
			g   = genkit.Init(ctx)
		)

		genkit.RegisterAction(g, agent)

		if agent.reg == nil {
			t.Fatal("error register")
		}
	})
}

func TestEmptyAgent(t *testing.T) {
	agent := &Agent{}

	t.Run("id", func(tt *testing.T) {
		if agent.ID() != "" {
			tt.Fatal("ID must be empty")
		}
	})

	t.Run("generate", func(tt *testing.T) {
		_, err := agent.Generate(context.Background(), &Input{})
		if !errors.Is(err, ErrAgentNotInitialized) {
			tt.Fatal(err)
		}
	})
}

func TestAgent(t *testing.T) {
	var (
		ctx = context.Background()

		g = genkit.Init(ctx)

		errSystemTest = errors.New("system err")
	)

	t.Run("errSystemPrompt", func(t *testing.T) {
		var (
			cfg = AgentConfig{
				ID:          "agent01",
				Name:        "e21",
				Description: "ai",
				Instructions: []string{
					"test",
				},
				SystemPromptFn: func(_ context.Context, _ *AgentConfig, _ *Input) (string, error) {
					return "", errSystemTest
				},
			}

			in = &Input{
				Messages: []*ai.Message{},
			}
		)

		agent, err := NewAgent(cfg)
		if err != nil {
			t.Fatal(err)
		}

		genkit.RegisterAction(g, agent)

		_, err = agent.Generate(ctx, in)
		if !errors.Is(err, errSystemTest) {
			t.Fatal(err)
		}
	})

	t.Run("historyErr", func(t *testing.T) {
		var (
			cfg = AgentConfig{
				ID:          "agent02",
				Name:        "e21",
				Description: "ai",
				Instructions: []string{
					"test",
				},
				HistoryMemory: &mockMemory{
					agentID: "agent02",
				},
			}

			in = &Input{
				GatewayID: "test",
				SessionID: "retrieve_err",
				Messages:  []*ai.Message{},
			}
		)

		agent, err := NewAgent(cfg)
		if err != nil {
			t.Fatal(err)
		}

		genkit.RegisterAction(g, agent)

		_, err = agent.Generate(ctx, in)
		if err.Error() != "retrieve err" {
			t.Fatal(err)
		}
	})

	t.Run("knowledgeErr", func(t *testing.T) {
		var (
			kMemory = &mockMemory{
				agentID: "agent03",
			}

			cfg = AgentConfig{
				ID:          "agent03",
				Name:        "e21",
				Description: "ai",
				Instructions: []string{
					"test",
				},
				KnowledgeMemory:  kMemory,
				KnowledgeQueryer: kMemory,
			}

			in = &Input{
				GatewayID: "test",
				SessionID: "212121",
				Messages:  []*ai.Message{},
			}
		)

		agent, err := NewAgent(cfg)
		if err != nil {
			t.Fatal(err)
		}

		genkit.RegisterAction(g, agent)

		ctx = context.WithValue(ctx, "knowledge_err", true)
		_, err = agent.Generate(ctx, in)
		if err.Error() != "knowledge err" {
			t.Fatal(err)
		}
	})
}

func TestConfig(t *testing.T) {
	t.Run("genFlow_err", func(t *testing.T) {
		cfg := &AgentConfig{}

		_, err := cfg.genFlow(context.TODO(), nil, nil)
		if !errors.Is(err, ErrAgentIDEmpty) {
			t.Fatal(err)
		}
	})

	t.Run("genOptions", func(t *testing.T) {
		var emptyIn = &Input{}

		tests := []struct {
			name    string
			cfg     *AgentConfig
			in      *Input
			history []*ai.Message
			docs    []*ai.Document
			len     int
		}{
			{
				name:    "system",
				cfg:     &AgentConfig{},
				in:      emptyIn,
				history: nil,
				len:     1,
			},
			{
				name: "base",
				cfg: &AgentConfig{
					GenerateOptions: []ai.GenerateOption{
						ai.WithConfig(struct{}{}),
					},
				},
				in:      emptyIn,
				history: nil,
				len:     2,
			},
			{
				name: "model",
				cfg: &AgentConfig{
					Model: ai.NewModelRef("test", nil),
					GenerateOptions: []ai.GenerateOption{
						ai.WithConfig(struct{}{}),
					},
				},
				in:      emptyIn,
				history: nil,
				len:     3,
			},
			{
				name: "knowledge",
				cfg: &AgentConfig{
					Model:           ai.NewModelRef("test", nil),
					KnowledgeMode:   KnowledgeModeTool,
					KnowledgeMemory: &mockMemory{},
					GenerateOptions: []ai.GenerateOption{
						ai.WithConfig(struct{}{}),
					},
				},
				in:      emptyIn,
				history: nil,
				len:     4,
			},
			{
				name: "maxTurns",
				cfg: &AgentConfig{
					Model:           ai.NewModelRef("test", nil),
					KnowledgeMode:   KnowledgeModeTool,
					KnowledgeMemory: &mockMemory{},
					MaxTurns:        5,
					GenerateOptions: []ai.GenerateOption{
						ai.WithConfig(struct{}{}),
					},
				},
				in:      emptyIn,
				history: nil,
				len:     5,
			},
			{
				name: "history",
				cfg: &AgentConfig{
					Model:           ai.NewModelRef("test", nil),
					KnowledgeMode:   KnowledgeModeTool,
					KnowledgeMemory: &mockMemory{},
					MaxTurns:        5,
					GenerateOptions: []ai.GenerateOption{
						ai.WithConfig(struct{}{}),
					},
				},
				in: emptyIn,
				history: []*ai.Message{
					ai.NewUserTextMessage("test"),
				},
				len: 6,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				opts := tt.cfg.genOptions(tt.in, "test system", tt.history, tt.docs)

				if len(opts) != tt.len {
					t.Fatalf("We expected %d; we got %d.", tt.len, len(opts))
				}
			})
		}
	})

	t.Run("customSystemPrompt", func(tt *testing.T) {
		cfg := AgentConfig{
			SystemPromptFn: func(_ context.Context, _ *AgentConfig, _ *Input) (string, error) {
				return "test", nil
			},
		}

		cfgSystem, err := cfg.SystemPrompt(context.Background(), &Input{})
		if err != nil {
			tt.Fatal(err)
		}

		if cfgSystem != "test" {
			tt.Fatal("invalid system prompt")
		}
	})

	tests := []struct {
		name string
		cfg  *AgentConfig
		err  error
	}{
		{
			name: "ErrHistoryManagerRequired",
			cfg: &AgentConfig{
				ID:          "021",
				Name:        "e21",
				Description: "ai",
				Instructions: []string{
					"test",
				},
				HistoryMemory: &mockMemory{},
			},
			err: ErrHistoryManagerRequired,
		},
		{
			name: "ErrInvalidKnowledgeMode",
			cfg: &AgentConfig{
				ID:          "021",
				Name:        "e21",
				Description: "ai",
				Instructions: []string{
					"test",
				},
				KnowledgeMode: "error",
			},
			err: ErrInvalidKnowledgeMode,
		},
		{
			name: "ErrKnowledgeMemoryRequired_tool",
			cfg: &AgentConfig{
				ID:          "021",
				Name:        "e21",
				Description: "ai",
				Instructions: []string{
					"test",
				},
				KnowledgeMode: KnowledgeModeTool,
			},
			err: ErrKnowledgeMemoryRequired,
		},
		{
			name: "ErrKnowledgeMemoryRequired",
			cfg: &AgentConfig{
				ID:          "021",
				Name:        "e21",
				Description: "ai",
				Instructions: []string{
					"test",
				},
				KnowledgeMode: KnowledgeModeStep,
			},
			err: ErrKnowledgeMemoryRequired,
		},
		{
			name: "ErrKnowledgeQueryerRequired",
			cfg: &AgentConfig{
				ID:          "021",
				Name:        "e21",
				Description: "ai",
				Instructions: []string{
					"test",
				},
				KnowledgeMode:   KnowledgeModeStep,
				KnowledgeMemory: &mockMemory{},
			},
			err: ErrKnowledgeQueryerRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if !errors.Is(err, tt.err) {
				t.Fatal(err)
			}
		})
	}
}

func TestHistoryMemory(t *testing.T) {
	var (
		ctx = context.Background()

		cfg = AgentConfig{
			ID:          "021",
			Name:        "e21",
			Description: "ai",
			Instructions: []string{
				"test",
			},
			HistoryManager: &mockMemory{}, // manager
		}

		msgs = []*ai.Message{
			ai.NewUserTextMessage("test"),
		}

		memory = &mockMemory{
			agentID: "021",
			msgs:    msgs,
		}

		retrieveFlow = genkit.NewFlow("testHistoryMemoryRetrieve", func(ctx context.Context, in struct {
			gatewayID string
			sessionID string
			stateless bool
		}) ([]*ai.Message, error) {
			return cfg.retrieveHistory(ctx, in.gatewayID, in.sessionID, in.stateless)
		})

		storeFlow = genkit.NewFlow("testHistoryMemoryStore", func(ctx context.Context, in struct {
			gatewayID string
			sessionID string
			stateless bool
			messages  []*ai.Message
		}) (struct{}, error) {
			return struct{}{}, cfg.storeHistory(ctx, in.gatewayID, in.sessionID, in.stateless, in.messages)
		})
	)

	t.Run("emptyRetrieve", func(t *testing.T) {
		in := struct {
			gatewayID string
			sessionID string
			stateless bool
		}{
			gatewayID: "TEST",
			sessionID: "test",
		}

		result, err := retrieveFlow.Run(ctx, in)
		if err != nil {
			t.Fatal(err)
		}

		if len(result) != 0 {
			t.Fatal("history must be nil")
		}
	})

	t.Run("emptyStore", func(t *testing.T) {
		in := struct {
			gatewayID string
			sessionID string
			stateless bool
			messages  []*ai.Message
		}{
			gatewayID: "TEST",
			sessionID: "test",
			messages:  []*ai.Message{},
		}

		_, err := storeFlow.Run(ctx, in)
		if err != nil {
			t.Fatal(err)
		}
	})

	// memory
	cfg.HistoryMemory = memory

	t.Run("retrieve", func(t *testing.T) {
		in := struct {
			gatewayID string
			sessionID string
			stateless bool
		}{
			gatewayID: "TEST",
			sessionID: "test",
		}

		result, err := retrieveFlow.Run(ctx, in)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(result, msgs) {
			t.Fatal("invalid messages")
		}
	})

	t.Run("store", func(t *testing.T) {
		in := struct {
			gatewayID string
			sessionID string
			stateless bool
			messages  []*ai.Message
		}{
			gatewayID: "TEST",
			sessionID: "test",
			messages:  []*ai.Message{},
		}

		_, err := storeFlow.Run(ctx, in)
		if err != nil {
			t.Fatal(err)
		}
	})

	// anonymous
	t.Run("anonymous_retrieve", func(t *testing.T) {
		in := struct {
			gatewayID string
			sessionID string
			stateless bool
		}{
			gatewayID: "",
			sessionID: "test",
		}

		result, err := retrieveFlow.Run(ctx, in)
		if err != nil {
			t.Fatal(err)
		}

		if len(result) != 0 {
			t.Fatal("history must be empty")
		}
	})

	t.Run("anonymous_store", func(t *testing.T) {
		in := struct {
			gatewayID string
			sessionID string
			stateless bool
			messages  []*ai.Message
		}{
			gatewayID: "TEST",
			sessionID: "",
			messages:  []*ai.Message{},
		}

		_, err := storeFlow.Run(ctx, in)
		if err != nil {
			t.Fatal(err)
		}
	})

	// stateless
	t.Run("stateless_retrieve", func(t *testing.T) {
		in := struct {
			gatewayID string
			sessionID string
			stateless bool
		}{
			gatewayID: "TEST",
			sessionID: "test",
			stateless: true,
		}

		result, err := retrieveFlow.Run(ctx, in)
		if err != nil {
			t.Fatal(err)
		}

		if len(result) != 0 {
			t.Fatal("history must be empty")
		}
	})

	t.Run("stateless_store", func(t *testing.T) {
		in := struct {
			gatewayID string
			sessionID string
			stateless bool
			messages  []*ai.Message
		}{
			gatewayID: "TEST",
			sessionID: "test",
			stateless: true,
			messages:  []*ai.Message{},
		}

		_, err := storeFlow.Run(ctx, in)
		if err != nil {
			t.Fatal(err)
		}
	})

	// err
	t.Run("retrieve_err", func(t *testing.T) {
		in := struct {
			gatewayID string
			sessionID string
			stateless bool
		}{
			gatewayID: "TEST",
			sessionID: "retrieve_err",
		}

		_, err := retrieveFlow.Run(ctx, in)
		if err == nil {
			t.Fatal("an error was expected")
		}
	})

	t.Run("store_err", func(t *testing.T) {
		in := struct {
			gatewayID string
			sessionID string
			stateless bool
			messages  []*ai.Message
		}{
			gatewayID: "TEST",
			sessionID: "store_err",
			messages:  []*ai.Message{},
		}

		_, err := storeFlow.Run(ctx, in)
		if err == nil {
			t.Fatal("an error was expected")
		}
	})

	// history manager err
	t.Run("history_manager_err", func(t *testing.T) {
		ctx = context.WithValue(ctx, "manager_err", true)

		in := struct {
			gatewayID string
			sessionID string
			stateless bool
			messages  []*ai.Message
		}{
			gatewayID: "TEST",
			sessionID: "test",
			messages:  []*ai.Message{},
		}

		_, err := storeFlow.Run(ctx, in)
		if err == nil {
			t.Fatal("an error was expected")
		}
	})
}

func TestKnowledgeMemory(t *testing.T) {
	var (
		cfg = AgentConfig{
			ID:          "021",
			Name:        "e21",
			Description: "ai",
			Instructions: []string{
				"test",
			},
		}

		kMemory = &mockMemory{
			agentID: "021",
			docs:    []*ai.Document{ai.DocumentFromText("test", nil)},
		}

		retrieveFlow = genkit.NewFlow("testRetrieveKnowledge",
			func(ctx context.Context, in *Input) ([]*ai.Document, error) {
				return cfg.retrieveKnowledge(ctx, in)
			},
		)

		in = &Input{
			GatewayID: "test",
			SessionID: "21",
			SenderID:  "batman",
		}
	)

	cfg.KnowledgeMemory = kMemory
	cfg.KnowledgeQueryer = kMemory
	cfg.KnowledgeMode = KnowledgeModeTool

	t.Run("KnowledgeModeTool", func(t *testing.T) {
		ctx := context.Background()

		result, err := retrieveFlow.Run(ctx, in)
		if err != nil {
			t.Fatal(err)
		}

		if len(result) != 0 {
			t.Fatal("docs must be empty")
		}
	})

	cfg.KnowledgeMode = KnowledgeModeStep

	t.Run("toolRestarts", func(t *testing.T) {
		ctx := context.Background()

		in := &Input{
			GatewayID: "test",
			SessionID: "21",
			SenderID:  "batman",
		}

		in = in.WithToolRestarts(ai.NewTextPart("test"))

		result, err := retrieveFlow.Run(ctx, in)
		if err != nil {
			t.Fatal(err)
		}

		if len(result) != 0 {
			t.Fatal("history must be nil")
		}
	})

	t.Run("queryerErr", func(t *testing.T) {
		ctx := context.Background()
		ctx = context.WithValue(ctx, "queryer_err", true)

		_, err := retrieveFlow.Run(ctx, in)
		if err == nil {
			t.Fatal("queryer_err error was expected")
		}
	})

	t.Run("knowledgeErr", func(t *testing.T) {
		ctx := context.Background()
		ctx = context.WithValue(ctx, "knowledge_err", true)

		_, err := retrieveFlow.Run(ctx, in)
		if err == nil {
			t.Fatal("knowledge_err error was expected")
		}
	})

	t.Run("ok", func(t *testing.T) {
		ctx := context.Background()

		result, err := retrieveFlow.Run(ctx, in)
		if err != nil {
			t.Fatal(err)
		}

		if len(result) != 1 {
			t.Fatal("docs must be 1")
		}
	})
}

func TestInput(t *testing.T) {
	tests := []struct {
		name    string
		inFn    func(preIn *Input) *Input
		history []*ai.Message
		docs    []*ai.Document
		len     int
	}{
		{
			name: "empty",
			inFn: func(_ *Input) *Input {
				return &Input{}
			},
		},
		{
			name: "metadata",
			inFn: func(_ *Input) *Input {
				return &Input{
					GatewayID: "test",
					SessionID: "21",
					SenderID:  "batman",
				}
			},
		},
		{
			name:    "history",
			history: []*ai.Message{ai.NewUserTextMessage("test")},
			len:     1,
		},
		{
			name: "WithMessages",
			inFn: func(preIn *Input) *Input {
				in := preIn.WithMessages(ai.NewUserTextMessage("test"))
				return in
			},
			len: 1,
		},
		{
			name: "docs",
			docs: []*ai.Document{ai.DocumentFromText("test", nil)},
			len:  2,
		},
		{
			name: "WithDocs",
			inFn: func(preIn *Input) *Input {
				in := preIn.WithDocs(ai.DocumentFromText("test", nil))
				return in
			},
			len: 2,
		},
		{
			name: "WithToolChoice",
			inFn: func(preIn *Input) *Input {
				in := preIn.WithToolChoice(ai.ToolChoiceRequired)
				return in
			},
			len: 3,
		},
		{
			name: "WithReturnToolRequests",
			inFn: func(preIn *Input) *Input {
				in := preIn.WithReturnToolRequests(true)
				return in
			},
			len: 4,
		},
		{
			name: "WithToolResponses",
			inFn: func(preIn *Input) *Input {
				in := preIn.WithToolResponses(ai.NewTextPart("test"))
				return in
			},
			len: 5,
		},
		{
			name: "WithToolRestarts",
			inFn: func(preIn *Input) *Input {
				in := preIn.WithToolRestarts(ai.NewTextPart("test"))
				return in
			},
			len: 6,
		},
		{
			name: "WithOutputTypeAndWithOutputFormat",
			inFn: func(preIn *Input) *Input {
				in := preIn.WithOutputType(struct{ name string }{name: "test"})
				return in
			},
			len: 8,
		},
		{
			name: "WithOutputEnums",
			inFn: func(preIn *Input) *Input {
				in := preIn.WithOutputEnums("test", "pass")
				return in
			},
			len: 8,
		},
		{
			name: "WithOutputInstructions",
			inFn: func(preIn *Input) *Input {
				in := preIn.WithOutputInstructions("test")
				return in
			},
			len: 9,
		},
	}

	var in = new(Input)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.inFn != nil {
				in = tt.inFn(in)
			}

			opts := in.genOptions(tt.history, tt.docs)

			if len(opts) != tt.len {
				t.Fatalf("We expected %d; we got %d.", tt.len, len(opts))
			}
		})
	}
}

func TestSlidingWindowManager(t *testing.T) {
	var (
		manager = SlidingWindowManager{
			WindowSize: 3,
		}

		msg1 = ai.NewUserTextMessage("test")
		msg2 = ai.NewUserTextMessage("test")

		model1 = ai.NewModelTextMessage("test")

		toolReq = ai.NewModelMessage(
			ai.NewToolRequestPart(&ai.ToolRequest{}),
		)

		toolRes = ai.NewMessage(ai.RoleTool, nil,
			ai.NewToolResponsePart(&ai.ToolResponse{}),
		)
	)

	tests := []struct {
		in  []*ai.Message
		out []*ai.Message
	}{
		// pass
		{
			in:  []*ai.Message{msg1},
			out: []*ai.Message{msg1},
		},

		// cut
		{
			in:  []*ai.Message{msg1, model1, msg2, toolReq},
			out: []*ai.Message{model1, msg2, toolReq},
		},

		// cut 2
		{
			in:  []*ai.Message{msg1, toolReq, toolRes, model1, msg2},
			out: []*ai.Message{toolReq, toolRes, model1, msg2},
		},
	}

	for _, test := range tests {
		msgs, err := manager.ProcessHistory(
			context.Background(),
			test.in,
		)

		if err != nil {
			t.Fatal(err)
		}

		if !slices.Equal(msgs, test.out) {
			t.Fatal("invalid out")
		}
	}
}

func TestSlidingWindowManager_EdgeCases(t *testing.T) {
	manager := SlidingWindowManager{WindowSize: 3}

	// Nil history
	_, err := manager.ProcessHistory(context.Background(), nil)
	if err != nil {
		t.Fatal("nil history should not error")
	}

	// Empty history
	result, _ := manager.ProcessHistory(context.Background(), []*ai.Message{})
	if len(result) != 0 {
		t.Fatal("empty history should return empty")
	}

	// Invalid window size
	badManager := SlidingWindowManager{WindowSize: 0}
	_, err = badManager.ProcessHistory(context.Background(), []*ai.Message{})
	if err == nil {
		t.Fatal("zero window size should error")
	}

	badManager = SlidingWindowManager{WindowSize: -1}
	_, err = badManager.ProcessHistory(context.Background(), []*ai.Message{})
	if err == nil {
		t.Fatal("negative window size should error")
	}
}

func TestAgentAsTool(t *testing.T) {
	type dummyInput struct {
		Query string `json:"query"`
	}

	type dummyOutput struct {
		Result string `json:"result"`
	}

	var (
		ctx = context.Background()
		g   = genkit.Init(ctx)

		cfg = AgentConfig{
			ID:          "021",
			Name:        "e21",
			Description: "ai",
			Instructions: []string{
				"test",
			},
		}
	)

	agent, err := NewAgent(cfg)
	if err != nil {
		t.Fatal(err)
	}
	genkit.RegisterAction(g, agent)

	t.Run("cfg_builders", func(t *testing.T) {
		cfg := &AgentAsToolConfig{}
		cfg.WithInputType(dummyInput{})
		cfg.WithOutputType(dummyOutput{})

		if cfg.InputSchema == nil {
			t.Fatal("expected InputSchema to be set")
		}
		if cfg.OutputSchema == nil {
			t.Fatal("expected OutputSchema to be set")
		}
	})

	t.Run("default", func(t *testing.T) {
		tool := agent.AsTool(nil)

		if tool.Name() != "e21" {
			t.Fatalf("invalid tool name: %s", tool.Name())
		}

		if tool.Definition().Description != "ai" {
			t.Fatalf("invalid tool description: %s", tool.Definition().Description)
		}

		expectedSchema := core.InferSchemaMap(defaultInputSchema)
		if !reflect.DeepEqual(tool.Definition().InputSchema, expectedSchema) {
			t.Fatal("invalid tool input schema")
		}
	})

	t.Run("custom", func(t *testing.T) {
		customCfg := &AgentAsToolConfig{
			Name:                "custom_tool",
			Description:         "custom description",
			EnableHistoryMemory: true,
		}
		customCfg.WithInputType(dummyInput{})
		customCfg.WithOutputType(dummyOutput{})

		tool := agent.AsTool(customCfg)

		if tool.Name() != "custom_tool" {
			t.Fatalf("invalid custom tool name: %s", tool.Name())
		}

		if tool.Definition().Description != "custom description" {
			t.Fatalf("invalid custom tool description: %s", tool.Definition().Description)
		}

		expectedInSchema := core.InferSchemaMap(dummyInput{})
		if !reflect.DeepEqual(tool.Definition().InputSchema, expectedInSchema) {
			t.Fatal("invalid custom tool input schema")
		}
	})
}

func TestDefaultHandlerToolInput(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		rawIn := map[string]any{"input": "hello world"}

		in, err := defaultHandlerToolInput(nil, rawIn)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(in.Messages) == 0 || in.Messages[0].Content[0].Text != "hello world" {
			t.Fatal("failed to map string input to message text")
		}
	})

	t.Run("dynamic_input", func(t *testing.T) {
		rawIn := map[string]any{"clave": "valor"}
		in, err := defaultHandlerToolInput(nil, rawIn)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expectedJSON := `{"clave":"valor"}`
		if len(in.Messages) == 0 || in.Messages[0].Content[0].Text != expectedJSON {
			t.Fatalf("failed to map dynamic input to json string, got: %s", in.Messages[0].Content[0].Text)
		}
	})

	t.Run("marshal_error", func(t *testing.T) {
		rawIn := map[string]any{"input": make(chan int)}

		_, err := defaultHandlerToolInput(nil, rawIn)
		if err == nil {
			t.Fatal("expected error when marshaling invalid type, got nil")
		}
	})

	t.Run("error", func(t *testing.T) {
		rawIn := make(chan int)
		_, err := defaultHandlerToolInput(nil, rawIn)
		if err == nil {
			t.Fatal("expected error when marshaling invalid type, got nil")
		}
	})
}

func TestDefaultHandlerModelResponse(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		mr := &ai.ModelResponse{
			Message: &ai.Message{
				Role: ai.RoleModel,
				Content: []*ai.Part{
					ai.NewTextPart("test21"),
					ai.NewMediaPart("test", "test"),
				},
				Metadata: map[string]any{"tokens": 10},
			},
		}

		toolResp, err := defaultHandlerModelResponse(nil, mr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if toolResp.Output != "test21" {
			t.Fatalf("expected output 'test21', got '%v'", toolResp.Output)
		}

		if toolResp.Metadata["tokens"] != 10 {
			t.Fatalf("metadata not mapped correctly")
		}
	})

	t.Run("nil", func(t *testing.T) {
		mr := &ai.ModelResponse{
			Message: nil,
		}

		toolResp, err := defaultHandlerModelResponse(nil, mr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if toolResp.Output != nil {
			t.Fatalf("expected nil output, got '%v'", toolResp.Output)
		}
		if toolResp.Metadata != nil {
			t.Fatalf("expected nil metadata")
		}
	})
}

func TestStringContextValues(t *testing.T) {
	tests := []struct {
		name    string
		key     contextKey
		setter  func(context.Context, string) context.Context
		getter  func(context.Context) (string, bool)
		testVal string
	}{
		{"GatewayID", gatewayIDKey, ContextWithGatewayID, GatewayIDFromContext, "test"},
		{"SessionID", sessionIDKey, ContextWithSessionID, SessionIDFromContext, "212121"},
		{"SenderID", senderIDKey, ContextWithSenderID, SenderIDFromContext, "e21"},
		{"AgentID", agentIDKey, ContextWithAgentID, AgentIDFromContext, "agent01"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Key does not exist (Nil)
			if v, ok := tt.getter(ctx); ok || v != "" {
				t.Errorf("Missing value failure: expected ('', false), got ('%s', %t)", v, ok)
			}

			// Key exists and is valid
			ctxValid := tt.setter(ctx, tt.testVal)
			if v, ok := tt.getter(ctxValid); !ok || v != tt.testVal {
				t.Errorf("Valid value failure: expected ('%s', true), got ('%s', %t)", tt.testVal, v, ok)
			}

			//  Key exists but has an invalid type
			ctxInvalid := context.WithValue(ctx, tt.key, 12345)
			if v, ok := tt.getter(ctxInvalid); ok || v != "" {
				t.Errorf("Invalid type failure: expected ('', false), got ('%s', %t)", v, ok)
			}
		})
	}
}

func TestBoolContextValues(t *testing.T) {
	tests := []struct {
		name    string
		key     contextKey
		setter  func(context.Context, bool) context.Context
		getter  func(context.Context) (bool, bool)
		testVal bool
	}{
		{"Stateless", statelessKey, ContextWithStateless, StatelessFromContext, true},
		{"AgentAsTool", agentAsToolKey, ContextWithAgentAsTool, AgentAsToolFromContext, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Key does not exist (Nil)
			if v, ok := tt.getter(ctx); ok || v != false {
				t.Errorf("Missing value failure: expected (false, false), got (%t, %t)", v, ok)
			}

			//  Key exists and is valid
			ctxValid := tt.setter(ctx, tt.testVal)
			if v, ok := tt.getter(ctxValid); !ok || v != tt.testVal {
				t.Errorf("Valid value failure: expected (%t, true), got (%t, %t)", tt.testVal, v, ok)
			}

			// Key exists but has an invalid type (Type assertion fails)
			ctxInvalid := context.WithValue(ctx, tt.key, "i_am_a_string")
			if v, ok := tt.getter(ctxInvalid); ok || v != false {
				t.Errorf("Invalid type failure: expected (false, false), got (%t, %t)", v, ok)
			}
		})
	}
}

func TestSetGenContext(t *testing.T) {
	tests := []struct {
		name      string
		cfg       *AgentConfig
		input     *Input
		wantGtw   string
		wantSess  string
		wantSend  string
		wantAgent string
	}{
		{
			name: "ok",
			cfg: &AgentConfig{
				ID: "agent21",
			},
			input: &Input{
				GatewayID: "telegram",
				SessionID: "505",
				SenderID:  "021",
			},
			wantGtw:   "telegram",
			wantSess:  "505",
			wantSend:  "021",
			wantAgent: "agent21",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseCtx := context.Background()
			gotCtx := tt.cfg.setGenContext(baseCtx, tt.input)

			// GatewayID
			if got := gotCtx.Value(gatewayIDKey); got != tt.wantGtw {
				t.Errorf("setGenContext() GatewayID = %v, expected %v", got, tt.wantGtw)
			}

			// SessionID
			if got := gotCtx.Value(sessionIDKey); got != tt.wantSess {
				t.Errorf("setGenContext() SessionID = %v, expected %v", got, tt.wantSess)
			}

			// SenderID
			if got := gotCtx.Value(senderIDKey); got != tt.wantSend {
				t.Errorf("setGenContext() SenderID = %v, expected %v", got, tt.wantSend)
			}

			// AgentID
			if got := gotCtx.Value(agentIDKey); got != tt.wantAgent {
				t.Errorf("setGenContext() AgentID = %v, expected %v", got, tt.wantAgent)
			}
		})
	}
}

// MOCKS
type mockMemory struct {
	agentID string
	msgs    []*ai.Message
	docs    []*ai.Document
}

// knowledge
func (k *mockMemory) RetrieveKnowledge(_ context.Context, agentID string, query string, _ int) ([]*ai.Document, error) {
	if agentID != k.agentID {
		return nil, errors.New("invalid agent id")
	} else if query == "knowledge_err" {
		return nil, errors.New("knowledge err")
	}

	return k.docs, nil
}

func (k *mockMemory) ReassignKnowledge(_ context.Context, _ string, _ string, _ string) error {
	return nil
}

func (k *mockMemory) IndexKnowledge(_ context.Context, _ string, _ string, _ []*ai.Document) error {
	return nil
}

func (k *mockMemory) DeleteKnowledge(_ context.Context, _ string, _ string) error {
	return nil
}

func (k *mockMemory) AsTool(agentID string, _ int) ai.Tool {
	if agentID != k.agentID {
		panic("invalid agent id")
	}
	return nil
}

func (k *mockMemory) Register(_ api.Registry) {}

// history
func (h *mockMemory) StoreHistory(_ context.Context, agentID string, _ string, sessionID string, _ []*ai.Message) error {
	if sessionID == "store_err" {
		return errors.New("store err")
	} else if agentID != h.agentID {
		return errors.New("invalid agent id")
	}
	return nil
}

func (h *mockMemory) RetrieveHistory(_ context.Context, agentID string, _ string, sessionID string) ([]*ai.Message, error) {
	if sessionID == "retrieve_err" {
		return nil, errors.New("retrieve err")
	} else if agentID != h.agentID {
		return nil, errors.New("invalid agent id")
	}
	return h.msgs, nil
}

func (h *mockMemory) DeleteHistory(_ context.Context, _ string, _ string, _ string) error {
	return nil
}

func (h *mockMemory) DeleteAgentHistories(_ context.Context, _ string) error {
	return nil
}

func (h *mockMemory) DeleteGatewayHistories(_ context.Context, _ string, _ string) error {
	return nil
}

// manager
func (h *mockMemory) ProcessHistory(ctx context.Context, history []*ai.Message) ([]*ai.Message, error) {
	v := ctx.Value("manager_err")
	if v != nil {
		return nil, errors.New("manager err")
	}
	return history, nil
}

// queryer
func (h *mockMemory) GenerateQuery(ctx context.Context, _ *Input) (string, error) {
	v := ctx.Value("queryer_err")
	if v != nil {
		return "", errors.New("queryer err")
	}

	v = ctx.Value("knowledge_err")
	if v != nil {
		return "knowledge_err", nil
	}

	return "test", nil
}
