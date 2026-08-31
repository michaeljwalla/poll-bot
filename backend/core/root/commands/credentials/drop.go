package credentials

import "github.com/bwmarrin/discordgo"

func cmd_drop_credentials(s session, i intxn, bcp bcpackage, web *webman) error {
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "TODO",
		},
	})
}
