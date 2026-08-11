package polls

import (
	"errors"

	"github.com/bwmarrin/discordgo"
)

func (man *PollManager) GetData(data *Poll) (*discordgo.Message, error) {
	if s := man.session.Load(); s != nil {
		return s.ChannelMessage(data.Message.ChannelID, data.Message.ID)
	}
	return nil, errors.New("no session assigned to PollManager")
}
