package authorize

import (
	"encoding/json"
	"os"

	"github.com/bwmarrin/discordgo"
)

type AuthorizedTable map[string]Rank

var AuthTable = AuthorizedTable{}
var path string

func SetGlobalPath(toPath string) {
	path = toPath
}

func CanUse(uid string, rank Rank, auth AuthorizedTable) bool {
	return GetRank(uid, auth) >= rank
}

func GetRank(uid string, auth AuthorizedTable) Rank {
	userRank, ok := auth[uid]
	if !ok {
		userRank = DEFAULT
	}
	return userRank
}

// maps are referential
func SetRank(uid string, rank Rank, auth AuthorizedTable) {
	auth[uid] = rank
}

func SetFile(auth AuthorizedTable) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close() //nolint
	return json.NewEncoder(file).Encode(auth)
}

func PermissionsErrorIntercept(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Sorry, you can't use that command.",
		},
	})
}
