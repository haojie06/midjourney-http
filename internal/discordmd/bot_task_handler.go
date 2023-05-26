package discordmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"

	"github.com/haojie06/midjourney-http/internal/logger"
)

// taskHandler 只负责请求的发起，并不负责获取结果，因为在discord内，所有的interaction都为异步执行的

func (bot *DiscordBot) ImagineTaskHandler(taskId string, payload json.RawMessage) (err error) {
	var taskPayload ImageGenerationTaskPayload
	if err = json.Unmarshal(payload, &taskPayload); err != nil {
		return
	}
	bot.runtimesLock.Lock()
	defer bot.runtimesLock.Unlock()
	taskRuntime, exist := bot.taskRuntimes[taskId]
	if !exist {
		err = ErrTaskRuntimeNotFound
		return
	}

	logger.Infof("imagine task %s received, fast mode: %t, prompt: %s", taskId, taskPayload.FastMode, taskPayload.Prompt)
	if taskPayload.FastMode {
		bot.switchFastMode(true)
		defer bot.switchFastMode(false)
	}
	statusCode, interactionId := bot.imagine(taskId, taskPayload.Prompt)
	taskRuntime.InteractionId = interactionId

	if statusCode >= 400 {
		logger.Warnf("task %s failed, status code: %d", taskId, statusCode)
		taskRuntime.taskResultChan <- TaskResult{
			TaskId:     taskId,
			Successful: false,
			Message:    fmt.Sprintf("imagine task: %s failed, code: %d", taskId, statusCode),
		}
		return
	}
	logger.Debugf("imagine task %s interaction created id: %s", taskId, interactionId)
	return
}

func (bot *DiscordBot) UpscaleTaskHandler(taskId string, payload json.RawMessage) (err error) {
	var taskPayload ImageUpscaleTaskPayload
	if err = json.Unmarshal(payload, &taskPayload); err != nil {
		return
	}
	if statusCode := bot.upscale(taskPayload.OriginImageId, taskPayload.Index, taskPayload.OriginImageMessageId); statusCode >= 400 {
		logger.Warnf("task %s failed, status code: %d", taskId, statusCode)
		bot.runtimesLock.Lock()
		defer bot.runtimesLock.Unlock()
		if taskRuntime, exist := bot.taskRuntimes[taskId]; exist {
			taskRuntime.taskResultChan <- TaskResult{
				TaskId:     taskId,
				Successful: false,
				Message:    fmt.Sprintf("imagine task: %s failed, code: %d", taskId, statusCode),
			}
		}
	}
	return
}

func (bot *DiscordBot) DescribeTaskHandler(taskId string, payload json.RawMessage) (err error) {
	var taskPayload ImageDescribeTaskPayload
	if err = json.Unmarshal(payload, &taskPayload); err != nil {
		return
	}
	bot.runtimesLock.Lock()
	defer bot.runtimesLock.Unlock()
	taskRuntime, exist := bot.taskRuntimes[taskId]
	if !exist {
		err = ErrTaskRuntimeNotFound
		return
	}
	fileHeaderI, exist := bot.FileHeaders.LoadAndDelete(taskId)
	if !exist {
		err = errors.New("failed to get image file header")
		return
	}
	fileHeader, ok := fileHeaderI.(*multipart.FileHeader)
	if !ok {
		err = errors.New("failed to get image file header")
		return
	}
	fileReader, err := fileHeader.Open()
	if err != nil {
		return
	}
	defer fileReader.Close()
	uploadFilename, err := bot.uploadImageToAttachment(taskPayload.ImageFileName, "0", taskPayload.ImageFileSize, fileReader)
	if err != nil {
		return
	}
	code, interactionid := bot.describe(taskPayload.ImageFileName, uploadFilename)
	taskRuntime.InteractionId = interactionid
	if code >= 400 {
		taskRuntime.taskResultChan <- TaskResult{
			TaskId:     taskId,
			Successful: false,
			Message:    fmt.Sprintf("imagine task: %s failed, code: %d", taskId, code),
		}
		return
	}
	logger.Debugf("describe task %s interaction created id: %s", taskId, interactionid)
	return
}
