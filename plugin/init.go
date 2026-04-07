package plugin

import (
	"github.com/mwantia/forge-plugin-openai/internal/openai"
	"github.com/mwantia/forge-sdk/pkg/plugins"
)

func init() {
	plugins.Register(openai.PluginName, openai.PluginDescription, openai.NewOpenAIDriver)
}
