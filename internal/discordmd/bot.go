package discordmd

import (
	"math/rand"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/haojie06/midjourney-http/internal/logger"
)

type DiscordBot struct {
	UniqueId string

	BotId string

	config DiscordBotConfig

	discordSession *discordgo.Session

	taskChan chan *MidjourneyTask

	interactionResponseChan chan *InteractionResponse

	taskRuntimes map[string]*TaskRuntime

	FileHeaders sync.Map

	discordCommands map[string]*discordgo.ApplicationCommand

	runtimesLock sync.RWMutex

	randGenerator *rand.Rand
}

func NewDiscordBot(config DiscordBotConfig) (*DiscordBot, error) {
	logger.Infof("creating discord bot, uniqueId: %s", config.UniqueId)
	ds, err := discordgo.New(config.DiscordToken)
	if err != nil {
		return nil, err
	}
	commands, err := ds.ApplicationCommands(config.DiscordAppId, "")
	if err != nil {
		return nil, err
	}
	bot := &DiscordBot{
		config:                  config,
		UniqueId:                config.UniqueId,
		BotId:                   uuid.New().String(),
		discordSession:          ds,
		interactionResponseChan: make(chan *InteractionResponse),
		taskChan:                make(chan *MidjourneyTask, 1),
		taskRuntimes:            make(map[string]*TaskRuntime),
		FileHeaders:             sync.Map{},
		discordCommands:         make(map[string]*discordgo.ApplicationCommand),
		runtimesLock:            sync.RWMutex{},
		randGenerator:           rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	for _, command := range commands {
		bot.discordCommands[command.Name] = command
	}
	bot.discordSession.AddHandler(bot.onDiscordMessageWithEmbedsCreate)
	bot.discordSession.AddHandler(bot.onDiscordMessageWithAttachmentsCreate)

	bot.discordSession.AddHandler(bot.onDiscordMessageUpdate)
	bot.discordSession.AddHandler(bot.onDiscordInteractionCreate)
	bot.discordSession.AddHandler(bot.onMessageWithInteractionCreate)
	bot.discordSession.Identify.Intents = discordgo.IntentsAll
	if err := bot.discordSession.Open(); err != nil {
		return nil, err
	}
	return bot, nil
}

// task worker loop
func (bot *DiscordBot) Start() {
	// reveive task and send interaction request
	for {
		task := <-bot.taskChan
		switch task.TaskType {
		// TODO 增加通用错误响应结构体
		case MidjourneyTaskTypeImageGeneration:
			logger.Infof("bot: %s receive image generation task: %s", bot.UniqueId, task.TaskId)
			if err := bot.ImagineTaskHandler(task.TaskId, task.Payload); err != nil {
				logger.Errorf("failed to handle imagine task, err: %s", err)
				continue
			}
		case MidjourneyTaskTypeImageUpscale:
			logger.Infof("bot: %s receive image upscale task: %s", bot.UniqueId, task.TaskId)
			if err := bot.UpscaleTaskHandler(task.TaskId, task.Payload); err != nil {
				logger.Errorf("failed to handle upscale task, err: %s", err)
				continue
			}
		case MidjourneyTaskTypeImageDescribe:
			logger.Infof("bot: %s receive image describe task: %s", bot.UniqueId, task.TaskId)
			if err := bot.DescribeTaskHandler(task.TaskId, task.Payload); err != nil {
				logger.Errorf("failed to handle describe task, err: %s", err)
				continue
			}
		default:
			logger.Warnf("unknown task type: %s", task.TaskType)
		}
	}
}

// remove task when timeout, no mutex lock
func (bot *DiscordBot) RemoveTaskRuntime(taskId string) {
	delete(bot.taskRuntimes, taskId)
}

func (bot *DiscordBot) getTaskRuntimeByOriginMessageId(messageId string) *TaskRuntime {
	for _, taskRuntime := range bot.taskRuntimes {
		if taskRuntime.OriginImageMessageId == messageId {
			return taskRuntime
		}
	}
	return nil
}

func (bot *DiscordBot) getTaskRuntimeByInteractionId(interactionId string) *TaskRuntime {
	if interactionId == "" {
		return nil
	}
	for _, taskRuntime := range bot.taskRuntimes {
		if taskRuntime.InteractionId == interactionId {
			return taskRuntime
		}
	}
	return nil
}

func (bot *DiscordBot) getTaskRuntimeByTaskKeywordHash(taskKeywordHash string) *TaskRuntime {
	if taskKeywordHash == "" {
		return nil
	}
	for _, taskRuntime := range bot.taskRuntimes {
		if taskRuntime.TaskKeywordHash == taskKeywordHash {
			return taskRuntime
		}
	}
	return nil
}
