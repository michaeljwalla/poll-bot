package ping

import (
	"poll-bot/src/authorize"
	"poll-bot/src/types"
	"poll-bot/src/unix"
	"strconv"

	"github.com/bwmarrin/discordgo"
)

type CommandInfo = types.CommandInfo
type EventCallback = types.EventCallback

var metadata = types.CommandMetadata{
	MinTrustLevel: authorize.DEFAULT,
}

// map is already reference-like but just for continuity
func Register(bcp *types.BotCommandPackage) {
	(*bcp.Handles)["ping"] = CommandInfo{
		DGInfo: &discordgo.ApplicationCommand{
			Name:        "ping",
			Description: "Responds with a pong message",
		},
		Metadata: metadata,
		Callback: func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
			return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Pong, recv. " + strconv.FormatInt(unix.DiffNowEpochMillis(i.ID), 10) + "ms",
				},
			})
		},
	}

}
