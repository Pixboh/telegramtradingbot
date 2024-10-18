package app

import (
	"github.com/caarlos0/env/v6"
	"tdlib/authmanager"
	"tdlib/config"
	"tdlib/redis_client"
	"tdlib/tgbot"
)

type App struct {
	TgBot     *tgbot.TgBot
	AppConfig *config.AppConfig
}

// single instance of the app creation
func NewApp() *App {
	app := App{}
	app.Setup()
	return &app
}
func (app *App) Setup() {
	//config := config.AppConfig{
	//	//PhoneNumber: "+221771307579",
	//	PhoneNumber: "+33658532534",
	//}
	config, errC := LoadConfig()
	if errC != nil {
		panic(errC)
	}
	app.AppConfig = config
	redisClient := redis_client.NewRedisClient()
	terminalAuth := authmanager.NewTerminalPrompt(*config)
	app.TgBot = tgbot.NewTgBot(*config, redisClient, terminalAuth)
}

func (app *App) IsPaused() bool {
	return app.AppConfig.Pause
}

func (app *App) Pause() {
	app.AppConfig.Pause = true
}
func (app *App) Resume() {
	app.AppConfig.Pause = false
}
func (app *App) Toggle() {
	app.AppConfig.Pause = !app.AppConfig.Pause
}

func (app *App) Run() {
	app.TgBot.Start()
}

func LoadConfig() (*config.AppConfig, error) {
	cfg := &config.AppConfig{}
	err := env.Parse(cfg)
	if err != nil {
		return nil, err
	}
	// eurka
	return cfg, nil
}
