package models

type Environment struct {
	AgentType      string `env:"AGENT_TYPE" required:"true"`
	DatabaseDriver string `env:"DATABASE_DRIVER" required:"true"`
	DatabaseURL    string `env:"DATABASE_URL" required:"true"`
	GroupID        string `env:"GROUP_ID" required:"true"`
	Region         string `env:"REGION" required:"true"`
	Topic          string `env:"TOPIC" required:"true"`
	BusBroker      string `env:"BUS_BROKER" required:"true"`
	OpenAIAPIKey   string `env:"OPENAI_API_KEY" required:"true"`
}
