package openai

import (
	"github.com/mwantia/forge-plugin-openai/plugin"
	"github.com/mwantia/forge-sdk/pkg/plugins"
)

func init() {
	plugins.Register(plugin.PluginName, plugin.PluginDescription, plugin.NewOpenAIDriver)
}
