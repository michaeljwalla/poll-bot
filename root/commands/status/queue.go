package status

import (
	"fmt"
	"poll-bot/root/managers/polls"
	"poll-bot/root/types"

	"github.com/bwmarrin/discordgo"
)

func view_queue(bcp *types.BotCommandPackage) (top *polls.PollPair, count int) {
	peek, ok := bcp.Polls.Peek()
	if !ok {
		return nil, 0
	}
	return &peek, bcp.Polls.Len()
}

func cmd_view_queue(s session, i intxn, bcp bcpackage) error {
	top, count := view_queue(bcp)

	var message string
	if count == 0 {
		message = `The queue is empty.
-# *polls which haven't closed yet will appear here to be processed.`
	} else {
		message = fmt.Sprintf(`### Next: %v
#### Total queued: %d
-# *only the top is meaningful in the queue heap.
-# *polls which haven't closed yet will appear here to be processed.`, top, count)
	}
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
		},
	})
}
