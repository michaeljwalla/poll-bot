package authorize

import (
	"iter"
	fd "poll-bot/root/datas/filedict"
	"strconv"

	"github.com/bwmarrin/discordgo"
)

type uid = string
type Rank = int64

type AuthManager struct {
	data *fd.FileDict[uid, Rank]
}

func validate(id *uid, rank *Rank) bool {
	_, err := strconv.Atoi(*id)
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

func (table *AuthManager) CanUse(id uid, rank Rank) bool {
	return table.GetRank(id) >= rank
}

func (table *AuthManager) GetRank(id uid) Rank {
	userRank, ok := table.data.Get(id)
	if !ok {
		userRank = DEFAULT
	}
	return userRank
}

func (table *AuthManager) SetRank(id uid, rank Rank) error {
	return table.data.Set(id, rank)
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

func (table *AuthManager) Iter() iter.Seq2[uid, Rank] {
	return table.data.Iter()
}
