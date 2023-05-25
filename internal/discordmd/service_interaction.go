// bot 对外提供的各方法, 调用后返回一个稍后会被填充的 channel, 用于接收结果
package discordmd

import (
	"encoding/json"
	"math"
	"mime/multipart"
	"strconv"
	"strings"
	"time"
)

// imagine a image (create a task)
func (m *MidJourneyService) Imagine(prompt, params string, fastMode, autoUpscale bool) (taskId string, taskResultChan chan TaskResult, err error) {
	// allocate taskId from prompt
	seed := strconv.Itoa(m.randGenerator.Intn(math.MaxUint32))
	params += " --seed " + seed
	// remove extra spaces
	prompt = strings.Join(strings.Fields(strings.Trim(strings.Trim(prompt, " ")+" "+params, " ")), " ")
	// midjourney will replace — to --, so we need to replace it before hash sum
	prompt = strings.ReplaceAll(prompt, "—", "--")
	// use hash for taskId
	taskId = getHashFromPrompt(prompt, seed)

	bot, err := m.GetBot(taskId)
	if err != nil {
		return
	}

	bot.runtimesLock.Lock()
	defer bot.runtimesLock.Unlock()
	// if len(bot.taskRuntimes) > bot.config.MaxUnfinishedTasks {
	// 	err = ErrTooManyTasks
	// 	return
	// }

	taskResultChan = make(chan TaskResult, 1)
	bot.taskRuntimes[taskId] = &TaskRuntime{
		TaskId: taskId,
		// ImagineResultChannel:  resultNotifyChan,
		UpscaleResultChannels: make(map[string]chan *ImageUpscaleResultPayload),
		UpscaledImageURLs:     make([]string, 0),
		taskResultChan:        taskResultChan,
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
	bot.taskChan <- &MidjourneyTask{
		TaskId:   taskId,
		TaskType: MidjourneyTaskTypeImageGeneration,
		Payload:  payload,
	}
	return
}

// Upscale a image with given taskId and index
func (m *MidJourneyService) Upscale(taskId, index string) (taskResultChan chan TaskResult, err error) {
	bot, err := m.GetBot(taskId)
	if err != nil {
		return
	}
	// find the task runtime, and get the result channel
	bot.runtimesLock.Lock()
	defer bot.runtimesLock.Unlock()
	taskRuntime, exist := bot.taskRuntimes[taskId]
	if !exist {
		err = ErrTaskNotFound
		return
	}
	taskRuntime.State = TaskStateManualUpscaling
	taskResultChan = taskRuntime.taskResultChan

	payload, _ := json.Marshal(ImageUpscaleTaskPayload{
		OriginImageId:        taskRuntime.OriginImageId,
		Index:                index,
		OriginImageMessageId: taskRuntime.OriginImageMessageId,
	})
	bot.taskChan <- &MidjourneyTask{
		TaskId:   taskId,
		TaskType: MidjourneyTaskTypeImageUpscale,
		Payload:  payload,
	}
	return
}

func (m *MidJourneyService) Describe(taskId string, file *multipart.FileHeader, filename string, size int) (taskResultChan chan TaskResult, err error) {
	bot, err := m.GetBot(taskId)
	if err != nil {
		return
	}
	bot.runtimesLock.Lock()
	defer bot.runtimesLock.Unlock()
	bot.FileHeaders.Store(taskId, file)
	taskRuntime := NewTaskRuntime(taskId, false)
	taskResultChan = taskRuntime.taskResultChan
	bot.taskRuntimes[taskId] = taskRuntime
	payload, _ := json.Marshal(ImageDescribeTaskPayload{
		ImageFileName: filename,
		ImageFileSize: size,
	})
	bot.taskChan <- &MidjourneyTask{
		TaskId:   taskId,
		TaskType: MidjourneyTaskTypeImageDescribe,
		Payload:  payload,
	}
	return
}
