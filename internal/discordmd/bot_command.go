// bot - discord 内部的命令实现, 主要为 interaction 请求的发送实现
package discordmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/bwmarrin/discordgo"
)

type DiscordCommand string

const (
	DiscordCommandFast     DiscordCommand = "fast"
	DiscordCommandRelax    DiscordCommand = "relax"
	DiscordCommandImagine  DiscordCommand = "imagine"
	DiscordCommandUpscale  DiscordCommand = "upscale"
	DiscordCommandDescribe DiscordCommand = "describe"
)

func (bot *DiscordBot) sendInteractionRequest(payload []byte) (status int, err error) {
	request, err := http.NewRequest("POST", "https://discord.com/api/v9/interactions", bytes.NewBuffer(payload))
	if err != nil {
		return 500, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", bot.config.DiscordToken)

	resposne, err := http.DefaultClient.Do(request)
	if err != nil {
		return 500, err
	}
	defer resposne.Body.Close()
	return resposne.StatusCode, nil
}
func checkCommandResponse(commandType DiscordCommand, slashCommandResponse SlashCommandResponse) bool {
	return slashCommandResponse.Name == string(commandType)
}

// 部分指令(目前除了upscale)，在发送执行请求后，需要阻塞等待，拿到interactionId
func (bot *DiscordBot) executeSlashCommand(commandType DiscordCommand, commandPayload []byte) (interactionId string, status int, err error) {
	// 通过 sync.cond 拿到执行结果
	status, err = bot.sendInteractionRequest(commandPayload)
	if err != nil {
		return "", status, err
	}
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

// MessageComponent交互 包括 upscale variant等，不需要等待响应拿到id
func (bot *DiscordBot) executeMessageComponent(commandPayload []byte) (status int, err error) {
	status, err = bot.sendInteractionRequest(commandPayload)
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

// 调用 discord /v9/interaction 接口, 执行 slash command 或者是 message component 点击等交互
func (bot *DiscordBot) switchFastMode(fast bool) (interactionId string, status int, err error) {
	commandPayload, err := bot.buildModeSwitchPayload(fast)
	if err != nil {
		return "", 500, err
	}
	c := DiscordCommandFast
	if !fast {
		c = DiscordCommandRelax
	}
	interactionId, status, err = bot.executeSlashCommand(c, commandPayload)
	return
}

func (bot *DiscordBot) imagine(taskId, prompt string) (interactionId string, status int, err error) {
	commandPayload, err := bot.buildImaginePayload(taskId, prompt)
	if err != nil {
		return "", 500, err
	}
	interactionId, status, err = bot.executeSlashCommand(DiscordCommandImagine, commandPayload)
	return
}

func (bot *DiscordBot) describe(filename, uploadFilename string) (interactionId string, status int, err error) {
	commandPayload, err := bot.describeRequest(filename, uploadFilename)
	if err != nil {
		return "", 500, err
	}
	interactionId, status, err = bot.executeSlashCommand(DiscordCommandDescribe, commandPayload)
	return
}

// upscale 的交互为 MessageComponent 和 SlashCommand 不同
func (bot *DiscordBot) upscale(originImageId, index, messageId string) (status int, err error) {
	commandPayload, err := bot.buildUpscalePayload(originImageId, index, messageId)
	if err != nil {
		return 500, err
	}
	status, err = bot.executeMessageComponent(commandPayload)
	return
}
