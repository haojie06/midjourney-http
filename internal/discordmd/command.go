package discordmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/bwmarrin/discordgo"
)

func (m *MidJourneyService) imagineRequest(taskId string, prompt string) (status int) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered in imagineRequest", r)
			status = 500
		}
	}()
	imagineCommand, exists := m.discordCommands["imagine"]
	if !exists {
		log.Println("Imagine command not found")
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

func (m *MidJourneyService) upscaleRequest(id string, index int, messageId string) int {
	payload := InteractionRequestTypeThree{
		Type:          3,
		MessageFlags:  0,
		MessageID:     messageId,
		ApplicationID: m.config.DiscordAppId,
		ChannelID:     m.config.DiscordChannelId,
		SessionID:     m.config.DiscordSessionId,
		Data: UpSampleData{
			ComponentType: 2,
			CustomID:      fmt.Sprintf("MJ::JOB::upsample::%d::%s", index, id),
		},
	}
	return m.sendRequest(payload)
}

func (m *MidJourneyService) sendRequest(payload interface{}) int {
	requestBody, err := json.Marshal(payload)
	if err != nil {
		log.Println("Error marshalling payload: ", err)
		panic(err)
	}

	request, err := http.NewRequest("POST", "https://discord.com/api/v9/interactions", bytes.NewBuffer(requestBody))
	if err != nil {
		log.Println("Error creating request: ", err)
		panic(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", m.config.DiscordToken)

	client := &http.Client{}
	resposne, err := client.Do(request)
	if err != nil {
		log.Println("Error sending request: ", err)
		panic(err)
	}
	defer resposne.Body.Close()
	return resposne.StatusCode
}
