package tgbot

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gonzxlezs/agens"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
)

const DefaultSubPath = "/tgbot/"

var ErrGatewayIDEmpty = errors.New("gateway id cannot be empty")

var _ agens.Gateway = &TgGateway{}
var _ agens.GatewayRef = &TgGateway{}

type TgGatewayConfig struct {
	ID                      string
	Name                    string
	Token                   string
	DisableDefaultPipelines bool

	// SubPath is the base routing path for the webhook.
	SubPath string

	// SecretPath is a unique identifier or token appended to the URL
	// to prevent unauthorized update injections. Defaults to the bot token.
	SecretPath string

	// WebhookSecret is a random string used to verify that the webhook
	// request comes from the official source.
	WebhookSecret string

	BotOpts        *gotgbot.BotOpts
	DispatcherOpts *ext.DispatcherOpts
	UpdaterOpts    *ext.UpdaterOpts
}

type TgGateway struct {
	Bot        *gotgbot.Bot
	Dispatcher *ext.Dispatcher
	Updater    *ext.Updater

	SubPath       string
	SecretPath    string
	WebhookSecret string

	id      string
	name    string
	agent   *agens.Agent
	batcher agens.MessageBatcher
}

func NewTgGateway(cfg TgGatewayConfig) (*TgGateway, error) {
	// id
	cfg.ID = strings.TrimSpace(cfg.ID)
	if cfg.ID == "" {
		return nil, ErrGatewayIDEmpty
	}

	// bot
	bot, err := gotgbot.NewBot(cfg.Token, cfg.BotOpts)
	if err != nil {
		return nil, err
	}

	// name
	if cfg.Name == "" {
		cfg.Name = fmt.Sprintf("tg_%s", bot.User.Username)
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
	dispatcher := ext.NewDispatcher(cfg.DispatcherOpts)

	gateway := &TgGateway{
		id:            cfg.ID,
		name:          cfg.Name,
		Bot:           bot,
		Dispatcher:    dispatcher,
		Updater:       ext.NewUpdater(dispatcher, cfg.UpdaterOpts),
		SubPath:       cfg.SubPath,
		SecretPath:    cfg.SecretPath,
		WebhookSecret: cfg.WebhookSecret,
	}

	// pipelines
	if cfg.DisableDefaultPipelines {
		return gateway, nil
	}

	// text
	err = gateway.AddPipeline(message.Text, NewTextProcessor(gateway))
	if err != nil {
		return nil, err
	}

	return gateway, nil
}

func (g *TgGateway) ID() string { return g.id }

func (g *TgGateway) Name() string { return g.name }

func (g *TgGateway) Agent() *agens.Agent { return g.agent }

func (g *TgGateway) RegisterAgent(agent *agens.Agent) error {
	g.agent = agent
	return nil
}

func (g *TgGateway) MessageBatcher() agens.MessageBatcher { return g.batcher }

func (g *TgGateway) WithMessageBatcher(batcher agens.MessageBatcher) error {
	g.batcher = batcher
	return nil
}
