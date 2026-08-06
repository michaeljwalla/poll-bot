package alias

import (
	"errors"
	"fmt"
	"poll-bot/src/authorize"
	"poll-bot/src/types"
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"
)

type CommandInfo = types.CommandInfo
type EventCallback = types.EventCallback

var metadata = types.CommandMetadata{
	MinTrustLevel: authorize.MANAGER,
}

func failedFormatting(s string) bool {
	matched, _ := regexp.MatchString(`[^a-zA-Z0-9\.\-_ ]`, s)
	return matched
}

// map is already reference-like but just for continuity
func Register(bcp *types.BotCommandPackage) {
	table := bcp.Aliases
	(*bcp.Handles)["alias"] = CommandInfo{
		DGInfo: &discordgo.ApplicationCommand{
			Name:        "alias",
			Description: "modify user rank",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user",
					Description: "Who to update",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "nickname",
					Description: "nickname (stored internally)",
					Required:    true,
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

			//
			var user *discordgo.User
			if uSelect, ok := optionMap["user"]; ok {
				user = uSelect.UserValue(s)
			} else {
				return errors.New("couldn't get user field to set")
			}
			if user == nil {
				return errors.New("couldn't fetch user (nil)")
			}
			var newAlias string
			if rSelect, ok := optionMap["nickname"]; ok {
				newAlias = strings.TrimSpace(rSelect.StringValue())
			}

			var message string
			if failedFormatting(newAlias) {
				message = "Alias should only be alphanumerics and/or whitespace . - _"
			} else {
				table.SetAlias(user.ID, newAlias)
				message = fmt.Sprintf("Set `%s`'s nickname to %s", user.GlobalName, newAlias)
			}
			if err := table.Write(); err != nil {
				return err
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
