package discordmd

import (
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/haojie06/midjourney-http/internal/logger"
)

var (
	MidJourneyServiceApp               *MidJourneyService
	ErrTooManyTasks                    = fmt.Errorf("too many tasks")
	FailedEmbededMessageTitlesInCreate = map[string]struct{}{
		"Blocked":                            {},
		"Banned prompt":                      {},
		"Invalid parameter":                  {},
		"Banned prompt detected":             {},
		"Invalid link":                       {},
		"Sorry! Could not complete the job!": {},
		"Action needed to continue":          {},
		"Queue full":                         {},
		"Action required to continue":        {},
	}
	FailedEmbededMessageTitlesInUpdate = map[string]struct{}{
		"Request cancelled due to image filters": {},
	}
)

func init() {
	MidJourneyServiceApp = &MidJourneyService{
		taskChan: make(chan *imageGenerationTask, 1),

		taskRuntimes: make(map[string]*TaskRuntime),

		discordCommands: make(map[string]*discordgo.ApplicationCommand),

		rwLock: sync.RWMutex{},

		randGenerator: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

type MidJourneyService struct {
	config MidJourneyServiceConfig

	discordSession *discordgo.Session

	taskChan chan *imageGenerationTask

	taskRuntimes map[string]*TaskRuntime

	discordCommands map[string]*discordgo.ApplicationCommand

	rwLock sync.RWMutex

	randGenerator *rand.Rand
}

// imagine a image (create a task)
func (m *MidJourneyService) Imagine(prompt, params string, fastMode bool) (taskId string, taskResultChannel chan *ImageGenerationResult, err error) {
	m.rwLock.Lock()
	defer m.rwLock.Unlock()
	if len(m.taskRuntimes) > m.config.MaxUnfinishedTasks {
		err = ErrTooManyTasks
		return
	}
	seed := strconv.Itoa(m.randGenerator.Intn(math.MaxUint32))
	params += " --seed " + seed
	// remove extra spaces
	prompt = strings.Join(strings.Fields(strings.Trim(strings.Trim(prompt, " ")+" "+params, " ")), " ")
	// midjourney will replace — to --, so we need to replace it before hash sum
	prompt = strings.ReplaceAll(prompt, "—", "--")
	// use hash for taskId
	taskId = getHashFromPrompt(prompt, seed)
	logger.Infof("task %s is starting, prompt: %s", taskId, prompt)
	taskResultChannel = make(chan *ImageGenerationResult, m.config.MaxUnfinishedTasks)
	m.taskRuntimes[taskId] = &TaskRuntime{
		TaskId:            taskId,
		ResultChannel:     taskResultChannel,
		UpscaledImageURLs: make([]string, 0),
		CreatedAt:         time.Now().Unix(),
	}
	// send task
	m.taskChan <- &imageGenerationTask{
		taskId:   taskId,
		prompt:   prompt,
		fastMode: fastMode,
	}
	return
}

// remove task when timeout, no mutex lock
func (m *MidJourneyService) RemoveTaskRuntime(taskId string) {
	if r, exist := m.taskRuntimes[taskId]; exist {
		close(r.ResultChannel)
		delete(m.taskRuntimes, taskId)
	}
}

func (m *MidJourneyService) getTaskRuntimeByOriginMessageId(messageId string) *TaskRuntime {
	for _, taskRuntime := range m.taskRuntimes {
		if taskRuntime.OriginImageMessageId == messageId {
			return taskRuntime
		}
	}
	return nil
}

func (m *MidJourneyService) Start(c MidJourneyServiceConfig) {
	m.config = c
	ds, err := discordgo.New(c.DiscordToken)
	if err != nil {
		logger.SugaredZapLogger.Panic(err)
	}
	m.discordSession = ds

	commands, err := m.discordSession.ApplicationCommands(c.DiscordAppId, "")
	if err != nil {
		logger.SugaredZapLogger.Panic(err)
	}
	for _, command := range commands {
		m.discordCommands[command.Name] = command
	}

	m.discordSession.AddHandler(m.onDiscordMessageCreate)
	m.discordSession.AddHandler(m.onDiscordMessageUpdate)
	m.discordSession.Identify.Intents = discordgo.IntentsAll
	err = m.discordSession.Open()
	if err != nil {
		logger.SugaredZapLogger.Panic(err)
	}

	// reveive task and imagine
	for {
		task := <-m.taskChan
		// to avoid discord 429
		time.Sleep(2 * time.Second)
		// send discord command(/imagine) request to imagine a image
		if task.fastMode {
			if status := m.switchMode(true); status >= 400 {
				logger.Warnf("switch mode to fast failed, status code: %d", status)
			}
			time.Sleep(time.Duration((m.randGenerator.Intn(1000))+1000) * time.Millisecond)
		}
		statusCode := m.imagineRequest(task.taskId, task.prompt)
		if statusCode >= 400 {
			logger.Warnf("task %s failed, status code: %d", task.taskId, statusCode)
			m.rwLock.Lock()
			if taskRuntime, exist := m.taskRuntimes[task.taskId]; exist {
				taskRuntime.ResultChannel <- &ImageGenerationResult{
					TaskId:     task.taskId,
					Successful: false,
					Message:    fmt.Sprintf("imagine task: %s failed, code: %d", task.taskId, statusCode),
					ImageURLs:  []string{},
				}
				m.RemoveTaskRuntime(task.taskId)
			}
			m.rwLock.Unlock()
		}
		time.Sleep(time.Duration((m.randGenerator.Intn(1000))+1000) * time.Millisecond)
		// switch back to slow mode
		if task.fastMode {
			if status := m.switchMode(false); status >= 400 {
				logger.Warnf("switch mode back to slow failed, status code: %d", status)
			}
			time.Sleep(time.Duration((m.randGenerator.Intn(1000))+1000) * time.Millisecond)
		}
	}
}

// when discord message updated (for example, when a request is intercepted by a filter)
func (m *MidJourneyService) onDiscordMessageUpdate(s *discordgo.Session, event *discordgo.MessageUpdate) {
	for _, embed := range event.Message.Embeds {
		if _, failed := FailedEmbededMessageTitlesInUpdate[embed.Title]; failed {
			taskId, _ := getHashFromMessage(event.Message.Content)
			m.rwLock.Lock()
			defer m.rwLock.Unlock()
			if taskRuntime, exist := m.taskRuntimes[taskId]; exist {
				logger.Infof("task %s failed, reason: %s descripiton: %s", taskId, embed.Title, embed.Description)
				taskRuntime.ResultChannel <- &ImageGenerationResult{
					TaskId:     taskId,
					Successful: false,
					Message:    embed.Title + " " + embed.Description,
					ImageURLs:  []string{},
				}
				m.RemoveTaskRuntime(taskId)
			}
		}
	}
}

// when receive message from discord(image generated, upscaled, etc.)
func (m *MidJourneyService) onDiscordMessageCreate(s *discordgo.Session, event *discordgo.MessageCreate) {
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
				m.rwLock.Lock()
				defer m.rwLock.Unlock()
				if taskRuntime, exist := m.taskRuntimes[taskId]; exist {
					logger.Infof("task %s failed, reason: %s descripiton: %s", taskId, embed.Title, embed.Description)
					taskRuntime.ResultChannel <- &ImageGenerationResult{
						TaskId:     taskId,
						Successful: false,
						Message:    embed.Title + " " + embed.Description,
						ImageURLs:  []string{},
					}
					m.RemoveTaskRuntime(taskId)
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
	m.rwLock.Lock()
	defer m.rwLock.Unlock()
	// upscaled message has attachments
	for _, attachment := range event.Attachments {
		if event.ReferencedMessage == nil {
			// receive origin image, send upscale request depends on config
			taskId, promptStr := getHashFromMessage(event.Content)
			logger.Infof("task %s receive origin image: %s", taskId, attachment.URL)
			fileId := getFileIdFromURL(attachment.URL)
			taskRuntime, exist := m.taskRuntimes[taskId]
			if taskId != "" && exist && taskRuntime != nil {
				// we will use messageId to map upscaled image to origin image
				taskRuntime.OriginImageMessageId = event.ID
				taskRuntime.OriginImageURL = attachment.URL
				// if m.config.UpscaleCount == 0 {
				// 	// only return origin image and message id
				// }
				// send upscale request depends on config
				for i := 1; i <= m.config.UpscaleCount; i++ {
					if code := m.upscaleRequest(fileId, i, event.ID); code >= 400 {
						logger.Errorf("failed to upscale image, code: %d", code)
					} else {
						logger.Infof("task %s request to upscale image %s %d", taskId, fileId, i)
					}
					time.Sleep(time.Duration((m.randGenerator.Intn(3000))+1000) * time.Millisecond)
				}
			} else {
				logger.Warnf("task %s is not created by this bot, prompt: %s", taskId, promptStr)
			}
		} else {
			// receive upscaling image, use referenced message id to map to taskId
			// when queue is full, we will also receive a message which refer to origin message
			taskRuntime := m.getTaskRuntimeByOriginMessageId(event.ReferencedMessage.ID)
			if taskRuntime == nil {
				logger.Warnf("no local task found for referenced message: %s", event.ReferencedMessage.ID) // non-local task result
				return
			}
			logger.Infof("task %s receives upscaled image: %s", taskRuntime.TaskId, attachment.URL)
			taskRuntime.UpscaledImageURLs = append(taskRuntime.UpscaledImageURLs, attachment.URL)
			if len(taskRuntime.UpscaledImageURLs) == m.config.UpscaleCount {
				logger.Infof("task %s image generation finished, current goroutine count: %d", taskRuntime.TaskId, runtime.NumGoroutine())
				taskRuntime.ResultChannel <- &ImageGenerationResult{
					TaskId:         taskRuntime.TaskId,
					Successful:     true,
					ImageURLs:      taskRuntime.UpscaledImageURLs,
					OriginImageURL: taskRuntime.OriginImageURL,
				}
				m.RemoveTaskRuntime(taskRuntime.TaskId)
			} else {
				logger.Infof("task %s image generation not finished, current images count: %d/%d", taskRuntime.TaskId, len(taskRuntime.UpscaledImageURLs), m.config.UpscaleCount)
			}
		}
	}
}
