package discordmd

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/haojie06/midjourney-http/internal/logger"
)

// imagine a image (create a task)
func (m *MidJourneyService) Imagine(prompt, params string, fastMode, autoUpscale bool) (taskId string, taskResultChannel chan *ImageGenerationResult, err error) {
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
		TaskId:                taskId,
		ResultChannel:         taskResultChannel,
		UpscaleResultChannels: make(map[string]chan *ImageUpscaleResult),
		UpscaledImageURLs:     make([]string, 0),
		AutoUpscale:           autoUpscale,
		CreatedAt:             time.Now().Unix(),
		UpdatedAt:             time.Now().Unix(),
		State:                 TaskStateCreated,
	}
	// send task
	m.taskChan <- &imageGenerationTask{
		taskId:      taskId,
		prompt:      prompt,
		fastMode:    fastMode,
		autoUpscale: autoUpscale,
	}
	return
}

// Upscale a image with given taskId and index
func (m *MidJourneyService) Upscale(taskId, index string) (upscaleResultChannel chan *ImageUpscaleResult, err error) {
	// find the task runtime, and get the result channel
	m.rwLock.Lock()
	defer m.rwLock.Unlock()
	taskRuntime, exist := m.taskRuntimes[taskId]
	if !exist {
		err = ErrTaskNotFound
		return
	}
	taskRuntime.State = TaskStateManualUpscaling
	upscaleResultChannel = make(chan *ImageUpscaleResult)
	taskRuntime.UpscaleResultChannels[index] = upscaleResultChannel
	if code := m.upscaleRequest(taskRuntime.OriginImageId, index, taskRuntime.OriginImageMessageId); code >= 400 {
		err = ErrFailedToCreateTask
		return
	}
	return
}

func (m *MidJourneyService) Describe(r io.Reader, filename string, size int) (description string, err error) {
	status := m.describeRequest(filename, size, r)
	if status >= 400 {
		description = fmt.Sprintf("Failed to describe image, status code: %d", status)
		err = ErrFailedToDescribeImage
	}
	return
}
