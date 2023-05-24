package discordmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/bwmarrin/discordgo"
	"github.com/haojie06/midjourney-http/internal/logger"
)

func (m *MidJourneyService) switchMode(fast bool) (status int) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("Recovered in fastRequest %s", r)
			status = 500
		}
	}()
	var commnad *discordgo.ApplicationCommand
	var exists bool
	if fast {
		commnad, exists = m.discordCommands["fast"]
	} else {
		commnad, exists = m.discordCommands["relax"]
	}
	if !exists || commnad == nil {
		logger.Error("Fast/Relax command not found")
		return 500
	}
	payload := InteractionRequest{
		Type:          2,
		ApplicationID: commnad.ApplicationID,
		ChannelID:     m.config.DiscordChannelId,
		SessionID:     m.config.DiscordSessionId,
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
	status = m.sendRequest(payload)
	return
}

func (m *MidJourneyService) imagineRequest(taskId string, prompt string) (status int) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered in imagineRequest", r)
			status = 500
		}
	}()
	imagineCommand, exists := m.discordCommands["imagine"]
	if !exists {
		logger.Error("Imagine command not found")
		return 500
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
		ChannelID:     m.config.DiscordChannelId,
		SessionID:     m.config.DiscordSessionId,
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
	return m.sendRequest(payload)
}

func (m *MidJourneyService) upscaleRequest(id, index, messageId string) int {
	payload := InteractionRequestTypeThree{
		Type:          3,
		MessageFlags:  0,
		MessageID:     messageId,
		ApplicationID: m.config.DiscordAppId,
		ChannelID:     m.config.DiscordChannelId,
		SessionID:     m.config.DiscordSessionId,
		Data: UpSampleData{
			ComponentType: 2,
			CustomID:      fmt.Sprintf("MJ::JOB::upsample::%s::%s", index, id),
		},
	}
	return m.sendRequest(payload)
}

func (m *MidJourneyService) describeRequest(filename string, size int, file io.Reader) int {
	describeCommand, exists := m.discordCommands["describe"]
	if !exists {
		logger.Error("Describe command not found")
		return 500
	}
	// get google api put url
	apiURL := fmt.Sprintf("https://discord.com/api/v9/channels/%s/attachments", m.config.DiscordChannelId)
	attachmentRequest := AttachmentRequest{
		Files: []AttachmentFile{
			{
				FileName: filename,
				FileSize: size,
				Id:       "0",
			},
		},
	}
	requestBody, _ := json.Marshal(attachmentRequest)
	request, _ := http.NewRequest("POST", apiURL, bytes.NewBuffer(requestBody))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", m.config.DiscordToken)
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		logger.Errorf("Error sending request: %s", err.Error())
		return 500
	}
	defer resp.Body.Close()
	var attachmentResponse AttachmentResponse
	if err := json.NewDecoder(resp.Body).Decode(&attachmentResponse); err != nil {
		logger.Errorf("Error decoding response: %s", err.Error())
		return 500
	}
	if len(attachmentResponse.Attachments) == 0 {
		logger.Error("No attachments found")
		return 500
	}
	// upload file to google storage
	attachment := attachmentResponse.Attachments[0]
	request, err = http.NewRequest("PUT", attachment.UploadURL, file)
	if err != nil {
		logger.Errorf("Error creating request: %s", err.Error())
		return 500
	}
	request.Header.Set("Content-Type", "image/jpeg")
	request.Header.Set("Authority", "discord-attachments-uploads-prd.storage.googleapis.com")
	resp, err = http.DefaultClient.Do(request)
	if err != nil {
		logger.Errorf("Error sending request: %s", err.Error())
		return 500
	}
	defer resp.Body.Close()
	logger.Infof("success to put file response: %s\n%+v", resp.Status, attachment)
	var dataOptions []*discordgo.ApplicationCommandInteractionDataOption
	dataOptions = append(dataOptions, &discordgo.ApplicationCommandInteractionDataOption{
		Type:  11,
		Name:  "image",
		Value: 0,
	})
	// send describe command request with attachment
	payload := InteractionRequest{
		Type:          2,
		ApplicationID: describeCommand.ApplicationID,
		ChannelID:     m.config.DiscordChannelId,
		SessionID:     m.config.DiscordSessionId,
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
				UploadedFilename: attachment.UploadFilename,
			}},
		},
	}
	return m.sendRequest(payload)
}

func (m *MidJourneyService) sendRequest(payload interface{}) int {
	requestBody, err := json.Marshal(payload)
	if err != nil {
		logger.Errorf("Error marshalling payload: %s", err.Error())
		return 500
	}

	request, err := http.NewRequest("POST", "https://discord.com/api/v9/interactions", bytes.NewBuffer(requestBody))
	if err != nil {
		logger.Errorf("Error creating request: %s", err.Error())
		return 500
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", m.config.DiscordToken)

	client := &http.Client{}
	resposne, err := client.Do(request)
	if err != nil {
		logger.Errorf("Error sending request: %s", err.Error())
		return 500
	}
	defer resposne.Body.Close()
	return resposne.StatusCode
}
