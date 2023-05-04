package discordmd

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
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

		originImageURLMap: make(map[string]string),

		imageURLsMap: make(map[string][]string),

		messageIdToTaskIdMap: make(map[string]string),

		commands: make(map[string]*discordgo.ApplicationCommand),

		rwLock: sync.RWMutex{},
	}
}

type MidJourneyService struct {
	config MidJourneyServiceConfig

	discordSession *discordgo.Session

	taskChan chan *imageGenerationTask

	taskResultChannels map[string]chan *ImageGenerationResult

	originImageURLMap map[string]string

	imageURLsMap map[string][]string

	messageIdToTaskIdMap map[string]string

	commands map[string]*discordgo.ApplicationCommand

	rwLock sync.RWMutex
}

// imagine a image
func (m *MidJourneyService) Imagine(prompt string, params string) (taskId string, taskResultChannel chan *ImageGenerationResult, err error) {
	m.rwLock.Lock()
	defer m.rwLock.Unlock()
	rand.Seed(time.Now().UnixNano())
	seed := strconv.Itoa(rand.Intn(math.MaxUint32))
	params += " --seed " + seed
	prompt = strings.Join(strings.Fields(strings.Trim(strings.Trim(prompt, " ")+" "+params, " ")), " ")
	// midjourney will replace — to --, so we need to replace it back for hash
	prompt = strings.ReplaceAll(prompt, "—", "--")
	taskId = getHashFromPrompt(prompt, seed)
	log.Println("start task:", taskId, "prompt:", prompt)
	taskResultChannel = make(chan *ImageGenerationResult, 30)
	m.taskResultChannels[taskId] = taskResultChannel
	if len(m.taskChan) == cap(m.taskChan) {
		err = ErrTooManyTasks
		return
	}
	m.taskChan <- &imageGenerationTask{
		taskId: taskId,
		prompt: prompt,
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
		// to avoid discord 429
		time.Sleep(3 * time.Second)

		statusCode := m.imagineRequest(task.taskId, task.prompt)
		if statusCode >= 400 {
			log.Printf("imagine task %s failed, status code: %d\n", task.taskId, statusCode)
			m.rwLock.Lock()
			if c, exist := m.taskResultChannels[task.taskId]; exist {
				c <- &ImageGenerationResult{
					TaskId:     task.taskId,
					Successful: false,
					Message:    fmt.Sprintf("imagine task failed, code: %d", statusCode),
					ImageURLs:  []string{},
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
			if embed.Title == "Blocked" || embed.Title == "Banned prompt" || embed.Title == "Invalid parameter" || embed.Title == "Banned prompt detected" || embed.Title == "Invalid link" {
				taskId := getHashFromEmbeds(embed.Footer.Text)
				log.Printf("%s prompt occoured in task: %s\n", embed.Title, taskId)
				log.Printf("desc: %s\n", embed.Description)
				m.rwLock.Lock()
				defer m.rwLock.Unlock()
				if c, exist := m.taskResultChannels[taskId]; exist {
					c <- &ImageGenerationResult{
						TaskId:     taskId,
						Successful: false,
						Message:    embed.Title + " " + embed.Description,
						ImageURLs:  []string{},
					}
					delete(m.taskResultChannels, taskId)
				}
				return
			}
		}
	}

	m.rwLock.Lock()
	defer m.rwLock.Unlock()
	for _, attachment := range message.Attachments {
		if message.ReferencedMessage == nil {
			// receive origin image
			taskId, promptStr := getHashFromMessage(message.Content)
			log.Println("receive origin image:", attachment.URL, "taskId:", taskId)
			fileId := getIdFromURL(attachment.URL)
			if taskId != "" && m.taskResultChannels[taskId] != nil {
				m.messageIdToTaskIdMap[message.ID] = taskId
				m.originImageURLMap[taskId] = attachment.URL
				for i := 1; i <= m.config.UpscaleCount; i++ {
					if code := m.upscaleRequest(fileId, i, message.ID); code >= 400 {
						log.Println("failed to upscale image, code: ", code)
					} else {
						log.Printf("upscale image %s %d\n", fileId, i)
					}
					time.Sleep(time.Duration((rand.Intn(2000))+1000) * time.Millisecond)
				}
			} else {
				log.Println("no task id found for message: ", message.Content, "prompt:", promptStr)
			}
		} else {
			// receive upscaling image
			taskId := m.messageIdToTaskIdMap[message.ReferencedMessage.ID]
			log.Println("receive upscaled image:", attachment.URL, "tasiId:", taskId)
			if taskId == "" {
				log.Println("no task id found for message: ", message.ReferencedMessage.ID)
				return
			}
			if m.imageURLsMap[taskId] == nil {
				log.Println("create image url map for task: ", taskId)
				m.imageURLsMap[taskId] = make([]string, 0)
			}
			m.imageURLsMap[taskId] = append(m.imageURLsMap[taskId], attachment.URL)
			if len(m.imageURLsMap[taskId]) == m.config.UpscaleCount {
				log.Println("image generation finished")
				if c, exist := m.taskResultChannels[taskId]; exist {
					c <- &ImageGenerationResult{
						TaskId:         taskId,
						Successful:     true,
						ImageURLs:      m.imageURLsMap[taskId],
						OriginImageURL: m.originImageURLMap[taskId],
					}
				}
				delete(m.taskResultChannels, taskId)
				delete(m.imageURLsMap, taskId)
				delete(m.originImageURLMap, taskId)
				delete(m.messageIdToTaskIdMap, message.ReferencedMessage.ID)
			} else {
				log.Printf("%s image generation not finished, current count: %d\n", taskId, len(m.imageURLsMap[taskId]))
			}
		}
	}
}

func (m *MidJourneyService) imagineRequest(taskId string, prompt string) int {
	imagineCommand, exists := m.commands["imagine"]
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

func getIdFromURL(url string) (fileId string) {
	tempStrs := strings.Split(url, ".")
	if len(tempStrs) < 2 {
		return ""
	}
	tempStr := tempStrs[len(tempStrs)-2]
	tempStrs = strings.Split(tempStr, "_")
	if len(tempStrs) < 2 {
		return ""
	}

	if isUUIDString(tempStrs[len(tempStrs)-1]) {
		fileId = tempStrs[len(tempStrs)-1]
	} else {
		fileId = ""
	}
	return
}

func isUUIDString(id string) bool {
	_, err := uuid.Parse(id)
	return err == nil
}

func getHashFromMessage(message string) (hashStr, promptStr string) {
	promptRe := regexp.MustCompile(`\*{2}(.+?)\*{2}`)
	linkRe := regexp.MustCompile(`<https?:\/\/\S+\>`)
	seedRe := regexp.MustCompile(`--seed\s+(\d+)`)
	matches := promptRe.FindStringSubmatch(message)
	if len(matches) < 2 {
		return "", ""
	}
	promptStr = strings.Trim(matches[1], " ")
	seedMatchs := seedRe.FindStringSubmatch(message)
	if len(seedMatchs) < 2 {
		return "", ""
	}
	seed := seedMatchs[1]
	promptStr = linkRe.ReplaceAllString(promptStr, seed)
	// print("get hash from message: ", promptStr, "\n")
	h := md5.Sum([]byte(promptStr))
	hashStr = hex.EncodeToString(h[:])
	if len(hashStr) > 32 {
		hashStr = hashStr[:32]
	}
	return
}

func getHashFromPrompt(prompt, seed string) (hashStr string) {
	// replace all image links with seed, because image link will change in response
	linkRe := regexp.MustCompile(`\bhttps?://\S+\b`)
	prompt = linkRe.ReplaceAllString(prompt, seed)
	// print("get hash from prompt: ", prompt, "\n")
	h := md5.Sum([]byte(prompt))
	hashStr = hex.EncodeToString(h[:])
	if len(hashStr) > 32 {
		hashStr = hashStr[:32]
	}
	return
}

func getHashFromEmbeds(message string) (hashStr string) {
	// get seed and replace all links with it
	linkRe := regexp.MustCompile(`<https?:\/\/\S+\>`)
	seedRe := regexp.MustCompile(`--seed\s+(\d+)`)
	matchSeeds := seedRe.FindStringSubmatch(message)
	if len(matchSeeds) < 2 {
		return ""
	}
	seed := matchSeeds[1]
	message = strings.Trim(message, " ")
	messageParts := strings.SplitN(message, " ", 2)
	if len(messageParts) < 2 {
		return ""
	}
	log.Println(messageParts[1])
	message = linkRe.ReplaceAllString(messageParts[1], seed)
	// print("get hash from embeds", message, "\n")
	h := md5.Sum([]byte(message))
	hashStr = hex.EncodeToString(h[:])
	if len(hashStr) > 32 {
		hashStr = hashStr[:32]
	}
	return
}
