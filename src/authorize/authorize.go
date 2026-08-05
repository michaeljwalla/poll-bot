package authorize

type AuthorizedTable map[string]Rank

var AuthTable = AuthorizedTable{}

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
