package agens

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

type testMemory struct {
	agentID string
	msgs    []*ai.Message
}

func (k *testMemory) AsTool(agentID string, limit int) ai.Tool {
	if agentID != k.agentID {
		panic("invalid agent id")
	}
	return nil
}

func (k *testMemory) IndexKnowledge(_ context.Context, agentID string, label string, _ []*ai.Document) error {
	if label == "err" {
		return errors.New("test err")
	} else if agentID != k.agentID {
		return errors.New("invalid agent id")
	}
	return nil
}

func (k *testMemory) DeleteKnowledge(_ context.Context, agentID string, label string) error {
	if label == "err" {
		return errors.New("test err")
	} else if agentID != k.agentID {
		return errors.New("invalid agent id")
	}
	return nil
}

func (h *testMemory) RetrieveHistory(_ context.Context, agentID string, sessionID string) ([]*ai.Message, error) {
	if sessionID == "test_retrieveErr" {
		return nil, errors.New("retrieve err")
	} else if agentID != h.agentID {
		return nil, errors.New("invalid agent id")
	}
	return h.msgs, nil
}

func (h *testMemory) StoreHistory(_ context.Context, agentID string, sessionID string, _ []*ai.Message, _ int) error {
	if sessionID == "err" {
		return errors.New("test err")
	} else if agentID != h.agentID {
		return errors.New("invalid agent id")
	}
	return nil
}

func (h *testMemory) DeleteHistory(_ context.Context, agentID string, sessionID string) error {
	if sessionID == "err" {
		return errors.New("test err")
	} else if agentID != h.agentID {
		return errors.New("invalid agent id")
	}
	return nil
}

func TestNewAgent(t *testing.T) {
	var (
		ctx = context.Background()

		g = genkit.Init(ctx)

		cfg = AgentConfig{
			ID:          "021",
			Name:        "e21",
			Description: "ai",
			Instructions: []string{
				"test",
			},
		}
	)

	agent, err := NewAgent(g, cfg)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("id", func(tt *testing.T) {
		if agent.ID() != "021" {
			tt.Fatal("ID must be 021")
		}
	})

	t.Run("indexKnowledge", func(tt *testing.T) {
		err := agent.IndexKnowledge(context.Background(), "label", nil)
		if !errors.Is(err, ErrKnowledgeMemoryNotConfigured) {
			tt.Fatal(err)
		}
	})

	t.Run("deleteKnowledge", func(tt *testing.T) {
		err := agent.DeleteKnowledge(context.Background(), "label")
		if !errors.Is(err, ErrKnowledgeMemoryNotConfigured) {
			tt.Fatal(err)
		}
	})

	t.Run("systemPrompt", func(tt *testing.T) {
		in := &Input{}

		agentSystem, err := agent.cfg.GenerateSystemPrompt(context.Background(), in)
		if err != nil {
			tt.Fatal(err)
		}

		system, err := DefaultSystemPromptFn(ctx, &cfg, in)
		if err != nil {
			tt.Fatal(err)
		}

		if agentSystem != system {
			tt.Fatal("invalid system prompt")
		}
	})
}

func TestErrNewAgent(t *testing.T) {
	t.Run("emptyID", func(t *testing.T) {
		_, err := NewAgent(nil, AgentConfig{})
		if !errors.Is(err, ErrEmptyAgentID) {
			t.Fatal(err)
		}
	})

	t.Run("emptyName", func(t *testing.T) {
		_, err := NewAgent(nil, AgentConfig{ID: "021"})
		if !errors.Is(err, ErrEmptyAgentName) {
			t.Fatal(err)
		}
	})

	t.Run("emptyDescription", func(t *testing.T) {
		_, err := NewAgent(nil, AgentConfig{ID: "021", Name: "e21"})
		if !errors.Is(err, ErrEmptyAgentDescription) {
			t.Fatal(err)
		}
	})

	t.Run("emptyInstructions", func(t *testing.T) {
		_, err := NewAgent(nil, AgentConfig{ID: "021", Name: "e21", Description: "ai"})
		if !errors.Is(err, ErrEmptyAgentInstructions) {
			t.Fatal(err)
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

	t.Run("indexKnowledge", func(tt *testing.T) {
		err := agent.IndexKnowledge(context.Background(), "label", nil)
		if !errors.Is(err, ErrAgentNotInitialized) {
			tt.Fatal(err)
		}
	})

	t.Run("deleteKnowledge", func(tt *testing.T) {
		err := agent.DeleteKnowledge(context.Background(), "label")
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

		errSortTest = errors.New("sort err")
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

		agent, err := NewAgent(g, cfg)
		if err != nil {
			t.Fatal(err)
		}

		_, err = agent.Generate(ctx, in)
		if !errors.Is(err, errSystemTest) {
			t.Fatal(err)
		}
	})

	t.Run("errHistory", func(t *testing.T) {
		var (
			cfg = AgentConfig{
				ID:          "agent02",
				Name:        "e21",
				Description: "ai",
				Instructions: []string{
					"test",
				},
				HistoryMemory: &testMemory{
					agentID: "agent02",
				},
			}

			in = &Input{
				Trigger:  "test",
				Source:   "retrieveErr",
				Messages: []*ai.Message{},
			}
		)

		agent, err := NewAgent(g, cfg)
		if err != nil {
			t.Fatal(err)
		}

		_, err = agent.Generate(ctx, in)
		if err.Error() != "retrieve err" {
			t.Fatal(err)
		}
	})

	t.Run("errSortMessages", func(t *testing.T) {
		var (
			cfg = AgentConfig{
				ID:          "agent03",
				Name:        "e21",
				Description: "ai",
				Instructions: []string{
					"test",
				},
				SortMessagesFn: func(_ []*ai.Message) ([]*ai.Message, error) {
					return []*ai.Message{}, errSortTest
				},
			}

			in = &Input{
				Messages: []*ai.Message{},
			}
		)

		agent, err := NewAgent(g, cfg)
		if err != nil {
			t.Fatal(err)
		}

		_, err = agent.Generate(ctx, in)
		if !errors.Is(err, errSortTest) {
			t.Fatal(err)
		}
	})
}

func TestInput(t *testing.T) {
	t.Run("empty", func(tt *testing.T) {
		emptyIn := &Input{}

		// session id
		if emptyIn.SessionID() != "" {
			tt.Fatalf("session id must be empty, SessionID(%s)", emptyIn.SessionID())
		}

		// empty getOutputOption
		if len(emptyIn.getOutputOption()) != 0 {
			tt.Fatal("output options must be empty")
		}

		// empty instructions
		if emptyIn.OutputInstructions != nil {
			tt.Fatal("output instructions must be nil")
		}

		emptyIn.WithOutputInstructions("")

		if len(emptyIn.getOutputOption()) != 1 {
			tt.Fatal("output options cannot be empty")
		}

		if emptyIn.OutputInstructions == nil {
			tt.Fatal("output instructions cannot be empty")
		}

		if *emptyIn.OutputInstructions != "" {
			tt.Fatal("output instructions must be empty")
		}
	})

	// input
	in := &Input{
		Trigger: "test",
		Source:  "21",
	}

	// session id
	if in.SessionID() != "test_21" {
		t.Fatalf("session id must be test_21, SessionID(%s)", in.SessionID())
	}

	// empty getOutputOption
	if len(in.getOutputOption()) != 0 {
		t.Fatal("output options must be empty")
	}

	// instructions
	if in.OutputInstructions != nil {
		t.Fatal("output instructions must be nil")
	}

	in.WithOutputInstructions("test")

	if len(in.getOutputOption()) != 1 {
		t.Fatal("output options cannot be empty")
	}

	if in.OutputInstructions == nil {
		t.Fatal("output instructions cannot be empty")
	}

	if *in.OutputInstructions != "test" {
		t.Fatal("output instructions must be empty")
	}

	// output schema
	if in.OutputSchema != nil {
		t.Fatal("output schema must be nil")
	}

	outputT := struct{ Name string }{Name: "test"}
	in.WithOutputType(outputT)

	if len(in.getOutputOption()) != 2 {
		t.Fatal("len output options must be 2")
	}

	if in.OutputSchema == nil {
		t.Fatal("output schema cannot be empty")
	}
}

func TestConfig(t *testing.T) {
	t.Run("generateOpts", func(t *testing.T) {
		cfg := AgentConfig{
			GenerateOptions: []ai.GenerateOption{
				ai.WithConfig(struct{}{}),
			},
		}

		if len(cfg.getGenerateOptions()) == 0 {
			t.Fatal("cannot be empty")
		}

		// model
		cfg.Model = ai.NewModelRef("test", nil)
		if len(cfg.getGenerateOptions()) != 2 {
			t.Fatal("invalid opts")
		}

		// knowledge
		cfg.KnowledgeMemory = &testMemory{}

		if len(cfg.getGenerateOptions()) != 3 {
			t.Fatal("invalid opts")
		}
	})

	t.Run("customSystemPrompt", func(tt *testing.T) {
		cfg := AgentConfig{
			SystemPromptFn: func(_ context.Context, _ *AgentConfig, _ *Input) (string, error) {
				return "test", nil
			},
		}

		cfgSystem, err := cfg.GenerateSystemPrompt(context.Background(), &Input{})
		if err != nil {
			tt.Fatal(err)
		}

		if cfgSystem != "test" {
			tt.Fatal("invalid system prompt")
		}
	})
}

func TestSortMessagesFn(t *testing.T) {
	var (
		systemMsg = ai.NewSystemTextMessage("test")

		userMsg  = ai.NewUserTextMessage("test")
		userMsg2 = ai.NewUserTextMessage("test 2")
		userMsg3 = ai.NewUserTextMessage("test 3")

		modelMsg  = ai.NewModelTextMessage("test")
		modelMsg2 = ai.NewModelTextMessage("test 2")
		modelMsg3 = ai.NewModelTextMessage("test 3")

		toolMsg  = ai.NewMessage(ai.RoleTool, nil, nil)
		toolMsg2 = ai.NewMessage(ai.RoleTool, nil, nil)

		noValidMsgs = []*ai.Message{
			systemMsg,
			modelMsg,
		}

		inMessages = []*ai.Message{
			toolMsg,
			userMsg,
			systemMsg,
			modelMsg,
			userMsg2,
			modelMsg2,
			toolMsg2,
			modelMsg3,
			userMsg3,
		}

		outMessages = []*ai.Message{
			userMsg,
			modelMsg,
			userMsg2,
			modelMsg2,
			toolMsg2,
			modelMsg3,
			userMsg3,
		}
	)

	t.Run("empty", func(t *testing.T) {
		msgs, err := DefaultSortMessagesFn(nil)
		if err != nil {
			t.Fatal(err)
		}

		if msgs != nil {
			t.Fatal("messages must be nil")
		}
	})

	t.Run("errNoValidMessages", func(t *testing.T) {
		_, err := DefaultSortMessagesFn(noValidMsgs)
		if !errors.Is(err, ErrNoValidMessages) {
			t.Fatal(err)
		}
	})

	t.Run("OK", func(t *testing.T) {
		result, err := DefaultSortMessagesFn(inMessages)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(result, outMessages) {
			t.Fatal("invalid sort")
		}
	})

	t.Run("flowStep", func(t *testing.T) {
		var (
			ctx = context.Background()

			g = genkit.Init(ctx)

			cfg = AgentConfig{
				ID:          "021",
				Name:        "e21",
				Description: "ai",
				Instructions: []string{
					"test",
				},
			}

			fn = func(ctx context.Context, msgs []*ai.Message) ([]*ai.Message, error) {
				return cfg.sortMessages(ctx, msgs)
			}

			flow = genkit.DefineFlow(g, "testSortMessagesFn", fn)

			inMessages = []*ai.Message{
				userMsg,
				systemMsg,
				modelMsg,
				userMsg2,
				modelMsg2,
				userMsg3,
			}

			outMessages = []*ai.Message{
				userMsg,
				modelMsg,
				userMsg2,
				modelMsg2,
				userMsg3,
			}
		)

		result, err := flow.Run(ctx, inMessages)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(result, outMessages) {
			t.Fatal("invalid sort")
		}

		// custom
		cfg.SortMessagesFn = func(msgs []*ai.Message) ([]*ai.Message, error) {
			return []*ai.Message{modelMsg, userMsg}, nil
		}

		result, err = flow.Run(ctx, inMessages)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(result, []*ai.Message{modelMsg, userMsg}) {
			t.Fatal("invalid sort")
		}

	})
}

func TestKnowledgeMemory(t *testing.T) {
	var (
		ctx = context.Background()

		g = genkit.Init(ctx)

		cfg = AgentConfig{
			ID:          "021",
			Name:        "e21",
			Description: "ai",
			Instructions: []string{
				"test",
			},
			KnowledgeMemory: &testMemory{
				agentID: "021",
			},
		}
	)

	agent, err := NewAgent(g, cfg)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("index", func(t *testing.T) {
		err := agent.IndexKnowledge(ctx, "test", nil)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("errIndex", func(t *testing.T) {
		err := agent.IndexKnowledge(ctx, "err", nil)
		if err == nil {
			t.Fatal(err)
		}
	})

	t.Run("delete", func(t *testing.T) {
		err := agent.DeleteKnowledge(ctx, "test")
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("errDelete", func(t *testing.T) {
		err := agent.DeleteKnowledge(ctx, "err")
		if err == nil {
			t.Fatal(err)
		}
	})
}

func TestHistoryMemory(t *testing.T) {
	var (
		ctx = context.Background()

		g = genkit.Init(ctx)

		cfg = AgentConfig{
			ID:          "021",
			Name:        "e21",
			Description: "ai",
			Instructions: []string{
				"test",
			},
		}

		msgs = []*ai.Message{
			ai.NewUserTextMessage("test"),
		}

		memory = &testMemory{
			agentID: "021",
			msgs:    msgs,
		}

		retrieveFn = func(ctx context.Context, sessionID string) ([]*ai.Message, error) {
			return cfg.retrieveHistory(ctx, sessionID)
		}

		storeFn = func(ctx context.Context, in struct {
			sessionID string
			messages  []*ai.Message
		}) (struct{}, error) {
			return struct{}{}, cfg.storeHistory(ctx, in.sessionID, in.messages)
		}

		retrieveFlow = genkit.DefineFlow(g, "testHistoryMemoryRetrieve", retrieveFn)
		storeFlow    = genkit.DefineFlow(g, "testHistoryMemoryStore", storeFn)
	)

	t.Run("emptyRetrieve", func(t *testing.T) {
		result, err := retrieveFlow.Run(ctx, "test")
		if err != nil {
			t.Fatal(err)
		}

		if len(result) != 0 {
			t.Fatal("history must be nil")
		}
	})

	t.Run("emptyStore", func(t *testing.T) {
		in := struct {
			sessionID string
			messages  []*ai.Message
		}{
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
		result, err := retrieveFlow.Run(ctx, "test")
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(result, msgs) {
			t.Fatal("invalid messages")
		}
	})

	t.Run("store", func(t *testing.T) {
		in := struct {
			sessionID string
			messages  []*ai.Message
		}{
			sessionID: "test",
			messages:  []*ai.Message{},
		}

		_, err := storeFlow.Run(ctx, in)
		if err != nil {
			t.Fatal(err)
		}
	})
}
