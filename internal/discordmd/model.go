package discordmd

import (
	"encoding/json"

	"github.com/bwmarrin/discordgo"
)

type MidJourneyServiceConfig struct {
	DiscordToken string `mapstructure:"discordToken"`

	DiscordAppId string `mapstructure:"discordAppId"` // midjourney application id

	DiscordChannelId string `mapstructure:"discordChannelId"` // midjourney channel id

	DiscordSessionId string `mapstructure:"discordSessionId"` // midjourney session id

	DiscordGuildId string `mapstructure:"discordGuildId"` // midjourney guild id

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
	GuildID       string                 `json:"guild_id"`
	Token         string                 `json:"token"`
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
	GuildID       string      `json:"guild_id"`
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

type MidjourneyTaskType string

const (
	MidjourneyTaskTypeImageGeneration MidjourneyTaskType = "image_generation"
	MidjourneyTaskTypeImageUpscale    MidjourneyTaskType = "image_upscale"
	MidjourneyTaskTypeImageDescribe   MidjourneyTaskType = "image_describe"
)

type MidjourneyTask struct {
	TaskId   string
	TaskType MidjourneyTaskType
	Payload  json.RawMessage
}

type ImageGenerationTaskPayload struct {
	Prompt string

	FastMode bool

	AutoUpscale bool
}

type ImageUpscaleTaskPayload struct {
	OriginImageId        string
	Index                string
	OriginImageMessageId string
}

type ImageDescribeTaskPayload struct {
	ImageFileName string
	ImageFileSize int
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

type DescribeResult struct {
	TaskId string `json:"task_id"`

	Successful bool `json:"successful"`

	Message string `json:"message"`

	Description string `json:"description"`
}

type AttachmentRequest struct {
	Files []AttachmentFile `json:"files"`
}

type AttachmentFile struct {
	FileName string `json:"filename"`
	FileSize int    `json:"file_size"`
	Id       string `json:"id"`
}

type AttachmentResponse struct {
	Attachments []AttachmentInResponse `json:"attachments"`
}

type AttachmentInResponse struct {
	Id             int    `json:"id"`
	UploadURL      string `json:"upload_url"`
	UploadFilename string `json:"upload_filename"`
}

type AttachmentInCommand struct {
	Id               string `json:"id"`
	Filename         string `json:"filename"`
	UploadedFilename string `json:"uploaded_filename"`
}

type TaskState string

const (
	TaskStateCreated         TaskState = "created"
	TaskStateGetOriginImage  TaskState = "get_origin_image"
	TaskStateAutoUpscaling   TaskState = "auto_upscaling"
	TaskStateManualUpscaling TaskState = "manual_upscaling"
)
