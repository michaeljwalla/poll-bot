package polls

import (
	"errors"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var ErrMessageNotFound = errors.New("message missing")

func (man *PollManager) GetData(data *Poll) (*discordgo.Message, error) {
	if s := man.session.Load(); s != nil {
		msg, err := data.LiveMessage(CACHEDFETCH, s)
		if err != nil && strings.HasPrefix(err.Error(), "HTTP 404") {
			return nil, ErrMessageNotFound
		}
		return msg, err
	}
	return nil, errors.New("no session assigned to PollManager")
}
