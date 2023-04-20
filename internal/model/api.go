package model

type WebhookConfig struct {
	URL string `json:"url"`
}

type GenerationTaskRequest struct {
	Prompt string `json:"prompt"`

	Params string `json:"params"`

	ReportType string `json:"report_type"`

	WebhookConfig WebhookConfig `json:"webhook_config"`
}

type GenerationTaskResponse struct {
	TaskId string `json:"task_id"`

	Status string `json:"status"` // pending, running, completed, failed

	ImageURL string `json:"image_url"`
}
