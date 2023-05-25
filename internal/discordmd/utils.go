package discordmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func (bot *DiscordBot) uploadImageToAttachment(fileName string, attachmentId string, fileSize int, file io.Reader) (uploadFileName string, err error) {
	attachmentAPI := fmt.Sprintf("https://discord.com/api/v9/channels/%s/attachments", bot.config.DiscordChannelId)
	attachmentRequest := AttachmentRequest{
		Files: []AttachmentFile{
			{
				FileName: fileName,
				FileSize: fileSize,
				Id:       attachmentId,
			},
		},
	}
	requestBody, _ := json.Marshal(attachmentRequest)
	request, _ := http.NewRequest("POST", attachmentAPI, bytes.NewBuffer(requestBody))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", bot.config.DiscordToken)
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	var attachmentResponse AttachmentResponse
	if err = json.NewDecoder(resp.Body).Decode(&attachmentResponse); err != nil {
		return
	}
	if len(attachmentResponse.Attachments) == 0 {
		err = fmt.Errorf("no attachments found")
		return
	}
	// upload file to google storage
	attacment := attachmentResponse.Attachments[0]
	uploadFileName = attacment.UploadFilename
	request, err = http.NewRequest("PUT", attacment.UploadURL, file)
	if err != nil {
		return
	}
	request.Header.Set("Content-Type", "image/jpeg")
	request.Header.Set("Authority", "discord-attachments-uploads-prd.storage.googleapis.com")
	resp, err = http.DefaultClient.Do(request)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	return
}
