package discordmd

import "time"

type TaskRuntime struct {
	TaskId string

	InteractionId string

	ImagineResultChannel chan *ImageGenerationResult

	UpscaleResultChannels map[string]chan *ImageUpscaleResult

	DescribeResultChannel chan *DescribeResult

	OriginImageURL string

	OriginImageId string

	OriginImageMessageId string

	UpscaledImageURLs []string

	UpscaleProcessCount int

	AutoUpscale bool

	CreatedAt int64

	UpdatedAt int64

	State TaskState
}

func NewTaskRuntime(taskId string, autoUpscale bool) *TaskRuntime {
	return &TaskRuntime{
		TaskId:                taskId,
		ImagineResultChannel:  make(chan *ImageGenerationResult),
		DescribeResultChannel: make(chan *DescribeResult),
		UpscaleResultChannels: make(map[string]chan *ImageUpscaleResult),
		UpscaledImageURLs:     make([]string, 0),
		AutoUpscale:           autoUpscale,
		CreatedAt:             time.Now().Unix(),
		UpdatedAt:             time.Now().Unix(),
		State:                 TaskStateCreated,
	}
}

// remove task when timeout, no mutex lock
func (m *MidJourneyService) RemoveTaskRuntime(taskId string) {
	if r, exist := m.taskRuntimes[taskId]; exist {
		close(r.ImagineResultChannel)
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

func (m *MidJourneyService) getTaskRuntimeByInteractionId(interactionId string) *TaskRuntime {
	if interactionId == "" {
		return nil
	}
	for _, taskRuntime := range m.taskRuntimes {
		if taskRuntime.InteractionId == interactionId {
			return taskRuntime
		}
	}
	return nil
}
