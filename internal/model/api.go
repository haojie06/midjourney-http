package model

type WebhookConfig struct {
	URL string `json:"url"`
}

type GenerationTaskRequest struct {
	Prompt string `json:"prompt"`

	Params string `json:"params"`

	ReportType string `json:"report_type"`

	FastMode bool `json:"fast_mode"`

	AutoUpscale bool `json:"auto_upscale"`

	WebhookConfig WebhookConfig `json:"webhook_config"`
}

type GenerationTaskResponse struct {
	TaskId string `json:"task_id"`

	Status string `json:"status"` // pending, running, completed, failed

	Message string `json:"message"`

	ImageURLs []string `json:"image_urls"`

	OriginImageURL string `json:"origin_image_url"`
}

type UpscaleTaskResponse struct {
	TaskId string `json:"task_id"`

	Status string `json:"status"`

	Message string `json:"message"`

	ImageURL string `json:"image_url"`

	Index string `json:"index"`
}
