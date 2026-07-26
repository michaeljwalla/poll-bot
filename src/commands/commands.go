package commands

import (
	"poll-bot/src/unix"
	"strconv"

	"github.com/bwmarrin/discordgo"
)

type Commands struct {
	Identifiers []*discordgo.ApplicationCommand
	Handlers    map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate)
}

// Define your slash commands configuration
var commands = []*discordgo.ApplicationCommand{
	{
		Name:        "ping",
		Description: "Responds with a pong message",
	},
}

// Define the execution handlers for yzouor commands
var commandHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
	"ping": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Pong, recv. " + strconv.FormatInt(unix.DiffNowEpochMillis(i.ID), 10) + "ms",
			},
		})
	},
}

var MainCommands = &Commands{Identifiers: commands, Handlers: commandHandlers}
