package discordmd

import (
	"io"

	"github.com/bwmarrin/discordgo"
)

func (m *MidJourneyService) uploadImage(channel, name string, fileReader io.Reader) (message *discordgo.Message, err error) {
	message, err = m.discordSession.ChannelFileSend(channel, name, fileReader)
	// m.discordSession.Attach
	return
}
