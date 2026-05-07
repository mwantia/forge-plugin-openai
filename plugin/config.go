package plugin

// OpenAIConfig holds the configuration for the OpenAI-compatible driver.
type OpenAIConfig struct {
	// Address is the base URL of the provider (without /v1 suffix).
	// Defaults to https://api.openai.com.
	Address string `mapstructure:"address"`
	// Token is the API key / bearer token for the provider.
	Token string `mapstructure:"token"`
	Seed  *int   `mapstructure:"seed"`

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
