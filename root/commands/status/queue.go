package status

import (
	"fmt"
	"math"
	"poll-bot/root/datas/set"
	"poll-bot/root/managers/polls"
	"poll-bot/root/types"

	"github.com/bwmarrin/discordgo"
)

type poll = polls.Poll

func view_queue(bcp *types.BotCommandPackage) (messages []*discordgo.Message, poll []polls.Poll, count int, err error) {
	polls, ok := bcp.Polls.GetTopOrdered()
	if !ok {
		return nil, nil, 0, nil
	}

	messages = make([]*discordgo.Message, len(polls))
	for i, poll := range polls {
		msg, err := bcp.Polls.GetData(&poll)
		if err != nil {
			return nil, nil, -1, fmt.Errorf("on Polls.GetData (%v): %v", poll, err)
		}
		messages[i] = msg
	}

	return messages, polls, bcp.Polls.Len(), nil
}

type pollFormatted struct {
	title          string
	num_votes      int
	id_data        poll
	best_choice    []string
	best_num_votes int
	finalized      bool
}

type ErrNoPoll struct {
	error
}

func fromMessage(msg *discordgo.Message, data poll) (formatted *pollFormatted, err error) {
	poll := msg.Poll
	if poll == nil {
		return nil, ErrNoPoll{}
	}

	formatted = &pollFormatted{
		title:       poll.Question.Text,
		id_data:     data,
		finalized:   poll.Results.Finalized,
		best_choice: make([]string, 0, len(poll.Answers)),
	}

	//get highest choice
	choiceIDs := set.New[int]()
	for _, v := range poll.Results.AnswerCounts {
		formatted.num_votes += v.Count
		if v.Count > formatted.best_num_votes {
			formatted.best_num_votes = v.Count
			choiceIDs.Clear()
		}
		if v.Count == formatted.best_num_votes {
			choiceIDs.Insert(v.ID)
		}
	}
	//stringify
	for _, v := range poll.Answers {
		if !choiceIDs.Has(v.AnswerID) {
			continue
		}
		formatted.best_choice = append(formatted.best_choice, v.Media.Text)
	}
	return
}

// with num, denom \in Z^+
func roundPercentFrac(num int, denom int) int {
	return int(math.Round(float64(num) / float64(denom) * 100))
}
func (pf *pollFormatted) Stringify() (msg string, link string) {
	message := fmt.Sprintf("**`%s`**", pf.title)
	//
	if pf.id_data.Expiry != nil {
		if pf.finalized {
			message += fmt.Sprintf(" | closed <t:%d:R>", pf.id_data.Expiry.Unix())
		} else {
			message += fmt.Sprintf(" | until <t:%d>)", pf.id_data.Expiry.Unix()) //why does <t: use unix but everything else use their dumb ahh snowflake
		}
	}
	//
	if pf.num_votes == 0 {
		if pf.finalized {
			message += "\n-# Nobody voted..."
		} else {
			message += "\n-# No votes yet."
		}
	} else {
		message += fmt.Sprintf("\n-# Votes:`%d`\n-# Lead(s):`%v` / %d%%", pf.num_votes, pf.best_choice, roundPercentFrac(pf.best_num_votes, pf.num_votes))
	}
	//

	return message,
		fmt.Sprintf("https://discord.com/channels/%s/%s/%s", pf.id_data.Guild, pf.id_data.Message.ChannelID, pf.id_data.Message.ID)
}
func cmd_view_queue(s session, i intxn, bcp bcpackage) error {
	tops, poll, count, err := view_queue(bcp)

	var message string
	if err != nil {
		return err
	} else if count == 0 {
		message = `The queue is empty.
-# *polls which haven't closed yet will appear here to be processed.`
	} else {
		var pollsPresent string
		for i, msg := range tops {
			formatted, err := fromMessage(msg, poll[i])
			if err != nil {
				return fmt.Errorf("no Poll on %v", msg)
			}
			str, link := formatted.Stringify()
			pollsPresent += fmt.Sprintf("%s %s\n\n", link, str)
		}

		message = fmt.Sprintf(`### Next Up:
%s**Total enqueued: %d**
-# *polls which haven't closed yet will appear here to be processed.`, pollsPresent, count)
	}
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
		},
	})
}
