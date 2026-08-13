package status

import "github.com/bwmarrin/discordgo"

func cmd_check_writer(s session, i intxn, bcp bcpackage) error {
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "The worker doesnt even exist yet!",
		},
	})
}
