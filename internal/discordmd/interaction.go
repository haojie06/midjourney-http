package discordmd

import (
	"encoding/json"
	"math"
	"mime/multipart"
	"strconv"
	"strings"
	"time"

	"github.com/haojie06/midjourney-http/internal/logger"
)

// imagine a image (create a task)
func (m *MidJourneyService) Imagine(prompt, params string, fastMode, autoUpscale bool) (taskId string, imagineResultChannel chan *ImageGenerationResult, err error) {
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
	imagineResultChannel = make(chan *ImageGenerationResult, m.config.MaxUnfinishedTasks)
	m.taskRuntimes[taskId] = &TaskRuntime{
		TaskId:                taskId,
		ImagineResultChannel:  imagineResultChannel,
		UpscaleResultChannels: make(map[string]chan *ImageUpscaleResult),
		DescribeResultChannel: make(chan *DescribeResult),
		UpscaledImageURLs:     make([]string, 0),
		AutoUpscale:           autoUpscale,
		CreatedAt:             time.Now().Unix(),
		UpdatedAt:             time.Now().Unix(),
		State:                 TaskStateCreated,
	}
	payload, _ := json.Marshal(ImageGenerationTaskPayload{
		Prompt:      prompt,
		FastMode:    fastMode,
		AutoUpscale: autoUpscale,
	})
	// send task
	m.taskChan <- &MidjourneyTask{
		TaskId:   taskId,
		TaskType: MidjourneyTaskTypeImageGeneration,
		Payload:  payload,
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
	payload, _ := json.Marshal(ImageUpscaleTaskPayload{
		OriginImageId:        taskRuntime.OriginImageId,
		Index:                index,
		OriginImageMessageId: taskRuntime.OriginImageMessageId,
	})
	m.taskChan <- &MidjourneyTask{
		TaskId:   taskId,
		TaskType: MidjourneyTaskTypeImageUpscale,
		Payload:  payload,
	}
	return
}

func (m *MidJourneyService) Describe(taskId string, file *multipart.FileHeader, filename string, size int) (describeResultChannel chan *DescribeResult, err error) {
	m.rwLock.Lock()
	defer m.rwLock.Unlock()

	taskRuntime := NewTaskRuntime(taskId, false)
	describeResultChannel = taskRuntime.DescribeResultChannel
	m.taskRuntimes[taskId] = taskRuntime
	payload, _ := json.Marshal(ImageDescribeTaskPayload{
		ImageFileName: filename,
		ImageFileSize: size,
	})
	m.taskChan <- &MidjourneyTask{
		TaskId:   taskId,
		TaskType: MidjourneyTaskTypeImageDescribe,
		Payload:  payload,
	}
	return
}
