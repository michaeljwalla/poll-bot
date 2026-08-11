package loadchannel

import (
	"fmt"
	"poll-bot/root/info/unix"
	"poll-bot/root/managers/authorize"
	"poll-bot/root/managers/components"
	"poll-bot/root/types"

	"github.com/bwmarrin/discordgo"
)

type CommandInfo = types.CommandInfo
type EventCallback = types.EventCallback

var metadata = types.CommandMetadata{
	MinTrustLevel: authorize.MANAGE,
}

const FETCHING_TIMEOUT_S = 30
const INTERACTION_IDLE_TIMEOUT_S = 90

func buttons(loadBegin string, stop string, id []string, disabled []bool) *[]discordgo.MessageComponent {
	return &[]discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    loadBegin,
					Style:    discordgo.PrimaryButton,
					CustomID: id[0],
					Disabled: disabled[0],
				},
				discordgo.Button{
					Label:    stop,
					Style:    discordgo.SecondaryButton,
					CustomID: id[1],
					Disabled: disabled[1],
				},
			},
		},
	}
}

func timestamp(i string) uint64 {
	return uint64(unix.IdToEpochMillis(i) / 1000)
}
func expired(a uint64, b uint64, expiryDiff int) bool {
	return a+uint64(expiryDiff) < b
}
func canceledResponse(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	str := "-# This interaction has expired or been canceled."
	emptyComponents := make([]discordgo.MessageComponent, 0)
	_, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content:    &str,
		Components: &emptyComponents,
	})
	return err
}
func genInteractionExpireMessage(i *discordgo.InteractionCreate) (timeout string, expiry string) {
	timestamp := timestamp(i.ID)
	expiry = fmt.Sprintf(`interaction expires in <t:%d:R>`, timestamp+INTERACTION_IDLE_TIMEOUT_S)
	timeout = fmt.Sprintf(`timeout <t:%d:R>`, timestamp+FETCHING_TIMEOUT_S)
	return
}
func genContinueMessage(i *discordgo.InteractionCreate) (message *string) {
	timeout, expiry := genInteractionExpireMessage(i)
	str := fmt.Sprintf(`### fetching polls... %s
-# -> %s
-# -> if you're on mobile, the timestamps update on-relaunch`,
		timeout, expiry)
	return &str
}
func genStartupMessage(i *discordgo.InteractionCreate) (message *string) {
	_, expiry := genInteractionExpireMessage(i)

	str := fmt.Sprintf(`### To begin fetching polls, please press %s.
The oldest fetched poll will be linked for reference how far back has been indexed.
-# -> %s`, "`Start`", expiry)
	return &str
}
func genSuccessQueryMessage(i *discordgo.InteractionCreate, num_polls int, num_messages int, msg *discordgo.Message) (message *string) {
	_, expiry := genInteractionExpireMessage(i)
	msg_link := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", i.GuildID, msg.ChannelID, msg.ID)

	str := fmt.Sprintf(`### Fetched %d messages and found %d polls.
Oldest message %s from <t:%d:f>
-# -> %s`, num_messages, num_polls,
		msg_link, timestamp(msg.ID),
		expiry)
	return &str
}

// map is already reference-like but just for continuity
func Register(bcp *types.BotCommandPackage) {

	bcp.Components.AddGroup("load", &components.GroupMetadata{
		NewInvalidatesOld:  true,
		InvalidationCloses: false,
		FromHandle:         "load",
	})

	(*bcp.Handles)["load"] = CommandInfo{
		DGInfo: &discordgo.ApplicationCommand{
			Name:        "load",
			Description: "Discover rating polls in this channel. Works in increments.",
		},
		Metadata: metadata,
		Callback: func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
			if err := createInteractionSession(bcp, s, i); err != nil {
				return err
			}
			return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content:    *genStartupMessage(i),
					Components: *buttons("Start", "Stop", list("load-continue", "load-stop"), list(false, false)),
				},
			})
		},
	}

}
