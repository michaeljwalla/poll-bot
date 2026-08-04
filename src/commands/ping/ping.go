package ping

import (
	"poll-bot/src/unix"
	"strconv"

	"github.com/bwmarrin/discordgo"
)

// map is already reference-like but just for continuity
func Register(infos *[]*discordgo.ApplicationCommand, callbacks *map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate) error) {
	*infos = append(*infos, &discordgo.ApplicationCommand{
		Name:        "ping",
		Description: "Responds with a pong message",
	})

	(*callbacks)["ping"] = func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
		return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Pong, recv. " + strconv.FormatInt(unix.DiffNowEpochMillis(i.ID), 10) + "ms",
			},
		})
	}
}
