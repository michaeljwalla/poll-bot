package loadchannel

import (
	"poll-bot/root/managers/authorize"
	"poll-bot/root/types"

	"github.com/bwmarrin/discordgo"
)

type CommandInfo = types.CommandInfo
type EventCallback = types.EventCallback

var metadata = types.CommandMetadata{
	MinTrustLevel: authorize.MANAGE,
}

// map is already reference-like but just for continuity
func Register(bcp *types.BotCommandPackage) {
	(*bcp.Handles)["load"] = CommandInfo{
		DGInfo: &discordgo.ApplicationCommand{
			Name:        "load",
			Description: "Discover rating polls in this channel. Works in increments.",
		},
		Metadata: metadata,
		Callback: func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
			// channel := i.ChannelID

			return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Okay",
					Components: []discordgo.MessageComponent{
						discordgo.ActionsRow{
							Components: []discordgo.MessageComponent{
								discordgo.Button{
									Label:    "Begin",
									Style:    discordgo.PrimaryButton,
									CustomID: "load-begin",
								},
								discordgo.Button{
									Label:    "Stop",
									Style:    discordgo.SecondaryButton,
									CustomID: "load-stop",
									Disabled: true,
								},
							},
						},
					},
				},
			})
		},
	}

}
