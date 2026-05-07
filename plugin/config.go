package plugin

// OpenAIConfig holds the configuration for the OpenAI-compatible driver.
type OpenAIConfig struct {
	// Address is the base URL of the provider.
	// For OpenAI-compatible endpoints omit the /v1 suffix — the driver appends it.
	// For Azure/Anthropic use the full endpoint URL as-is.
	// Defaults to https://api.openai.com.
	Address string `mapstructure:"address"`
	// Token is the API key / bearer token for the provider.
	Token string `mapstructure:"token"`
	// OrgID is the OpenAI organisation ID sent as the OpenAI-Organization header.
	OrgID string `mapstructure:"org_id"`
	// APIType selects the wire protocol: OPEN_AI (default), AZURE, AZURE_AD, ANTHROPIC.
	APIType string `mapstructure:"api_type"`
	// APIVersion is the API version string, required for AZURE, AZURE_AD, and ANTHROPIC.
	APIVersion string `mapstructure:"api_version"`

	Seed *int `mapstructure:"seed"`

	Http OpenAIHttpConfig `mapstructure:"http"`
}

type OpenAIHttpConfig struct {
	Timeout string `mapstructure:"timeout"`
}

//	config {
//	    address = "https://api.openai.com" # provider base URL (no /v1 suffix)
//	    token   = ""                       # API key; required for ollama.com
//	    seed    =                          # optional fixed seed
//
//	    http {
//	        timeout = "60s"
//	    }
//	}
func NewDefaultConfig() OpenAIConfig {
	return OpenAIConfig{
		Address: "https://api.openai.com",
		Token:   "",
		Http: OpenAIHttpConfig{
			Timeout: "60s",
		},
	}
}
