package aliases

import (
	"iter"
	fd "poll-bot/root/datas/filedict"
	"strconv"
)

type uid = string
type alias = string

type AliasManager struct {
	data *fd.FileDict[uid, alias]
}

func validate(id *uid, name *alias) bool {
	_, err := strconv.Atoi(*id)
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
func (table *AliasManager) GetAlias(id uid) alias {
	userRank, ok := table.data.Get(id)
	if !ok {
		userRank = "?"
	}
	return userRank
}

// maps are referential
func (table *AliasManager) SetAlias(id uid, alias alias) error {
	return table.data.Set(id, alias)
}

func (table *AliasManager) Write() error { return table.data.SyncWrite() }
func (table *AliasManager) Read() error  { return table.data.SyncRead() }

func (table *AliasManager) Close() error {
	return table.data.Close()
}

func (table *AliasManager) Iter() iter.Seq2[uid, alias] {
	return table.data.Iter()
}
