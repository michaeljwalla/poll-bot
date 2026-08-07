package modrank

import (
	"errors"
	"fmt"
	"poll-bot/src/authorize"
	"poll-bot/src/types"

	"github.com/bwmarrin/discordgo"
)

type CommandInfo = types.CommandInfo
type EventCallback = types.EventCallback

var metadata = types.CommandMetadata{
	MinTrustLevel: authorize.PROMOTER,
}

var rankChoices = make([]*discordgo.ApplicationCommandOptionChoice, authorize.NUM_RANKS)

func init() {
	for i := range authorize.NUM_RANKS {
		rankChoices[i] = &discordgo.ApplicationCommandOptionChoice{
			Name:  authorize.Stringify(i),
			Value: i,
		}
	}
}

// map is already reference-like but just for continuity
func Register(bcp *types.BotCommandPackage) {
	table := bcp.Auth
	(*bcp.Handles)["modrank"] = CommandInfo{
		DGInfo: &discordgo.ApplicationCommand{
			Name:        "modrank",
			Description: "modify user rank",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionUser,
					Name:        "user",
					Description: "Who to update",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "rank",
					Description: "rank to set (cannot be >= your own)",
					Required:    true,
					Choices:     rankChoices,
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

			// init param data (target, reqRank)
			var target *discordgo.User
			if uSelect, ok := optionMap["user"]; ok {
				target = uSelect.UserValue(s)
			} else {
				return errors.New("couldn't get user field to set")
			}
			if target == nil {
				return errors.New("couldn't fetch user (nil)")
			}
			var reqRank authorize.Rank
			if rSelect, ok := optionMap["rank"]; ok {
				reqRank = rSelect.IntValue()
			}
			reqRankStr := authorize.Stringify(reqRank)

			//validate request then apply
			var message string
			senderRank := table.GetRank(i.Member.User.ID)
			if reqRankStr == "UNKNOWN" {
				message = "I don't know what rank that is."
			} else if reqRank >= senderRank || table.GetRank(target.ID) >= senderRank {
				message = "You can't rank someone higher than or equal to yourself."
			} else {
				if err := table.SetRank(target.ID, reqRank); err != nil {
					return err
				}
				if err := table.Write(); err != nil {
					return err
				}
				message = fmt.Sprintf("Set `%s`'s rank to %s", target.Username, reqRankStr)
			}

			// response
			return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: message,
				},
			})
		},
	}

}
