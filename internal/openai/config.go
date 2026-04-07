package openai

// OpenAIConfig holds the configuration for the OpenAI-compatible driver.
type OpenAIConfig struct {
	Address string `mapstructure:"address"`
	Token   string `mapstructure:"token"`
	Timeout string `mapstructure:"timeout"`
	Seed    *int   `mapstructure:"seed"`

	Models map[string]*OpenAIModelTemplate `mapstructure:"model"`
}

// OpenAIModelTemplate defines a named model alias within forge.
// The alias is resolved in-process: base_model is sent to the provider, system
// is prepended as a system message, and options are merged into every request.
type OpenAIModelTemplate struct {
	// BaseModel is the underlying model name forwarded to the provider (e.g. "gpt-4o").
	BaseModel string `mapstructure:"base_model"`

	// Reasoning controls whether thinking tokens are forwarded in ChatChunk.Thinking.
	// The provider must support an extended-thinking mode for this to have any effect.
	Reasoning bool `mapstructure:"reasoning"`

	// System is prepended as a system message when none is present in the history.
	System string `mapstructure:"system"`

	// Options overrides generation parameters for this alias.
	Options *OpenAIModelOptions `mapstructure:"options"`

	// CostPerInputToken is the cost in USD per prompt token.
	CostPerInputToken float64 `mapstructure:"cost_per_input_token"`

	// CostPerOutputToken is the cost in USD per completion token.
	CostPerOutputToken float64 `mapstructure:"cost_per_output_token"`
}

// OpenAIModelOptions maps to OpenAI-compatible generation parameters.
// All fields are pointers so unset values are omitted from the request.
type OpenAIModelOptions struct {
	Temperature *float64 `mapstructure:"temperature"`
	TopP        *float64 `mapstructure:"top_p"`
	TopK        *int     `mapstructure:"top_k"`
	MaxTokens   *int     `mapstructure:"max_tokens"`
}
