package authorize

import (
	fd "poll-bot/root/datas/filedict"
	"strconv"

	"github.com/bwmarrin/discordgo"
)

type AuthManager struct {
	data *fd.FileDict[string, Rank]
}

func validate(uid *string, rank *Rank) bool {
	_, err := strconv.Atoi(*uid)
	return err == nil
}
func New(path string) (*AuthManager, error) {
	table, err := fd.New(path, validate)
	if err != nil {
		return nil, err
	}
	return &AuthManager{
		data: table,
	}, nil
}

func (table *AuthManager) CanUse(uid string, rank Rank) bool {
	return table.GetRank(uid) >= rank
}

func (table *AuthManager) GetRank(uid string) Rank {
	userRank, ok := table.data.Get(uid)
	if !ok {
		userRank = DEFAULT
	}
	return userRank
}

func (table *AuthManager) SetRank(uid string, rank Rank) error {
	return table.data.Set(uid, rank)
}

func (table *AuthManager) Write() error { return table.data.SyncWrite() }
func (table *AuthManager) Read() error  { return table.data.SyncRead() }

func PermissionsErrorIntercept(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Sorry, you can't use that command.",
		},
	})
}
func (table *AuthManager) Close() error {
	return table.data.Close()
}
