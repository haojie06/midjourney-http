package discordmd

// imagine and describe will create a new TaskRuntime while upscale will reuse.
type TaskRuntime struct {
	TaskId string

	TaskKeywordHash string

	InteractionId string // Some command responses will reference the interaction ID that created the command, so we need to keep track of it and use it to find the corresponding TaskRuntime later.

	UpscaleResultChannels map[string]chan *ImageUpscaleResultPayload

	OriginImageURL string

	OriginImageId string

	OriginImageMessageId string

	UpscaledImageURLs []string

	UpscaleProcessCount int

	AutoUpscale bool

	State TaskState

	taskResultChan chan TaskResult
}

func NewTaskRuntime(taskId string, autoUpscale bool) *TaskRuntime {
	return &TaskRuntime{
		TaskId:                taskId,
		TaskKeywordHash:       "", // eg: prompt hash
		UpscaleResultChannels: make(map[string]chan *ImageUpscaleResultPayload),
		UpscaledImageURLs:     make([]string, 0),
		taskResultChan:        make(chan TaskResult, 1),
		AutoUpscale:           autoUpscale,
		State:                 TaskStateCreated,
	}
}

func (r *TaskRuntime) Response(successful bool, message string, payload interface{}) {
	r.taskResultChan <- TaskResult{
		TaskId:     r.TaskId,
		Successful: successful,
		Message:    message,
		Payload:    payload,
	}
}
