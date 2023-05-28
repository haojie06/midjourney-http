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

	taskRuntimes map[string]*TaskRuntime

	FileHeaders sync.Map

	discordCommands map[string]*discordgo.ApplicationCommand

	runtimesLock sync.RWMutex

	randGenerator *rand.Rand

	interactionResponseMutex *sync.RWMutex

	interactionResponseCond *sync.Cond

	slashCommandResponse SlashCommandResponse

	logger *logger.CustomLogger
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
	interactionResposneMutex := sync.RWMutex{}
	bot := &DiscordBot{
		config:                   config,
		UniqueId:                 config.UniqueId,
		BotId:                    uuid.New().String(),
		discordSession:           ds,
		taskChan:                 make(chan *MidjourneyTask, 1),
		taskRuntimes:             make(map[string]*TaskRuntime),
		FileHeaders:              sync.Map{},
		discordCommands:          make(map[string]*discordgo.ApplicationCommand),
		runtimesLock:             sync.RWMutex{},
		randGenerator:            rand.New(rand.NewSource(time.Now().UnixNano())),
		interactionResponseMutex: &interactionResposneMutex,
		interactionResponseCond:  sync.NewCond(&interactionResposneMutex),
		logger:                   logger.NewCustomLogger().With("uniqueId", config.UniqueId),
	}
	for _, command := range commands {
		bot.discordCommands[command.Name] = command
	}
	bot.discordSession.AddHandler(bot.onDiscordMessageWithEmbedsCreate)
	bot.discordSession.AddHandler(bot.onDiscordMessageWithAttachmentsCreate)

	bot.discordSession.AddHandler(bot.onDiscordMessageUpdate)
	bot.discordSession.AddHandler(bot.onMessageWithInteractionCreate)
	bot.discordSession.Identify.Intents = discordgo.IntentsAll
	if err := bot.discordSession.Open(); err != nil {
		return nil, err
	}
	return bot, nil
}

// task worker loop
// 接收任务，并向discord发送请求
func (bot *DiscordBot) Start() {
	for {
		task := <-bot.taskChan
		bot.logger.Infof("receive %s task: %s", task.TaskType, task.TaskId)
		switch task.TaskType {
		case MidjourneyTaskTypeImageGeneration:
			bot.ImagineTaskHandler(task.TaskId, task.Payload)
		case MidjourneyTaskTypeImageUpscale:
			bot.UpscaleTaskHandler(task.TaskId, task.Payload)
		case MidjourneyTaskTypeImageDescribe:
			bot.DescribeTaskHandler(task.TaskId, task.Payload)
		default:
			bot.logger.Warnf("found unknown task type: %s", task.TaskType)
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
