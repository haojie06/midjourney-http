package model

// 请求部分
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

type UpscaleTaskRequest struct {
	TaskId string `json:"task_id"`

	Index string `json:"index"`
}

// 响应部分
type TaskHTTPResponse struct {
	TaskId string `json:"task_id"`

	Status string `json:"status"` // pending, created, completed, failed

	Message string `json:"message"`

	Payload interface{} `json:"payload"`
}

type GenerationTaskResponsePayload struct {
	ImageURLs []string `json:"image_urls"`

	OriginImageURL string `json:"origin_image_url"`
}

type UpscaleTaskResponsePayload struct {
	ImageURL string `json:"image_url"`

	Index string `json:"index"`
}

type DescribeTaskResponsePayload struct {
	Description string `json:"description"`
}
