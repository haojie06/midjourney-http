// bot 对外提供的各方法, 调用后返回一个稍后会被填充的 channel, 用于接收结果
package discordmd

import (
	"encoding/json"
	"math"
	"mime/multipart"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// imagine a image (create a task)
func (m *MidJourneyService) Imagine(prompt, params string, fastMode, autoUpscale bool) (taskId string, taskResultChan chan TaskResult, err error) {
	// allocate taskId from prompt
	taskId = uuid.New().String()

	seed := strconv.Itoa(m.randGenerator.Intn(math.MaxUint32))
	params += " --seed " + seed
	// remove extra spaces
	prompt = strings.Join(strings.Fields(strings.Trim(strings.Trim(prompt, " ")+" "+params, " ")), " ")
	// midjourney will replace — to --, so we need to replace it before hash sum
	prompt = strings.ReplaceAll(prompt, "—", "--")
	// use hash for taskId
	taskKeywordHash := getHashFromPrompt(prompt, seed)

	bot, err := m.GetBot(taskId)
	if err != nil {
		return
	}

	bot.runtimesLock.Lock()
	defer bot.runtimesLock.Unlock()

	taskResultChan = make(chan TaskResult, 1)
	bot.taskRuntimes[taskId] = &TaskRuntime{
		TaskId:                taskId,
		TaskKeywordHash:       taskKeywordHash, // 部分交互的回复，不引用interaction, 因此需要通过关键词来关联
		UpscaleResultChannels: make(map[string]chan *ImageUpscaleResultPayload),
		UpscaledImageURLs:     make([]string, 0),
		taskResultChan:        taskResultChan,
		AutoUpscale:           autoUpscale,
		State:                 TaskStateCreated,
	}
	// TODO 改为不需要marshal
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
// upscale 基于已有的 图片生成任务进行，所以需要传入 taskId 和 index
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

func (m *MidJourneyService) Describe(file *multipart.FileHeader, filename string, size int) (taskId string, taskResultChan chan TaskResult, err error) {
	taskId = uuid.New().String()
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
