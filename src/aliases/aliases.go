package aliases

import (
	fd "poll-bot/src/filedict"
)

type AliasTable struct {
	data *fd.FileDict[string, string]
}

func New(path string) (*AliasTable, error) {
	table, err := fd.New[string, string](path)
	if err != nil {
		return nil, err
	}
	return &AliasTable{
		data: table,
	}, nil
}
func (table *AliasTable) GetAlias(uid string) string {
	userRank, ok := table.data.Get(uid)
	if !ok {
		userRank = "?"
	}
	return userRank
}

// maps are referential
func (table *AliasTable) SetAlias(uid string, alias string) error {
	return table.data.Set(uid, alias)
}

func (table *AliasTable) Write() error { return table.data.SyncWrite() }
func (table *AliasTable) Read() error  { return table.data.SyncRead() }
