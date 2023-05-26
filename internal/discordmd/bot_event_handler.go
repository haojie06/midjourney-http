// discord websocket event 响应
package discordmd

import (
	"runtime"
	"strconv"

	"github.com/bwmarrin/discordgo"
	"github.com/haojie06/midjourney-http/internal/logger"
)

func (bot *DiscordBot) onDiscordMessageWithEmbedsCreate(s *discordgo.Session, event *discordgo.MessageCreate) {
	if bot.config.DiscordChannelId != "" && event.ChannelID != bot.config.DiscordChannelId {
		return
	}
	// warn or error message are embeded messages with title and description
	if len(event.Embeds) == 0 {
		return
	}
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
				taskRuntime.taskResultChan <- TaskResult{
					TaskId:     taskId,
					Successful: false,
					Message:    embed.Title + " " + embed.Description,
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

// when receive message from discord(image generated, upscaled, etc.)
// origin message or upscaled message does not have embeded message
func (bot *DiscordBot) onDiscordMessageWithAttachmentsCreate(s *discordgo.Session, event *discordgo.MessageCreate) {
	if bot.config.DiscordChannelId != "" && event.ChannelID != bot.config.DiscordChannelId {
		return
	}
	// warn or error message are embeded messages with title and description
	if len(event.Attachments) == 0 {
		return
	}
	bot.runtimesLock.Lock()
	defer bot.runtimesLock.Unlock()
	// upscaled message has attachments
	for _, attachment := range event.Attachments {
		if event.ReferencedMessage == nil {
			// 原始图片没有referenced message
			// receive origin image, send upscale request depends on config
			taskKeywordHash, promptStr := getHashFromMessage(event.Content)
			taskRuntime := bot.getTaskRuntimeByTaskKeywordHash(taskKeywordHash)
			if taskRuntime == nil {
				logger.Warnf("bot: %s keywordHash %s is not created by this bot, prompt: %s", bot.UniqueId, taskKeywordHash, promptStr)
				return
			}
			logger.Infof("bot: %s task: %s receive origin image: %s", bot.UniqueId, taskRuntime.TaskId, attachment.URL)
			// we will use messageId to map upscaled image to origin image
			taskRuntime.OriginImageMessageId = event.ID
			taskRuntime.OriginImageURL = attachment.URL
			taskRuntime.OriginImageId = getFileIdFromURL(attachment.URL)
			if !taskRuntime.AutoUpscale {
				// only return origin image url, user can upscale it manually
				taskRuntime.taskResultChan <- TaskResult{
					TaskId:     taskRuntime.TaskId,
					Successful: true,
					Payload: ImageGenerationResultPayload{
						OriginImageURL: attachment.URL,
						ImageURLs:      []string{},
					},
				}
				return
			}
			// auto upscale enable
			taskRuntime.State = TaskStateAutoUpscaling
			for i := 1; i <= bot.config.UpscaleCount; i++ {
				if code := bot.upscale(taskRuntime.OriginImageId, strconv.Itoa(i), event.ID); code >= 400 {
					logger.Errorf("failed to upscale image, code: %d", code)
					taskRuntime.UpscaleProcessCount += 1
				} else {
					logger.Infof("task %s request to upscale image %s %d", taskRuntime.TaskId, taskRuntime.OriginImageId, i)
				}
			}
		} else {
			// upscaled 的图片有referenced message
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
				// 自动upscale时，接收到图片还需要判断是否接收到了所有图片，如果是则返回结果
				taskRuntime.UpscaleProcessCount += 1
				if taskRuntime.UpscaleProcessCount == bot.config.UpscaleCount {
					logger.Infof("task %s image generation finished, current goroutine count: %d", taskRuntime.TaskId, runtime.NumGoroutine())
					taskRuntime.taskResultChan <- TaskResult{
						TaskId:     taskRuntime.TaskId,
						Successful: true,
						Payload: ImageGenerationResultPayload{
							ImageURLs:      taskRuntime.UpscaledImageURLs,
							OriginImageURL: taskRuntime.OriginImageURL,
						},
					}
					bot.RemoveTaskRuntime(taskRuntime.TaskId)
				} else {
					logger.Infof("task %s image generation not finished, current images count: %d/%d", taskRuntime.TaskId, len(taskRuntime.UpscaledImageURLs), bot.config.UpscaleCount)
				}
			case TaskStateManualUpscaling:
				// get index from message
				index := getImageIndexFromMessage(event.Content)
				taskRuntime.taskResultChan <- TaskResult{
					TaskId:     taskRuntime.TaskId,
					Successful: true,
					Payload: ImageUpscaleResultPayload{
						ImageURL: attachment.URL,
						Index:    index,
					},
				}
			}
		}
	}
}

// when discord message updated (for example, when a request is intercepted by a filter)
func (bot *DiscordBot) onDiscordMessageUpdate(s *discordgo.Session, event *discordgo.MessageUpdate) {
	if bot.config.DiscordChannelId != "" && event.ChannelID != bot.config.DiscordChannelId {
		return
	}
	for _, embed := range event.Message.Embeds {
		if _, failed := FailedEmbededMessageTitlesInUpdate[embed.Title]; failed {
			// 大部分失败提示都是 embeded message
			taskKeywordHash, _ := getHashFromMessage(event.Message.Content)
			bot.runtimesLock.Lock()
			defer bot.runtimesLock.Unlock()
			taskRuntime := bot.getTaskRuntimeByTaskKeywordHash(taskKeywordHash)
			if taskRuntime == nil {
				logger.Warnf("bot: %s keywordHash %s is not created by this bot", bot.UniqueId, taskKeywordHash)
				return
			}
			logger.Infof("task %s failed, reason: %s descripiton: %s", taskRuntime.TaskId, embed.Title, embed.Description)
			taskRuntime.taskResultChan <- TaskResult{
				TaskId:     taskRuntime.TaskId,
				Successful: false,
				Message:    embed.Title + " " + embed.Description,
			}
			bot.RemoveTaskRuntime(taskRuntime.TaskId)
		} else if event.Interaction != nil {
			// 部分 interaction 的结果来源于 message update
			switch event.Interaction.Name {
			case "describe":
				taskRuntime := bot.getTaskRuntimeByInteractionId(event.Interaction.ID)
				if taskRuntime == nil {
					continue
				}
				taskRuntime.taskResultChan <- TaskResult{
					TaskId:     taskRuntime.TaskId,
					Successful: true,
					Message:    "success",
					Payload: ImageDescribeResultPayload{
						Description: embed.Description,
					},
				}
			}
		}
	}
}

// 在发送命令后，用其取回interaction的id
func (bot *DiscordBot) onMessageWithInteractionCreate(s *discordgo.Session, event *discordgo.MessageCreate) {
	if bot.config.DiscordChannelId != "" && event.ChannelID != bot.config.DiscordChannelId {
		return
	}
	if event.Interaction == nil {
		return
	}
	bot.interactionResponseMutex.Lock()
	bot.slashCommandResponse = SlashCommandResponse{
		InteractionId: event.Interaction.ID,
		Name:          event.Interaction.Name,
	}
	bot.interactionResponseMutex.Unlock()
	bot.interactionResponseCond.Broadcast()

}
