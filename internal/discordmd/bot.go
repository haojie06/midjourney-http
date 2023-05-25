package discordmd

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"mime/multipart"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/haojie06/midjourney-http/internal/logger"
)

type DiscordBot struct {
	UniqueId string

	BotId string

	config *DiscordBotConfig

	discordSession *discordgo.Session

	taskChan chan *MidjourneyTask

	taskRuntimes map[string]*TaskRuntime

	FileHeaders map[string]*multipart.FileHeader

	discordCommands map[string]*discordgo.ApplicationCommand

	runtimesLock sync.RWMutex

	randGenerator *rand.Rand
}

func NewDiscordBot(config *DiscordBotConfig) (*DiscordBot, error) {
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
		config:          config,
		UniqueId:        config.UniqueId,
		BotId:           uuid.New().String(),
		discordSession:  ds,
		taskChan:        make(chan *MidjourneyTask, 1),
		taskRuntimes:    make(map[string]*TaskRuntime),
		FileHeaders:     make(map[string]*multipart.FileHeader),
		discordCommands: make(map[string]*discordgo.ApplicationCommand),
		runtimesLock:    sync.RWMutex{},
		randGenerator:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	for _, command := range commands {
		bot.discordCommands[command.Name] = command
	}
	bot.discordSession.AddHandler(bot.onDiscordMessageCreate)
	bot.discordSession.AddHandler(bot.onDiscordMessageUpdate)
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
		time.Sleep(2 * time.Second) // to avoid discord 429
		switch task.TaskType {
		case MidjourneyTaskTypeImageGeneration:
			var taskPayload ImageGenerationTaskPayload
			if err := json.Unmarshal(task.Payload, &taskPayload); err != nil {
				logger.Errorf("failed to unmarshal image generation payload, err: %s", err)
				continue
			}
			logger.Infof("bot: %s receive image generation task: %s prompt: %s", bot.UniqueId, task.TaskId, taskPayload.Prompt)
			// XXX 这里同样会导致 interaction created
			if taskPayload.FastMode {
				if status := bot.switchMode(true); status >= 400 {
					logger.Warnf("switch mode to fast failed, status code: %d", status)
				}
				time.Sleep(time.Duration((bot.randGenerator.Intn(1000))+1000) * time.Millisecond)
			}
			statusCode := bot.imagineRequest(task.TaskId, taskPayload.Prompt)
			if statusCode >= 400 {
				logger.Warnf("task %s failed, status code: %d", task.TaskId, statusCode)
				bot.runtimesLock.Lock()
				if taskRuntime, exist := bot.taskRuntimes[task.TaskId]; exist {
					taskRuntime.ImagineResultChannel <- &ImageGenerationResult{
						TaskId:     task.TaskId,
						Successful: false,
						Message:    fmt.Sprintf("imagine task: %s failed, code: %d", task.TaskId, statusCode),
						ImageURLs:  []string{},
					}
					bot.RemoveTaskRuntime(task.TaskId)
				}
				bot.runtimesLock.Unlock()
			}
			time.Sleep(time.Duration((bot.randGenerator.Intn(1000))+1000) * time.Millisecond)
			// switch back to slow mode
			if taskPayload.FastMode {
				if status := bot.switchMode(false); status >= 400 {
					logger.Warnf("switch mode back to slow failed, status code: %d", status)
				}
			}
		case MidjourneyTaskTypeImageUpscale:
			logger.Infof("bot: %s receive image upscale task: %s", bot.UniqueId, task.TaskId)
			var taskPayload ImageUpscaleTaskPayload
			if err := json.Unmarshal(task.Payload, &taskPayload); err != nil {
				logger.Errorf("failed to unmarshal image upscale payload, err: %s", err)
				continue
			}
			if code := bot.upscaleRequest(taskPayload.OriginImageId, taskPayload.Index, taskPayload.OriginImageMessageId); code >= 400 {
				logger.Errorf("task %s failed, status code: %d", task.TaskId, code)
				continue
			}
		case MidjourneyTaskTypeImageDescribe:
			logger.Infof("bot: %s receive image describe task: %s", bot.UniqueId, task.TaskId)
			var taskPayload ImageDescribeTaskPayload
			if err := json.Unmarshal(task.Payload, &taskPayload); err != nil {
				logger.Errorf("failed to unmarshal image describe payload, err: %s", err)
				continue
			}
			fileHeader, exist := bot.FileHeaders[task.TaskId]
			if !exist {
				logger.Errorf("failed to get image file header, task id: %s", task.TaskId)
				continue
			}
			fileReader, err := fileHeader.Open()
			if err != nil {
				logger.Errorf("failed to open image file header, err: %s", err)
				delete(bot.FileHeaders, task.TaskId)
				continue
			}

			if code := bot.describeRequest(taskPayload.ImageFileName, taskPayload.ImageFileSize, fileReader); code >= 400 {
				logger.Errorf("task %s failed, status code: %d", task.TaskId, code)
				if taskRuntime, exist := bot.taskRuntimes[task.TaskId]; exist {
					taskRuntime.DescribeResultChannel <- &DescribeResult{
						TaskId:     task.TaskId,
						Successful: false,
						Message:    fmt.Sprintf("imagine task: %s failed, code: %d", task.TaskId, code),
					}
				}
				continue
			}
			fileReader.Close()
			delete(bot.FileHeaders, task.TaskId)
		default:
			logger.Warnf("unknown task type: %s", task.TaskType)
		}
		time.Sleep(time.Duration((bot.randGenerator.Intn(1000))+1000) * time.Millisecond)
	}
}

// remove task when timeout, no mutex lock
func (bot *DiscordBot) RemoveTaskRuntime(taskId string) {
	if r, exist := bot.taskRuntimes[taskId]; exist {
		close(r.ImagineResultChannel)
		delete(bot.taskRuntimes, taskId)
	}
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
