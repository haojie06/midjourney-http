package discordmd

import "github.com/bwmarrin/discordgo"

type MidJourneyServiceConfig struct {
	DiscordToken string `mapstructure:"discordToken"`

	DiscordAppId string `mapstructure:"discordAppId"` // midjourney application id

	DiscordChannelId string `mapstructure:"discordChannelId"` // midjourney channel id

	DiscordSessionId string `mapstructure:"discordSessionId"` // midjourney session id

	UpscaleCount int `mapstructure:"upscaleCount"`

	MaxUnfinishedTasks int `mapstructure:"maxUnfinishedTasks"`
}

type InteractionRequestWrapper struct {
	PayloadJSON InteractionRequest `json:"payload_json"`
}

type InteractionRequest struct {
	Type          int                    `json:"type"`
	ApplicationID string                 `json:"application_id"`
	ChannelID     string                 `json:"channel_id"`
	SessionID     string                 `json:"session_id"`
	Data          InteractionRequestData `json:"data"`
	Nonce         string                 `json:"nonce"`
}

type UpSampleData struct {
	ComponentType int    `json:"component_type"`
	CustomID      string `json:"custom_id"`
}

type InteractionRequestTypeThree struct {
	Type          int         `json:"type"`
	ChannelID     string      `json:"channel_id"`
	MessageFlags  int         `json:"message_flags"`
	MessageID     string      `json:"message_id"`
	ApplicationID string      `json:"application_id"`
	SessionID     string      `json:"session_id"`
	Data          interface{} `json:"data"`
}

type InteractionRequestData struct {
	Version            string                                               `json:"version"`
	ID                 string                                               `json:"id"`
	Name               string                                               `json:"name"`
	Type               int                                                  `json:"type"`
	Options            []*discordgo.ApplicationCommandInteractionDataOption `json:"options"`
	ApplicationCommand *discordgo.ApplicationCommand                        `json:"application_command"`
	Attachments        []interface{}                                        `json:"attachments"`
}

type InteractionRequestApplicationCommand struct {
	ID                       string      `json:"id"`
	ApplicationID            string      `json:"application_id"`
	Version                  string      `json:"version"`
	DefaultMemberPermissions interface{} `json:"default_member_permissions"`
	Type                     int         `json:"type"`
	Nsfw                     bool        `json:"nsfw"`
	Name                     string      `json:"name"`
	Description              string      `json:"description"`
	DmPermission             bool        `json:"dm_permission"`
}

type imageGenerationTask struct {
	taskId string

	prompt string

	fastMode bool

	autoUpscale bool
}

type ImageGenerationResult struct {
	TaskId string `json:"task_id"`

	Successful bool `json:"successful"`

	Message string `json:"message"`

	OriginImageURL string `json:"origin_image_url"`

	ImageURLs []string `json:"image_urls"`
}

type ImageUpscaleResult struct {
	TaskId string `json:"task_id"`

	Successful bool `json:"successful"`

	Message string `json:"message"`

	Index string `json:"index"`

	ImageURL string `json:"image_url"`
}

type TaskState string

const (
	TaskStateCreated         TaskState = "created"
	TaskStateGetOriginImage  TaskState = "get_origin_image"
	TaskStateAutoUpscaling   TaskState = "auto_upscaling"
	TaskStateManualUpscaling TaskState = "manual_upscaling"
)

type TaskRuntime struct {
	TaskId string

	ResultChannel chan *ImageGenerationResult

	UpscaleResultChannels map[string]chan *ImageUpscaleResult

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
