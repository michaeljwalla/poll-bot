package commands

import (
	"fmt"
	"poll-bot/src/unix"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

type Commands struct {
	Identifiers []*discordgo.ApplicationCommand
	Handlers    map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate)
}

var DEFAULT_RATING_ANSWERS []string = []string{
	repStr("⭐", 5),
	repStr("⭐", 4),
	repStr("⭐", 3),
	repStr("⭐", 2),
	repStr("⭐", 1),
	"N/A",
}

// Define your slash commands configuration
var commands = []*discordgo.ApplicationCommand{
	{
		Name:        "ping",
		Description: "Responds with a pong message",
	},
	{
		Name:        "rate",
		Description: "Make a rating poll",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "topic",
				Description: "title of the poll",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "c5",
				Description: fmt.Sprintf("%s (your_comment_goes_here)", DEFAULT_RATING_ANSWERS[0]),
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "c4",
				Description: fmt.Sprintf("%s (your_comment_goes_here)", DEFAULT_RATING_ANSWERS[1]),
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "c3",
				Description: fmt.Sprintf("%s (your_comment_goes_here)", DEFAULT_RATING_ANSWERS[2]),
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "c2",
				Description: fmt.Sprintf("%s (your_comment_goes_here)", DEFAULT_RATING_ANSWERS[3]),
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "c1",
				Description: fmt.Sprintf("%s (your_comment_goes_here)", DEFAULT_RATING_ANSWERS[4]),
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "c0",
				Description: fmt.Sprintf("%s (your_comment_goes_here)", DEFAULT_RATING_ANSWERS[5]),
				Required:    false,
			},
		},
	},
}

func repStr(s string, n int) string {
	builder := strings.Builder{}
	builder.Grow(len(s) * n)
	for range n {
		builder.WriteString(s)
	}
	return builder.String()
}

func get_rate_answers(comment_map map[string]*discordgo.ApplicationCommandInteractionDataOption) *[]discordgo.PollAnswer {
	answers := make([]discordgo.PollAnswer, len(DEFAULT_RATING_ANSWERS))
	for i := range 6 {
		answer := &answers[5-i]
		answer.Media = &discordgo.PollMedia{Text: DEFAULT_RATING_ANSWERS[5-i]}

		comment, ok := comment_map["c"+strconv.Itoa(i)]
		if !ok {
			continue
		}
		answer.Media.Text += fmt.Sprintf(" (%s)", comment.StringValue())
	}
	return &answers
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
	"rate": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		options := i.ApplicationCommandData().Options
		optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
		for _, opt := range options {
			optionMap[opt.Name] = opt
		}

		topicOption := optionMap["topic"]
		poll := discordgo.Poll{
			Question: discordgo.PollMedia{Text: topicOption.StringValue()},
			Answers:  *get_rate_answers(optionMap),
		}
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Poll: &poll,
				// Flags: discordgo.MessageFlagsEphemeral,
			},
		})
	},
}

var MainCommands = &Commands{Identifiers: commands, Handlers: commandHandlers}
