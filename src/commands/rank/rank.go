package rank

import (
	"fmt"
	"poll-bot/src/authorize"
	"poll-bot/src/types"

	"github.com/bwmarrin/discordgo"
)

type CommandInfo = types.CommandInfo
type EventCallback = types.EventCallback

var metadata = types.CommandMetadata{
	MinTrustLevel: authorize.DEFAULT,
}

// map is already reference-like but just for continuity
func Register(bcp *types.BotCommandPackage) {
	table := bcp.Auth
	(*bcp.Handles)["rank"] = CommandInfo{
		DGInfo: &discordgo.ApplicationCommand{
			Name:        "rank",
			Description: "check the rank of a user",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user",
					Description: "Defaults to yourself.",
					Required:    false,
				},
			},
		},
		Metadata: metadata,
		Callback: func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
			options := i.ApplicationCommandData().Options
			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
			for _, opt := range options {
				optionMap[opt.Name] = opt
			}

			var user *discordgo.User
			//
			if target, ok := optionMap["user"]; ok {
				user = target.UserValue(s)
			} else {
				user = i.Member.User
			}

			rank := table.GetRank(user.ID)
			//
			return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("**%s**'s rank level is `%02d/%s`", user.GlobalName, rank, authorize.Stringify(rank)),
				},
			})
		},
	}

}
