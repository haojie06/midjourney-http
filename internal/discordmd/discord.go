package discordmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
)

var (
	MidJourneyServiceApp *MidJourneyService
	ErrTooManyTasks      = fmt.Errorf("too many tasks")
)

func init() {
	MidJourneyServiceApp = &MidJourneyService{
		taskChan: make(chan *imageGenerationTask, 1),

		taskResultChannels: make(map[string]chan *ImageGenerationResult),

		commands: make(map[string]*discordgo.ApplicationCommand),

		rwLock: sync.RWMutex{},
	}
}

type MidJourneyService struct {
	config MidJourneyServiceConfig

	discordSession *discordgo.Session

	taskChan chan *imageGenerationTask

	taskResultChannels map[string]chan *ImageGenerationResult

	commands map[string]*discordgo.ApplicationCommand

	rwLock sync.RWMutex
}

// imagine a image
func (m *MidJourneyService) Imagine(prompt string, params string) (taskId string, taskResultChannel chan *ImageGenerationResult, err error) {
	m.rwLock.Lock()
	defer m.rwLock.Unlock()

	taskId = uuid.New().String()
	taskResultChannel = make(chan *ImageGenerationResult, 10)
	m.taskResultChannels[taskId] = taskResultChannel
	if len(m.taskChan) == cap(m.taskChan) {
		err = ErrTooManyTasks
		return
	}
	m.taskChan <- &imageGenerationTask{
		taskId: taskId,
		prompt: prompt,
		params: params,
	}
	return
}

func (m *MidJourneyService) Start(c MidJourneyServiceConfig) {
	m.config = c
	ds, err := discordgo.New(c.DiscordToken)
	if err != nil {
		panic(err)
	}
	m.discordSession = ds
	commands, err := m.discordSession.ApplicationCommands(c.DiscordAppId, "")
	if err != nil {
		panic(err)
	}
	for _, command := range commands {
		m.commands[command.Name] = command
	}

	m.discordSession.AddHandler(m.onDiscordMessage)
	m.discordSession.Identify.Intents = discordgo.IntentsAll
	err = m.discordSession.Open()
	if err != nil {
		panic(err)
	}
	// reveive task and imagine
	for {
		task := <-m.taskChan
		// avoid discord 429
		time.Sleep(3 * time.Second)
		statusCode := m.imagineRequest(task.taskId, task.prompt, task.params)
		if statusCode >= 400 {
			log.Printf("imagine task %s failed, status code: %d\n", task.taskId, statusCode)
			m.rwLock.Lock()
			if c, exist := m.taskResultChannels[task.taskId]; exist {
				c <- &ImageGenerationResult{
					TaskId:   task.taskId,
					ImageURL: "",
				}
				delete(m.taskResultChannels, task.taskId)
			}
			m.rwLock.Unlock()
		}
	}
}

// when receive message from discord
func (m *MidJourneyService) onDiscordMessage(s *discordgo.Session, message *discordgo.MessageCreate) {
	if len(message.Embeds) > 0 {
		for _, embed := range message.Embeds {
			if embed.Title == "Banned prompt" {
				taskId := getTaskIdFromBannedPrompt(embed.Footer.Text)
				log.Printf("banned prompt occoured in task: %s\n", taskId)
				log.Printf("desc: %s\n", embed.Description)
				m.rwLock.Lock()
				defer m.rwLock.Unlock()
				if c, exist := m.taskResultChannels[taskId]; exist {
					c <- &ImageGenerationResult{
						TaskId:   taskId,
						ImageURL: "",
					}
					delete(m.taskResultChannels, taskId)
				}
				return
			}
		}
	}

	for _, attachment := range message.Attachments {
		if message.ReferencedMessage == nil {
			fileId, taskId := getIdFromURL(attachment.URL)
			if taskId != "" {
				m.upscaleRequest(fileId, 1, message.ID)
			}
		} else {
			// receive upscaling image
			_, taskId := getIdFromURL(attachment.URL)
			if taskId != "" {
				m.rwLock.Lock()
				defer m.rwLock.Unlock()
				if c, exist := m.taskResultChannels[taskId]; exist {
					c <- &ImageGenerationResult{
						TaskId:   taskId,
						ImageURL: attachment.URL,
					}
					delete(m.taskResultChannels, taskId)
				}
			}
		}
	}
}

func (m *MidJourneyService) imagineRequest(taskId string, prompt string, params string) int {
	imagineCommand, exists := m.commands["imagine"]
	if !exists {
		log.Println("Imagine command not found")
		return 500
	}
	prompt = taskId + " " + strings.Trim(prompt, " ") + " " + params
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

func (m *MidJourneyService) upscaleRequest(id string, index int, messageId string) {
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
	m.sendRequest(payload)
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

func getIdFromURL(url string) (fileId, taskId string) {
	tempStrs := strings.Split(url, ".")
	if len(tempStrs) < 2 {
		return "", ""
	}
	tempStr := tempStrs[len(tempStrs)-2]
	tempStrs = strings.Split(tempStr, "_")
	if len(tempStrs) < 2 {
		return "", ""
	}

	if isUUIDString(tempStrs[1]) {
		taskId = tempStrs[1]
	} else {
		taskId = ""
	}
	if isUUIDString(tempStrs[len(tempStrs)-1]) {
		fileId = tempStrs[len(tempStrs)-1]
	} else {
		fileId = ""
	}
	return
}

func getTaskIdFromBannedPrompt(text string) string {
	tempStrs := strings.Split(text, " ")
	if len(tempStrs) < 2 {
		return ""
	}
	return tempStrs[1]
}

func isUUIDString(id string) bool {
	_, err := uuid.Parse(id)
	return err == nil
}
