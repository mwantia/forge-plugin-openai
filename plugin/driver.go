package plugin

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/mapstructure"
	"github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/sashabaranov/go-openai"
)

const (
	PluginName        = "openai"
	PluginAuthor      = "forge"
	PluginVersion     = "1.0.0"
	PluginDescription = "OpenAI-compatible provider for any API that follows the OpenAI specification"
)

type OpenAIDriver struct {
	plugins.UnimplementedDriver

	log    hclog.Logger
	config OpenAIConfig

	client   *openai.Client // unary requests (probe, embed, list models); has timeout
	streamer *openai.Client // streaming chat; no timeout — context cancellation controls it
}

func NewOpenAIDriver(log hclog.Logger) plugins.Driver {
	return &OpenAIDriver{
		log:    log.Named(PluginName),
		config: NewDefaultConfig(),
	}
}

func (d *OpenAIDriver) GetPluginInfo() plugins.PluginInfo {
	return plugins.PluginInfo{
		Name:        PluginName,
		Author:      PluginAuthor,
		Version:     PluginVersion,
		Description: PluginDescription,
	}
}

func (d *OpenAIDriver) GetCapabilities(_ context.Context) (*plugins.DriverCapabilities, error) {
	return &plugins.DriverCapabilities{
		Types: []string{plugins.PluginTypeProvider},
		Provider: &plugins.ProviderCapabilities{
			SupportsStreaming: true,
			SupportsVision:    false,
		},
	}, nil
}

func (d *OpenAIDriver) ConfigDriver(_ context.Context, cfg plugins.PluginConfig) error {
	if err := mapstructure.Decode(cfg.ConfigMap, &d.config); err != nil {
		return fmt.Errorf("failed to decode config: %w", err)
	}

	timeout, err := time.ParseDuration(d.config.Http.Timeout)
	if err != nil {
		return fmt.Errorf("invalid timeout %q: %w", d.config.Http.Timeout, err)
	}

	baseURL := d.config.Address + "/v1"

	unaryCfg := openai.DefaultConfig(d.config.Token)
	unaryCfg.BaseURL = baseURL
	unaryCfg.HTTPClient = &http.Client{Timeout: timeout}
	d.client = openai.NewClientWithConfig(unaryCfg)

	streamCfg := openai.DefaultConfig(d.config.Token)
	streamCfg.BaseURL = baseURL
	streamCfg.HTTPClient = &http.Client{}
	d.streamer = openai.NewClientWithConfig(streamCfg)

	d.log.Info("Configured OpenAI driver", "address", d.config.Address, "timeout", timeout, "auth", d.config.Token != "")
	return nil
}

func (d *OpenAIDriver) ProbePlugin(ctx context.Context) (bool, error) {
	if d.client == nil {
		return false, fmt.Errorf("driver not configured")
	}

	_, err := d.client.ListModels(ctx)
	if err != nil {
		d.log.Warn("OpenAI probe failed", "address", d.config.Address, "error", err)
		return false, nil
	}
	return true, nil
}

func (d *OpenAIDriver) OpenDriver(_ context.Context) error  { return nil }
func (d *OpenAIDriver) CloseDriver(_ context.Context) error { return nil }

func (d *OpenAIDriver) GetProviderPlugin(_ context.Context) (plugins.ProviderPlugin, error) {
	return &OpenAIProviderPlugin{
		driver: d,
	}, nil
}

var _ plugins.Driver = (*OpenAIDriver)(nil)
