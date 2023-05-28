// discord websocket event 响应
package discordmd

import (
	"runtime"
	"strconv"

	"github.com/bwmarrin/discordgo"
)

func (bot *DiscordBot) onDiscordMessageWithEmbedsCreate(s *discordgo.Session, event *discordgo.MessageCreate) {
	if bot.config.DiscordChannelId != "" && event.ChannelID != bot.config.DiscordChannelId {
		return
	}
	// warn or error message are embeded messages with title and description
	if len(event.Embeds) == 0 {
		return
	}

	bot.runtimesLock.Lock()
	defer bot.runtimesLock.Unlock()
	embed := event.Embeds[0]
	// 部分title预示着任务失败
	if _, failed := FailedEmbededMessageTitlesInCreate[embed.Title]; failed {
		if embed.Footer == nil {
			bot.logger.Warnf("embed footer is nil, embed: %+v", embed)
			// continue
			return
		}
		if event.Interaction == nil {
			bot.logger.Warnf("interaction is nil, embed: %+v", embed)
		}
		// warn or error message will contain origin prompt in footer, so we can get taskId from it
		// taskKeywordHash := getHashFromEmbeds(embed.Footer.Text)
		// taskRuntime := bot.getTaskRuntimeByTaskKeywordHash(taskKeywordHash)
		taskRuntime := bot.getTaskRuntimeByInteractionId(event.Interaction.ID)
		if taskRuntime == nil {
			bot.logger.Warnf("interaction %s is not created by this bot, prompt: %s", event.Interaction.ID, embed.Footer.Text)
			return
		}
		bot.logger.Warnf("task %s failed, reason: %s descripiton: %s", taskRuntime.TaskId, embed.Title, embed.Description)
		taskRuntime.Response(false, embed.Title+"\n"+embed.Description, nil)
		bot.RemoveTaskRuntime(taskRuntime.TaskId)
	} else {
		bot.logger.Warnf("unknown embed title found: %s\n%s", embed.Title, embed.Description)
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
	// 当前只处理单一attachment
	attachment := event.Attachments[0]
	if event.ReferencedMessage == nil {
		// 原始图片没有referenced message
		// receive origin image, send upscale request depends on config
		taskKeywordHash, promptStr := getHashFromMessage(event.Content)
		taskRuntime := bot.getTaskRuntimeByTaskKeywordHash(taskKeywordHash)
		if taskRuntime == nil {
			bot.logger.Warnf("task with keywordHash %s is not created by this bot, prompt: %s", taskKeywordHash, promptStr)
			return
		}
		bot.logger.Infof("task: %s receives origin image: %s", taskRuntime.TaskId, attachment.URL)
		// we will use messageId to map upscaled image to origin image
		taskRuntime.OriginImageMessageId = event.ID
		taskRuntime.OriginImageURL = attachment.URL
		taskRuntime.OriginImageId = getFileIdFromURL(attachment.URL)
		if !taskRuntime.AutoUpscale {
			// only return origin image url, user can upscale it manually
			taskRuntime.Response(true, "", ImageGenerationResultPayload{
				OriginImageURL: attachment.URL,
				ImageURLs:      []string{},
			})
			return
		}

		// when auto upscale enable
		taskRuntime.State = TaskStateAutoUpscaling
		for i := 1; i <= bot.config.UpscaleCount; i++ {
			status, err := bot.upscale(taskRuntime.OriginImageId, strconv.Itoa(i), event.ID)
			if err != nil {
				bot.logger.Errorf("failed to upscale image, err: %s", err.Error())
				taskRuntime.UpscaleProcessCount += 1
			} else if status >= 400 {
				bot.logger.Errorf("failed to upscale image, status: %d", status)
				taskRuntime.UpscaleProcessCount += 1
			} else {
				bot.logger.Infof("task %s autoUpscale image %s %d", taskRuntime.TaskId, taskRuntime.OriginImageId, i)
			}
		}
	} else {
		// upscaled 的图片有referenced message
		// receive upscaling image, use referenced message id to map to taskId
		// when queue is full, we will also receive a message which refer to origin message
		taskRuntime := bot.getTaskRuntimeByOriginMessageId(event.ReferencedMessage.ID)
		if taskRuntime == nil {
			bot.logger.Warnf("no local task found for referenced message: %s", event.ReferencedMessage.ID) // non-local task result
			return
		}
		bot.logger.Infof("task %s receives upscaled image: %s", taskRuntime.TaskId, attachment.URL)
		taskRuntime.UpscaledImageURLs = append(taskRuntime.UpscaledImageURLs, attachment.URL)
		switch taskRuntime.State {
		case TaskStateAutoUpscaling:
			// 自动upscale时，接收到图片还需要判断是否接收到了所有图片，如果是则返回结果
			taskRuntime.UpscaleProcessCount += 1
			if taskRuntime.UpscaleProcessCount == bot.config.UpscaleCount {
				bot.logger.Infof("task %s image generation is completed, current goroutine count: %d", taskRuntime.TaskId, runtime.NumGoroutine())
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
				bot.logger.Infof("task %s image generation is not completed, waiting for images: %d/%d", taskRuntime.TaskId, len(taskRuntime.UpscaledImageURLs), bot.config.UpscaleCount)
			}
		case TaskStateManualUpscaling:
			// get index from message
			index := getImageIndexFromMessage(event.Content)
			taskRuntime.Response(true, "", ImageUpscaleResultPayload{
				ImageURL: attachment.URL,
				Index:    index,
			})
		}
	}
}

// when discord message updated (for example, when a request is intercepted by a filter)
func (bot *DiscordBot) onDiscordMessageUpdate(s *discordgo.Session, event *discordgo.MessageUpdate) {
	if bot.config.DiscordChannelId != "" && event.ChannelID != bot.config.DiscordChannelId {
		return
	}
	bot.runtimesLock.Lock()
	defer bot.runtimesLock.Unlock()
	for _, embed := range event.Message.Embeds {
		if _, failed := FailedEmbededMessageTitlesInUpdate[embed.Title]; failed {
			// 大部分失败提示都是 embeded message
			taskKeywordHash, _ := getHashFromMessage(event.Message.Content)
			taskRuntime := bot.getTaskRuntimeByTaskKeywordHash(taskKeywordHash)
			if taskRuntime == nil {
				bot.logger.Warnf("task with keywordHash %s is not created by this bot", taskKeywordHash)
				return
			}
			bot.logger.Infof("task %s failed, reason: %s descripiton: %s", taskRuntime.TaskId, embed.Title, embed.Description)
			taskRuntime.Response(false, embed.Title+" "+embed.Description, nil)
			bot.RemoveTaskRuntime(taskRuntime.TaskId)
		} else if event.Interaction != nil {
			// 部分 interaction 的结果来源于 message update
			switch event.Interaction.Name {
			case "describe":
				// TODO 记录进度
				taskRuntime := bot.getTaskRuntimeByInteractionId(event.Interaction.ID)
				if taskRuntime == nil {
					continue
				}
				taskRuntime.Response(true, "", ImageDescribeResultPayload{
					Description: embed.Description,
				})
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
