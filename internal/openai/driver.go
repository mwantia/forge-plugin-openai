package openai

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/mapstructure"
	"github.com/mwantia/forge-sdk/pkg/plugins"
	"github.com/mwantia/forge-sdk/pkg/retry"
	openailib "github.com/sashabaranov/go-openai"
)

const (
	PluginName        = "openai"
	PluginAuthor      = "forge"
	PluginVersion     = "0.1.0"
	PluginDescription = "OpenAI-compatible provider for any API that follows the OpenAI specification"
)

// OpenAIDriver implements plugins.Driver for OpenAI-compatible endpoints.
type OpenAIDriver struct {
	plugins.UnimplementedDriver

	log    hclog.Logger
	config *OpenAIConfig
	client *openailib.Client
}

// NewOpenAIDriver creates a new driver that supports the OpenAI-compatible provider plugin type.
func NewOpenAIDriver(log hclog.Logger) plugins.Driver {
	return &OpenAIDriver{
		log: log.Named(PluginName),
		config: &OpenAIConfig{
			Address: "https://api.openai.com",
			Timeout: "60s",
			Models:  make(map[string]*OpenAIModelTemplate),
		},
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

func (d *OpenAIDriver) ProbePlugin(ctx context.Context) (bool, error) {
	if d.client == nil {
		return false, nil
	}

	err := retry.Do(ctx, retry.DefaultConfig, func(ctx context.Context) error {
		_, callErr := d.client.ListModels(ctx)
		return classifyError(callErr)
	})
	if err != nil {
		d.log.Debug("OpenAI probe failed", "error", err)
		return false, nil
	}
	return true, nil
}

func (d *OpenAIDriver) GetCapabilities(ctx context.Context) (*plugins.DriverCapabilities, error) {
	return &plugins.DriverCapabilities{
		Types: []string{plugins.PluginTypeProvider},
		Provider: &plugins.ProviderCapabilities{
			SupportsStreaming: true,
			SupportsVision:    false,
		},
	}, nil
}

func (d *OpenAIDriver) ConfigDriver(ctx context.Context, config plugins.PluginConfig) error {
	if err := mapstructure.Decode(config.ConfigMap, d.config); err != nil {
		return fmt.Errorf("failed to decode config: %w", err)
	}

	timeout := parseDuration(d.config.Timeout, 60*time.Second)

	cfg := openailib.DefaultConfig(d.config.Token)
	if d.config.Address != "" {
		cfg.BaseURL = d.config.Address + "/v1"
	}
	// Use context cancellation for streaming; timeout only on unary calls.
	cfg.HTTPClient = &http.Client{Timeout: timeout}
	d.client = openailib.NewClientWithConfig(cfg)

	d.log.Info("Configured OpenAI driver",
		"address", d.config.Address,
		"timeout", timeout,
		"auth", d.config.Token != "",
	)

	for name, m := range d.config.Models {
		d.log.Debug("Registered model alias",
			"alias", name,
			"base_model", m.BaseModel,
			"reasoning", m.Reasoning,
			"system", fmt.Sprintf("len:%d", len(m.System)),
		)
	}

	return nil
}

func (d *OpenAIDriver) OpenDriver(ctx context.Context) error {
	return nil
}

func (d *OpenAIDriver) CloseDriver(ctx context.Context) error {
	return nil
}

func (d *OpenAIDriver) GetProviderPlugin(ctx context.Context) (plugins.ProviderPlugin, error) {
	return &OpenAIProviderPlugin{driver: d}, nil
}

func parseDuration(s string, fallback time.Duration) time.Duration {
	if s == "" {
		return fallback
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}

var _ plugins.Driver = (*OpenAIDriver)(nil)
