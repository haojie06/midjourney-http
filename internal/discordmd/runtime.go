package discordmd

import "time"

type TaskRuntime struct {
	TaskId string

	TaskKeywordHash string

	InteractionId string

	UpscaleResultChannels map[string]chan *ImageUpscaleResultPayload

	OriginImageURL string

	OriginImageId string

	OriginImageMessageId string

	UpscaledImageURLs []string

	UpscaleProcessCount int

	AutoUpscale bool

	CreatedAt int64

	UpdatedAt int64

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
		CreatedAt:             time.Now().Unix(),
		UpdatedAt:             time.Now().Unix(),
		State:                 TaskStateCreated,
	}
}
