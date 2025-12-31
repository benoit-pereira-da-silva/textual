package openaitextual

import "os"

type Model string

const (
	MODEL_GPT_4_1 Model = "gpt-4.1"
	MODEL_GPT_5_2 Model = "gpt-5.2"

	DEFAULT_API_URL = "httpS://api.openai.com/v1"
)

type ClientConfig struct {
	apiKey  string
	baseURL string
	model   Model
}

func NewClientConfig(apiKey string, baseURL string, model Model) ClientConfig {
	config := ClientConfig{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
	}
	if config.apiKey == "" {
		config.apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if config.baseURL == "" {
		config.baseURL = os.Getenv("OPENAI_API_URL")
		if config.baseURL == "" {
			config.baseURL = DEFAULT_API_URL
		}
	}
	if config.model == "" {
		config.model = Model(os.Getenv("OPENAI_MODEL"))
		if config.model == "" {
			config.model = MODEL_GPT_4_1
		}
	}
	return config
}
