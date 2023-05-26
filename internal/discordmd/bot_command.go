// bot - discord 内部的命令实现, 主要为 interaction 请求的发送实现
package discordmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/haojie06/midjourney-http/internal/logger"
)

type DiscordCommand string

const (
	DiscordCommandFast     DiscordCommand = "fast"
	DiscordCommandRelax    DiscordCommand = "relax"
	DiscordCommandImagine  DiscordCommand = "imagine"
	DiscordCommandUpscale  DiscordCommand = "upscale"
	DiscordCommandDescribe DiscordCommand = "describe"
)

func (bot *DiscordBot) sendRequest(payload []byte) int {
	request, err := http.NewRequest("POST", "https://discord.com/api/v9/interactions", bytes.NewBuffer(payload))
	if err != nil {
		logger.Errorf("Error creating request: %s", err.Error())
		return 500
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", bot.config.DiscordToken)

	resposne, err := http.DefaultClient.Do(request)
	if err != nil {
		logger.Errorf("Error sending request: %s", err.Error())
		return 500
	}
	defer resposne.Body.Close()
	return resposne.StatusCode
}
func checkCommandResponse(commandType DiscordCommand, slashCommandResponse SlashCommandResponse) bool {
	return slashCommandResponse.Name == string(commandType)
}

// 部分指令(目前除了upscale)，在发送执行请求后，需要阻塞等待，拿到interactionId
func (bot *DiscordBot) executeSlashCommand(commandType DiscordCommand, commandPayload []byte) (status int, interactionId string) {
	// 通过 sync.cond 拿到执行结果
	status = bot.sendRequest(commandPayload)
	sigChan := make(chan struct{})
	// 防止部分指令在发送后，没有收到响应，导致一直阻塞
	timoutChan := time.After(3 * time.Minute)
	go func() {
		bot.interactionResponseMutex.Lock()
		// 直到收到对应的响应
		for !checkCommandResponse(commandType, bot.slashCommandResponse) {
			bot.interactionResponseCond.Wait()
		}
		interactionId = bot.slashCommandResponse.InteractionId
		// 移除命令，因为一个响应只对应一个请求
		// bot.slashCommandResponse = SlashCommandResponse{}
		bot.interactionResponseMutex.Unlock()
		time.Sleep(time.Duration((bot.randGenerator.Intn(1000))+1000) * time.Millisecond)
		sigChan <- struct{}{}
	}()
	select {
	case <-sigChan:
		return
	case <-timoutChan:
		status = 408
		return
	}
}

// 类似于 button 点击这种交互，是不需要interaction Id的
func (bot *DiscordBot) executeInteractionCommand(commandPayload []byte) (status int) {
	status = bot.sendRequest(commandPayload)
	time.Sleep(time.Duration((bot.randGenerator.Intn(1000))+1000) * time.Millisecond)
	return
}

func (bot *DiscordBot) buildModeSwitchPayload(fast bool) (commandPayload []byte, err error) {
	var commnad *discordgo.ApplicationCommand
	var exists bool
	if fast {
		commnad, exists = bot.discordCommands["fast"]
	} else {
		commnad, exists = bot.discordCommands["relax"]
	}
	if !exists || commnad == nil {
		err = ErrCommandNotFound
		return
	}
	payload := InteractionRequest{
		Type:          2,
		ApplicationID: commnad.ApplicationID,
		ChannelID:     bot.config.DiscordChannelId,
		SessionID:     bot.config.DiscordSessionId,
		GuildID:       bot.config.DiscordGuildId,
		Data: InteractionRequestData{
			Version:            commnad.Version,
			ID:                 commnad.ID,
			Name:               commnad.Name,
			Type:               int(commnad.Type),
			Options:            []*discordgo.ApplicationCommandInteractionDataOption{},
			ApplicationCommand: commnad,
			Attachments:        []interface{}{},
		},
	}
	commandPayload, err = json.Marshal(payload)
	return
}

func (bot *DiscordBot) buildImaginePayload(taskId string, prompt string) (commandPayload []byte, err error) {
	imagineCommand, exists := bot.discordCommands["imagine"]
	if !exists {
		err = ErrCommandNotFound
		return
	}
	var dataOptions []*discordgo.ApplicationCommandInteractionDataOption
	dataOptions = append(dataOptions, &discordgo.ApplicationCommandInteractionDataOption{
		Type:  3,
		Name:  "prompt",
		Value: prompt,
	})
	payload := InteractionRequest{
		Type:          2,
		ApplicationID: imagineCommand.ApplicationID,
		ChannelID:     bot.config.DiscordChannelId,
		SessionID:     bot.config.DiscordSessionId,
		GuildID:       bot.config.DiscordGuildId,
		Data: InteractionRequestData{
			Version:            imagineCommand.Version,
			ID:                 imagineCommand.ID,
			Name:               imagineCommand.Name,
			Type:               int(imagineCommand.Type),
			Options:            dataOptions,
			ApplicationCommand: imagineCommand,
			Attachments:        []interface{}{},
		},
	}
	commandPayload, err = json.Marshal(payload)
	return
}

// different from slash command interaction
func (bot *DiscordBot) buildUpscalePayload(id, index, messageId string) (commandPayload []byte, err error) {
	payload := InteractionRequestTypeThree{
		Type:          3,
		MessageFlags:  0,
		MessageID:     messageId,
		ApplicationID: bot.config.DiscordAppId,
		ChannelID:     bot.config.DiscordChannelId,
		GuildID:       bot.config.DiscordGuildId,
		SessionID:     bot.config.DiscordSessionId,
		Data: UpSampleData{
			ComponentType: 2,
			CustomID:      fmt.Sprintf("MJ::JOB::upsample::%s::%s", index, id),
		},
	}
	commandPayload, err = json.Marshal(payload)
	return
}

func (bot *DiscordBot) describeRequest(filename, uploadFilename string) (commandPayload []byte, err error) {
	describeCommand, exists := bot.discordCommands["describe"]
	if !exists {
		err = ErrCommandNotFound
		return
	}

	var dataOptions []*discordgo.ApplicationCommandInteractionDataOption
	dataOptions = append(dataOptions, &discordgo.ApplicationCommandInteractionDataOption{
		Type:  11,
		Name:  "image",
		Value: 0,
	})
	payload := InteractionRequest{
		Type:          2,
		ApplicationID: describeCommand.ApplicationID,
		ChannelID:     bot.config.DiscordChannelId,
		GuildID:       bot.config.DiscordGuildId,
		SessionID:     bot.config.DiscordSessionId,
		Data: InteractionRequestData{
			Version:            describeCommand.Version,
			ID:                 describeCommand.ID,
			Name:               describeCommand.Name,
			Type:               int(describeCommand.Type),
			Options:            dataOptions,
			ApplicationCommand: describeCommand,
			Attachments: []interface{}{AttachmentInCommand{
				Id:               "0",
				Filename:         filename,
				UploadedFilename: uploadFilename,
			}},
		},
	}
	commandPayload, err = json.Marshal(payload)

	return
}

func (bot *DiscordBot) switchFastMode(fast bool) (status int, interactionId string) {
	commandPayload, err := bot.buildModeSwitchPayload(fast)
	if err != nil {
		logger.Errorf("buildModeSwitchPayload error: %s", err)
		return 500, ""
	}
	c := DiscordCommandFast
	if !fast {
		c = DiscordCommandRelax
	}
	return bot.executeSlashCommand(c, commandPayload)
}

func (bot *DiscordBot) imagine(taskId, prompt string) (status int, interactionId string) {
	commandPayload, err := bot.buildImaginePayload(taskId, prompt)
	if err != nil {
		logger.Errorf("buildImaginePayload error: %s", err)
		return 500, ""
	}
	return bot.executeSlashCommand(DiscordCommandImagine, commandPayload)
}

func (bot *DiscordBot) upscale(originImageId, index, messageId string) (status int) {
	commandPayload, err := bot.buildUpscalePayload(originImageId, index, messageId)
	if err != nil {
		logger.Errorf("buildUpscalePayload error: %s", err)
		return 500
	}
	return bot.executeInteractionCommand(commandPayload)
}

func (bot *DiscordBot) describe(filename, uploadFilename string) (status int, interactionId string) {
	commandPayload, err := bot.describeRequest(filename, uploadFilename)
	if err != nil {
		logger.Errorf("describeRequest error: %s", err)
		return 500, ""
	}
	return bot.executeSlashCommand(DiscordCommandDescribe, commandPayload)
}
