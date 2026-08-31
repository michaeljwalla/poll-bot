package polls

import (
	"errors"

	"github.com/bwmarrin/discordgo"
)

func (man *PollManager) GetData(data *Poll) (*discordgo.Message, error) {
	if s := man.session.Load(); s != nil {
		return data.LiveMessage(CACHEDFETCH, s)
	}
	return nil, errors.New("no session assigned to PollManager")
}
