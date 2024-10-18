package config

type AppConfig struct {
	Pause            bool   `env:"PAUSE" envDefault:"false"`
	PhoneNumber      string `env:"PHONE_NUMBER,required"`
	BotToken         string `env:"BOT_TOKEN,required"`
	MetaApiAccountID string `env:"META_API_ACCOUNT_ID,required"`
	MetaApiToken     string `env:"META_API_TOKEN,required"`
	OpenAiToken      string `env:"OPENAI_TOKEN,required"`
	MetaApiEndpoint  string `env:"META_API_ENDPOINT,required"`
}
