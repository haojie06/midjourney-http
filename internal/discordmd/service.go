package discordmd

import (
	"fmt"
	"math/rand"

	"github.com/haojie06/midjourney-http/internal/logger"
)

var (
	MidJourneyServiceApp               *MidJourneyService
	ErrTooManyTasks                    = fmt.Errorf("too many tasks")
	ErrTaskNotFound                    = fmt.Errorf("task not found")
	ErrFailedToCreateTask              = fmt.Errorf("failed to create task")
	ErrFailedToDescribeImage           = fmt.Errorf("failed to describe image")
	ErrBotNotFound                     = fmt.Errorf("bot not found")
	FailedEmbededMessageTitlesInCreate = map[string]struct{}{
		"Blocked":                            {},
		"Banned prompt":                      {},
		"Invalid parameter":                  {},
		"Banned prompt detected":             {},
		"Invalid link":                       {},
		"Sorry! Could not complete the job!": {},
		"Action needed to continue":          {},
		"Queue full":                         {},
		"Action required to continue":        {},
		"Job action restricted":              {},
		"Empty prompt":                       {},
	}
	FailedEmbededMessageTitlesInUpdate = map[string]struct{}{
		"Request cancelled due to image filters": {},
	}
)

func init() {
	MidJourneyServiceApp = &MidJourneyService{
		discordBots: make(map[string]*DiscordBot),
	}
}

type MidJourneyService struct {
	// request interaction -> get interaction id -> request another interaction, before an interaction is created, no more interaction can be created

	discordBots map[string]*DiscordBot
}

func (m *MidJourneyService) Start(botConfigs []DiscordBotConfig) {
	for _, botConfig := range botConfigs {
		bot, err := NewDiscordBot(&botConfig)
		if err != nil {
			logger.Errorf("failed to create discord bot, err: %s", err)
			continue
		}
		m.discordBots[bot.BotId] = bot
		go bot.Start()
	}
}

// get a random bot
func (m *MidJourneyService) GetRandomBot() *DiscordBot {
	keys := make([]string, 0, len(m.discordBots))
	for k := range m.discordBots {
		keys = append(keys, k)
	}
	randomKey := keys[rand.Intn(len(keys))]
	return m.discordBots[randomKey]
}
