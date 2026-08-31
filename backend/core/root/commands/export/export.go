package export

import (
	"fmt"
	"os"
	"poll-bot/root/csv"
	"poll-bot/root/managers/authorize"
	"poll-bot/root/managers/polls"
	"poll-bot/root/types"
	"time"

	"github.com/bwmarrin/discordgo"
)

type CommandInfo = types.CommandInfo
type EventCallback = types.EventCallback

var metadata = types.CommandMetadata{
	MinTrustLevel: authorize.DEFAULT,
}

// map is already reference-like but just for continuity
func Register(bcp *types.BotCommandPackage) {
	(*bcp.Handles)["export"] = CommandInfo{
		DGInfo: &discordgo.ApplicationCommand{
			Name:        "export",
			Description: "Export captured (/load) polls to a csv. Does not export open polls.",
		},
		Metadata: metadata,
		Callback: func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
			if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "I'm generating a file now.",
				},
			}); err != nil {
				return err
			}

			records, err := bcp.Polls.GetFinalized(0, 0, true)
			if err != nil {
				msg := fmt.Sprintf("I couldn't access the DB: %v", err)
				s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{ //nolint
					Content: &msg,
				})
				return err
			}
			path := bcp.Polls.Path() + polls.EXPORT_SUBPATH
			_, err = csv.ToCSV(path, records, bcp.Aliases)
			if err != nil {
				msg := fmt.Sprintf("I couldn't generate the file: %v", err)
				s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{ //nolint
					Content: &msg,
				})
				return err
			}
			file, err := os.Open(path)
			if err != nil {
				msg := fmt.Sprintf("I couldn't open the file, but I generated it: %v", err)
				s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{ //nolint
					Content: &msg,
				})
				return err
			}
			msg := fmt.Sprintf("Done <@%v>", i.Member.User.ID)
			_, err = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &msg,
				Files: []*discordgo.File{
					{
						Name:        fmt.Sprintf("Results %s %s.csv", time.Now().Format("01_02_2006"), i.GuildID),
						ContentType: "text/csv",
						Reader:      file,
					},
				},
			})
			return err
		},
	}

}
