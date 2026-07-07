package wabot

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/gonzxlezs/agens"
	wapi "github.com/wapikit/wapi.go/pkg/client"
	"github.com/wapikit/wapi.go/pkg/events"
)

const DefaultSubPath = "/wabot/"

var ErrGatewayIDEmpty = errors.New("gateway id cannot be empty")

var _ agens.Gateway = &WAGateway{}
var _ agens.GatewayRef = &WAGateway{}

type WAGatewayConfig struct {
	ID   string
	Name string

	ClientConfig *wapi.ClientConfig
	Logger       *slog.Logger

	DisableDefaultPipelines bool

	// SubPath is the base routing path for the webhook.
	SubPath string

	// SecretPath is a unique identifier or token appended to the URL
	// to prevent unauthorized update injections. Defaults to the business id.
	SecretPath string
}

type WAGateway struct {
	Client *wapi.Client
	Logger *slog.Logger

	SubPath    string
	SecretPath string

	id      string
	name    string
	agent   *agens.Agent
	batcher agens.MessageBatcher
}

func NewWAGateway(cfg WAGatewayConfig) (*WAGateway, error) {
	// ID
	cfg.ID = strings.TrimSpace(cfg.ID)
	if cfg.ID == "" {
		return nil, ErrGatewayIDEmpty
	}

	// client
	client := wapi.New(cfg.ClientConfig)

	// logger
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}

	// name
	if cfg.Name == "" {
		cfg.Name = fmt.Sprintf("wa_%s", client.Business.BusinessAccountId)
	}

	// sub path
	if cfg.SubPath == "" {
		cfg.SubPath = DefaultSubPath
	}

	// secret path
	if cfg.SecretPath == "" {
		cfg.SecretPath = cfg.ID
	}

	// gateway
	gateway := &WAGateway{
		id:         cfg.ID,
		name:       cfg.Name,
		Client:     client,
		Logger:     cfg.Logger,
		SubPath:    cfg.SubPath,
		SecretPath: cfg.SecretPath,
	}

	if cfg.DisableDefaultPipelines {
		return gateway, nil
	}

	// text
	err := gateway.AddPipeline(events.TextMessageEventType, NewTextProcessor())
	if err != nil {
		return nil, err
	}

	return gateway, nil
}

func (g *WAGateway) ID() string { return g.id }

func (g *WAGateway) Name() string { return g.name }

func (g *WAGateway) RegisterAgent(agent *agens.Agent) error {
	g.agent = agent
	return nil
}

func (g *WAGateway) Agent() *agens.Agent { return g.agent }

func (g *WAGateway) WithMessageBatcher(batcher agens.MessageBatcher) error {
	g.batcher = batcher
	return nil
}

func (g *WAGateway) MessageBatcher() agens.MessageBatcher { return g.batcher }
