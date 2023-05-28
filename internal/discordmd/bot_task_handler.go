package discordmd

import (
	"encoding/json"
	"fmt"
	"mime/multipart"
)

// taskHandler 只负责请求的发起，并不负责获取结果，因为在discord内，所有的interaction都为异步执行的

func (bot *DiscordBot) ImagineTaskHandler(taskId string, payload json.RawMessage) {
	bot.runtimesLock.Lock()
	defer bot.runtimesLock.Unlock()
	taskRuntime, exist := bot.taskRuntimes[taskId]
	if !exist {
		eMessage := fmt.Sprintf("cannot find task runtime for task: %s", taskId)
		bot.logger.Errorf(eMessage)
		return
	}

	var taskPayload ImageGenerationTaskPayload
	if err := json.Unmarshal(payload, &taskPayload); err != nil {
		eMessage := fmt.Sprintf("task %s failed to unmarshal payload: %s", taskId, err.Error())
		taskRuntime.Response(false, eMessage, nil)
		bot.logger.Errorf(eMessage)
		return
	}

	if taskPayload.FastMode {
		bot.switchFastMode(true)
		defer bot.switchFastMode(false)
	}

	interactionId, statusCode, err := bot.imagine(taskId, taskPayload.Prompt)
	if err != nil {
		eMessage := fmt.Sprintf("imagine task %s failed to request, error occured: %s", taskId, err.Error())
		taskRuntime.Response(false, eMessage, nil)
		bot.logger.Errorf(eMessage)
		return
	}

	if statusCode >= 400 {
		eMessage := fmt.Sprintf("imagine task %s failed to request, status code: %d", taskId, statusCode)
		taskRuntime.Response(false, eMessage, nil)
		bot.logger.Warnf(eMessage)
		return
	}
	taskRuntime.InteractionId = interactionId
	bot.logger.Infof("imagine task %s is starting, fast: %t, autoUpscale: %t, prompt: %s", taskId, taskPayload.FastMode, taskPayload.AutoUpscale, taskPayload.Prompt)
	// 创建任务成功时，不需要返回结果，当前结果在eventHandler中才返回
}

func (bot *DiscordBot) UpscaleTaskHandler(taskId string, payload json.RawMessage) {
	bot.runtimesLock.Lock()
	defer bot.runtimesLock.Unlock()
	taskRuntime, exist := bot.taskRuntimes[taskId]
	if !exist {
		eMessage := fmt.Sprintf("cannot find task runtime for task: %s", taskId)
		bot.logger.Errorf(eMessage)
		return
	}

	var taskPayload ImageUpscaleTaskPayload
	if err := json.Unmarshal(payload, &taskPayload); err != nil {
		eMessage := fmt.Sprintf("task %s failed to unmarshal payload: %s", taskId, err.Error())
		taskRuntime.Response(false, eMessage, nil)
		bot.logger.Errorf(eMessage)
		return
	}
	status, err := bot.upscale(taskPayload.OriginImageId, taskPayload.Index, taskPayload.OriginImageMessageId)
	if err != nil {
		eMessage := fmt.Sprintf("task %s failed to request, error occured: %s", taskId, err.Error())
		taskRuntime.Response(false, eMessage, nil)
		bot.logger.Errorf(eMessage)
		return
	}
	if status >= 400 {
		eMessage := fmt.Sprintf("task %s failed to request, status code: %d", taskId, status)
		taskRuntime.Response(false, eMessage, nil)
		bot.logger.Warnf(eMessage)
	}
	bot.logger.Infof("upscale task %s is starting, originImageId: %s, index: %d", taskId, taskPayload.OriginImageId, taskPayload.Index)
}

func (bot *DiscordBot) DescribeTaskHandler(taskId string, payload json.RawMessage) {
	bot.runtimesLock.Lock()
	defer bot.runtimesLock.Unlock()
	taskRuntime, exist := bot.taskRuntimes[taskId]
	if !exist {
		bot.logger.Errorf("cannot find task runtime for task: %s", taskId)
		return
	}

	var taskPayload ImageDescribeTaskPayload
	if err := json.Unmarshal(payload, &taskPayload); err != nil {
		eMessage := fmt.Sprintf("task %s failed to unmarshal payload: %s", taskId, err.Error())
		taskRuntime.Response(false, eMessage, nil)
		bot.logger.Errorf(eMessage)
		return
	}

	fileHeaderI, exist := bot.FileHeaders.LoadAndDelete(taskId)
	if !exist {
		eMessage := fmt.Sprintf("task %s failed to get image file header", taskId)
		taskRuntime.Response(false, eMessage, nil)
		bot.logger.Errorf(eMessage)
		return
	}

	fileHeader, ok := fileHeaderI.(*multipart.FileHeader)
	if !ok {
		eMessage := fmt.Sprintf("task %s failed to assert image file header", taskId)
		taskRuntime.Response(false, eMessage, nil)
		bot.logger.Errorf(eMessage)
		return
	}

	fileReader, err := fileHeader.Open()
	if err != nil {
		eMessage := fmt.Sprintf("task %s failed to open image file: %s", taskId, err.Error())
		taskRuntime.Response(false, eMessage, nil)
		return
	}
	defer fileReader.Close()

	// 先将文件上传为 discord attachment, 稍后引用
	uploadFilename, err := bot.uploadImageToAttachment(taskPayload.ImageFileName, "0", taskPayload.ImageFileSize, fileReader)
	if err != nil {
		eMessage := fmt.Sprintf("task %s failed to upload image file: %s", taskId, err.Error())
		taskRuntime.Response(false, eMessage, nil)
		bot.logger.Errorf(eMessage)
		return
	}

	interactionid, status, err := bot.describe(taskPayload.ImageFileName, uploadFilename)
	if err != nil {
		eMessage := fmt.Sprintf("task %s failed to request, error occured: %s", taskId, err.Error())
		taskRuntime.Response(false, eMessage, nil)
		return
	}
	if status >= 400 {
		eMessage := fmt.Sprintf("task %s failed to request, status code: %d", taskId, status)
		taskRuntime.Response(false, eMessage, nil)
		bot.logger.Warnf(eMessage)
		return
	}

	taskRuntime.InteractionId = interactionid
	bot.logger.Infof("describe task %s is starting, imageFileName: %s", taskId, taskPayload.ImageFileName)
}
