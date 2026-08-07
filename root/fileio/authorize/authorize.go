package authorize

import (
	fd "poll-bot/root/datas/filedict"

	"github.com/bwmarrin/discordgo"
)

type AuthTable struct {
	data *fd.FileDict[string, Rank]
}

func New(path string) (*AuthTable, error) {
	table, err := fd.New[string, Rank](path)
	if err != nil {
		return nil, err
	}
	return &AuthTable{
		data: table,
	}, nil
}

func (table *AuthTable) CanUse(uid string, rank Rank) bool {
	return table.GetRank(uid) >= rank
}

func (table *AuthTable) GetRank(uid string) Rank {
	userRank, ok := table.data.Get(uid)
	if !ok {
		userRank = DEFAULT
	}
	return userRank
}

func (table *AuthTable) SetRank(uid string, rank Rank) error {
	return table.data.Set(uid, rank)
}

func (table *AuthTable) Write() error { return table.data.SyncWrite() }
func (table *AuthTable) Read() error  { return table.data.SyncRead() }

func PermissionsErrorIntercept(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Sorry, you can't use that command.",
		},
	})
}
