package aliases

import (
	"encoding/json"
	"os"
)

type AliasedTable map[string]string

var AliasTable = AliasedTable{}
var path string

func SetGlobalPath(toPath string) {
	path = toPath
}

func GetAlias(uid string, auth AliasedTable) string {
	userRank, ok := auth[uid]
	if !ok {
		userRank = "?"
	}
	return userRank
}

// maps are referential
func SetAlias(uid string, alias string, data AliasedTable) {
	data[uid] = alias
}

// TODO tmp + mv for this and auth
func SetFile(data AliasedTable) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close() //nolint
	return json.NewEncoder(file).Encode(data)
}
