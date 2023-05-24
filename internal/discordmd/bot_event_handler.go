// discord websocket event 响应
package discordmd

import (
	"runtime"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/haojie06/midjourney-http/internal/logger"
)

// when receive message from discord(image generated, upscaled, etc.)
func (bot *DiscordBot) onDiscordMessageCreate(s *discordgo.Session, event *discordgo.MessageCreate) {
	// warn or error message are embeded messages with title and description
	if len(event.Embeds) > 0 {
		for _, embed := range event.Embeds {
			if _, failed := FailedEmbededMessageTitlesInCreate[embed.Title]; failed {
				if embed.Footer == nil {
					logger.Warnf("embed footer is nil, embed: %+v", embed) // Queue full message does not have footer
					continue
				}
				// warn or error message will contain origin prompt in footer, so we can get taskId from it
				taskId := getHashFromEmbeds(embed.Footer.Text)
				logger.Warnf("task %s receive embeded message: %s --- %s", taskId, embed.Title, embed.Description)
				bot.runtimesLock.Lock()
				defer bot.runtimesLock.Unlock()
				if taskRuntime, exist := bot.taskRuntimes[taskId]; exist {
					logger.Infof("task %s failed, reason: %s descripiton: %s", taskId, embed.Title, embed.Description)
					taskRuntime.ImagineResultChannel <- &ImageGenerationResult{
						TaskId:     taskId,
						Successful: false,
						Message:    embed.Title + " " + embed.Description,
						ImageURLs:  []string{},
					}
					bot.RemoveTaskRuntime(taskId)
				} else {
					logger.Warnf("task %s not exist, embed: %+v", taskId, embed)
				}
				return
			} else {
				logger.Warnf("unknown embed title found: %s \n %s", embed.Title, embed.Description)
			}
		}
	}

	// origin message or upscaled message does not have embeded message
	bot.runtimesLock.Lock()
	defer bot.runtimesLock.Unlock()
	// upscaled message has attachments
	for _, attachment := range event.Attachments {
		if event.ReferencedMessage == nil {
			// receive origin image, send upscale request depends on config
			taskId, promptStr := getHashFromMessage(event.Content)
			logger.Infof("task %s receive origin image: %s", taskId, attachment.URL)
			taskRuntime, exist := bot.taskRuntimes[taskId]
			if taskId != "" && exist && taskRuntime != nil {
				// we will use messageId to map upscaled image to origin image
				taskRuntime.OriginImageMessageId = event.ID
				taskRuntime.OriginImageURL = attachment.URL
				taskRuntime.OriginImageId = getFileIdFromURL(attachment.URL)
				if !taskRuntime.AutoUpscale {
					// only return origin image url, user can upscale it manually
					taskRuntime.ImagineResultChannel <- &ImageGenerationResult{
						TaskId:         taskId,
						Successful:     true,
						OriginImageURL: attachment.URL,
						ImageURLs:      []string{},
					}
					return
				}
				// auto upscale
				taskRuntime.State = TaskStateAutoUpscaling
				for i := 1; i <= bot.config.UpscaleCount; i++ {
					if code := bot.upscaleRequest(taskRuntime.OriginImageId, strconv.Itoa(i), event.ID); code >= 400 {
						logger.Errorf("failed to upscale image, code: %d", code)
						taskRuntime.UpscaleProcessCount += 1
					} else {
						logger.Infof("task %s request to upscale image %s %d", taskId, taskRuntime.OriginImageId, i)
					}
					time.Sleep(time.Duration((bot.randGenerator.Intn(3000))+1000) * time.Millisecond)
				}
			} else {
				logger.Warnf("task %s is not created by this bot, prompt: %s", taskId, promptStr)
			}
		} else {
			// receive upscaling image, use referenced message id to map to taskId
			// when queue is full, we will also receive a message which refer to origin message
			taskRuntime := bot.getTaskRuntimeByOriginMessageId(event.ReferencedMessage.ID)
			if taskRuntime == nil {
				logger.Warnf("no local task found for referenced message: %s", event.ReferencedMessage.ID) // non-local task result
				return
			}
			logger.Infof("task %s receives upscaled image: %s", taskRuntime.TaskId, attachment.URL)
			taskRuntime.UpscaledImageURLs = append(taskRuntime.UpscaledImageURLs, attachment.URL)
			switch taskRuntime.State {
			case TaskStateAutoUpscaling:
				taskRuntime.UpscaleProcessCount += 1
				if taskRuntime.UpscaleProcessCount == bot.config.UpscaleCount {
					logger.Infof("task %s image generation finished, current goroutine count: %d", taskRuntime.TaskId, runtime.NumGoroutine())
					taskRuntime.ImagineResultChannel <- &ImageGenerationResult{
						TaskId:         taskRuntime.TaskId,
						Successful:     true,
						ImageURLs:      taskRuntime.UpscaledImageURLs,
						OriginImageURL: taskRuntime.OriginImageURL,
					}
					bot.RemoveTaskRuntime(taskRuntime.TaskId)
				} else {
					logger.Infof("task %s image generation not finished, current images count: %d/%d", taskRuntime.TaskId, len(taskRuntime.UpscaledImageURLs), bot.config.UpscaleCount)
				}
			case TaskStateManualUpscaling:
				// get index from message
				index := getImageIndexFromMessage(event.Content)
				c, exist := taskRuntime.UpscaleResultChannels[index]
				if !exist {
					logger.Warnf("task %s does not have matched upscale channel, index: %s", taskRuntime.TaskId, index)
					return
				}
				c <- &ImageUpscaleResult{
					TaskId:     taskRuntime.TaskId,
					Successful: true,
					ImageURL:   attachment.URL,
					Index:      index,
				}
			}
		}
	}
}

// when discord message updated (for example, when a request is intercepted by a filter)
func (bot *DiscordBot) onDiscordMessageUpdate(s *discordgo.Session, event *discordgo.MessageUpdate) {
	for _, embed := range event.Message.Embeds {
		if _, failed := FailedEmbededMessageTitlesInUpdate[embed.Title]; failed {
			taskId, _ := getHashFromMessage(event.Message.Content)
			bot.runtimesLock.Lock()
			defer bot.runtimesLock.Unlock()
			if taskRuntime, exist := bot.taskRuntimes[taskId]; exist {
				logger.Infof("task %s failed, reason: %s descripiton: %s", taskId, embed.Title, embed.Description)
				taskRuntime.ImagineResultChannel <- &ImageGenerationResult{
					TaskId:     taskId,
					Successful: false,
					Message:    embed.Title + " " + embed.Description,
					ImageURLs:  []string{},
				}
				bot.RemoveTaskRuntime(taskId)
			}
		} else if event.Interaction != nil {
			switch event.Interaction.Name {
			case "describe":
				taskRuntime := bot.getTaskRuntimeByInteractionId(event.Interaction.ID)
				if taskRuntime == nil {
					continue
				}
				taskRuntime.DescribeResultChannel <- &DescribeResult{
					TaskId:      taskRuntime.TaskId,
					Successful:  true,
					Message:     "success",
					Description: embed.Description,
				}
			}
		}
	}
}

// when a discord interaction created (for example, when a user click a button or use a slash command)
// 当前面临的最大问题是, discord 的 interaction api request 时，我们无法拿到创建的 interaction 的 id， 因此无法轻易将后续的interaction响应与http请求对应起来
// 因此我们考虑，所有的 request 发出后，都等待 interaction create 事件，并将 interaction id 与 task id 关联起来
// 于是在发出 request 之后，任务队列处要阻塞，等待 interaction create 事件(但是又并非所有的 interaction create 事件都是我们想要的)
// 因此，所有的 interaction request 都需要走 taskChan 来分发，保证没有同时进行的 interaction request
func (bot *DiscordBot) onDiscordInteractionCreate(s *discordgo.Session, event discordgo.InteractionCreate) {
	// record current taskId and interactionId
	// d, _ := json.Marshal(event)
	// logger.Debugf("receive interaction create event: %s", string(d))
	// bot.runtimesLock.Lock()
	// defer bot.runtimesLock.Unlock()
	// if taskRuntime, exist := bot.taskRuntimes[bot.activeTaskId]; exist {
	// 	taskRuntime.InteractionId = event.Interaction.ID
	// 	logger.Infof("%s create interaction: %s", bot.activeTaskId, event.Interaction.ID)
	// }
}
