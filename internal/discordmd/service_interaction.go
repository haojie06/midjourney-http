// bot 对外提供的各方法, 调用后返回一个稍后会被填充的 channel, 用于接收结果
package discordmd

import (
	"encoding/json"
	"math"
	"mime/multipart"
	"strconv"
	"strings"
	"time"

	"github.com/haojie06/midjourney-http/internal/logger"
)

// imagine a image (create a task)
func (m *MidJourneyService) Imagine(botId, prompt, params string, fastMode, autoUpscale bool) (allocatedBotId string, taskId string, imagineResultChannel chan *ImageGenerationResult, err error) {
	var bot *DiscordBot
	var exist bool
	if botId == "" {
		bot = m.GetRandomBot()
	} else if bot, exist = m.discordBots[botId]; !exist {
		err = ErrBotNotFound
		return
	}
	allocatedBotId = bot.BotId
	bot.runtimesLock.Lock()
	defer bot.runtimesLock.Unlock()
	if len(bot.taskRuntimes) > bot.config.MaxUnfinishedTasks {
		err = ErrTooManyTasks
		return
	}

	seed := strconv.Itoa(bot.randGenerator.Intn(math.MaxUint32))
	params += " --seed " + seed
	// remove extra spaces
	prompt = strings.Join(strings.Fields(strings.Trim(strings.Trim(prompt, " ")+" "+params, " ")), " ")
	// midjourney will replace — to --, so we need to replace it before hash sum
	prompt = strings.ReplaceAll(prompt, "—", "--")
	// use hash for taskId
	taskId = getHashFromPrompt(prompt, seed)
	logger.Infof("task %s is starting, prompt: %s", taskId, prompt)
	imagineResultChannel = make(chan *ImageGenerationResult, bot.config.MaxUnfinishedTasks)
	bot.taskRuntimes[taskId] = &TaskRuntime{
		TaskId:                taskId,
		ImagineResultChannel:  imagineResultChannel,
		UpscaleResultChannels: make(map[string]chan *ImageUpscaleResult),
		DescribeResultChannel: make(chan *DescribeResult),
		UpscaledImageURLs:     make([]string, 0),
		AutoUpscale:           autoUpscale,
		CreatedAt:             time.Now().Unix(),
		UpdatedAt:             time.Now().Unix(),
		State:                 TaskStateCreated,
	}
	payload, _ := json.Marshal(ImageGenerationTaskPayload{
		Prompt:      prompt,
		FastMode:    fastMode,
		AutoUpscale: autoUpscale,
	})
	// send task
	bot.taskChan <- &MidjourneyTask{
		TaskId:   taskId,
		TaskType: MidjourneyTaskTypeImageGeneration,
		Payload:  payload,
	}
	return
}

// Upscale a image with given taskId and index
func (m *MidJourneyService) Upscale(botId, taskId, index string) (allocatedBotId string, upscaleResultChannel chan *ImageUpscaleResult, err error) {
	var bot *DiscordBot
	var exist bool
	if botId == "" {
		bot = m.GetRandomBot()
	} else if bot, exist = m.discordBots[botId]; !exist {
		err = ErrBotNotFound
		return
	}
	// find the task runtime, and get the result channel
	allocatedBotId = bot.BotId
	bot.runtimesLock.Lock()
	defer bot.runtimesLock.Unlock()
	taskRuntime, exist := bot.taskRuntimes[taskId]
	if !exist {
		err = ErrTaskNotFound
		return
	}
	taskRuntime.State = TaskStateManualUpscaling
	upscaleResultChannel = make(chan *ImageUpscaleResult)
	taskRuntime.UpscaleResultChannels[index] = upscaleResultChannel
	payload, _ := json.Marshal(ImageUpscaleTaskPayload{
		OriginImageId:        taskRuntime.OriginImageId,
		Index:                index,
		OriginImageMessageId: taskRuntime.OriginImageMessageId,
	})
	bot.taskChan <- &MidjourneyTask{
		TaskId:   taskId,
		TaskType: MidjourneyTaskTypeImageUpscale,
		Payload:  payload,
	}
	return
}

func (m *MidJourneyService) Describe(botId, taskId string, file *multipart.FileHeader, filename string, size int) (allocatedBotId string, describeResultChannel chan *DescribeResult, err error) {
	var bot *DiscordBot
	var exist bool
	if botId == "" {
		bot = m.GetRandomBot()
	} else if bot, exist = m.discordBots[botId]; !exist {
		err = ErrBotNotFound
		return
	}
	allocatedBotId = bot.BotId
	bot.runtimesLock.Lock()
	defer bot.runtimesLock.Unlock()
	bot.FileHeaders[taskId] = file
	taskRuntime := NewTaskRuntime(taskId, false)
	describeResultChannel = taskRuntime.DescribeResultChannel
	bot.taskRuntimes[taskId] = taskRuntime
	payload, _ := json.Marshal(ImageDescribeTaskPayload{
		ImageFileName: filename,
		ImageFileSize: size,
	})
	bot.taskChan <- &MidjourneyTask{
		TaskId:   taskId,
		TaskType: MidjourneyTaskTypeImageDescribe,
		Payload:  payload,
	}
	return
}
