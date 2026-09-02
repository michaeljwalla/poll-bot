package rate

import (
	"fmt"
	"poll-bot/root/managers/authorize"
	"poll-bot/root/managers/polls"
	"poll-bot/root/types"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

type CommandInfo = types.CommandInfo
type EventCallback = types.EventCallback

var metadata = types.CommandMetadata{
	MinTrustLevel: authorize.DEFAULT,
}

var DEFAULT_RATING_ANSWERS []string = []string{
	"N/A",
	repStr("⭐", 1),
	repStr("⭐", 2),
	repStr("⭐", 3),
	repStr("⭐", 4),
	repStr("⭐", 5),
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
	for i := 5; i >= 0; i-- {
		answer := &answers[5-i]
		answer.Media = &discordgo.PollMedia{Text: DEFAULT_RATING_ANSWERS[i]}
		answer.AnswerID = i
		comment, ok := comment_map["c"+strconv.Itoa(i)]
		if !ok {
			continue
		}
		answer.Media.Text += fmt.Sprintf(" (%s)", comment.StringValue())
	}
	return &answers
}
func Register(bcp *types.BotCommandPackage) {
	(*bcp.Handles)["rate"] = CommandInfo{
		DGInfo: &discordgo.ApplicationCommand{
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
		Metadata: metadata,
		Callback: func(s *discordgo.Session, i *discordgo.InteractionCreate) error {
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

			err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Poll: &poll,
					// Flags: discordgo.MessageFlagsEphemeral,
				},
			})
			if err != nil {
				return err
			}

			msg, err := s.InteractionResponse(i.Interaction)
			if err != nil {
				return err
			}
			_, err = bcp.Polls.Push(*polls.From(msg, i.GuildID))
			return err
		},
	}
}
