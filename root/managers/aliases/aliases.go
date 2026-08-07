package aliases

import (
	fd "poll-bot/root/datas/filedict"
	"strconv"
)

type AliasManager struct {
	data *fd.FileDict[string, string]
}

func validate(uid *string, name *string) bool {
	_, err := strconv.Atoi(*uid)
	return err == nil
}

func New(path string) (*AliasManager, error) {
	table, err := fd.New(path, validate)
	if err != nil {
		return nil, err
	}
	return &AliasManager{
		data: table,
	}, nil
}
func (table *AliasManager) GetAlias(uid string) string {
	userRank, ok := table.data.Get(uid)
	if !ok {
		userRank = "?"
	}
	return userRank
}

// maps are referential
func (table *AliasManager) SetAlias(uid string, alias string) error {
	return table.data.Set(uid, alias)
}

func (table *AliasManager) Write() error { return table.data.SyncWrite() }
func (table *AliasManager) Read() error  { return table.data.SyncRead() }

func (table *AliasManager) Close() error {
	return table.data.Close()
}
