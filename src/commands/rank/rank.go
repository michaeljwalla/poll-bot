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
func Register(handles *map[string]CommandInfo, auth *authorize.AuthorizedTable) {
	(*handles)["rank"] = CommandInfo{
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

			var message string
			if uSelect, ok := optionMap["user"]; ok {
				user := uSelect.UserValue(s)
				rank := authorize.GetRank(user.ID, *auth)
				message = fmt.Sprintf("`%s`'s rank level is `%02d`", user.GlobalName, rank)
			} else {
				uid := i.Member.User.ID
				rank := authorize.GetRank(uid, *auth)
				message = fmt.Sprintf("Your rank level is `%02d`", rank)
			}
			//
			return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: message,
				},
			})
		},
	}

}
