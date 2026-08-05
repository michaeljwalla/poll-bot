package authorize

type AuthTable map[string]Rank

func CanUse(uid string, rank Rank, auth AuthTable) bool {
	return GetRank(uid, auth) >= rank
}

func GetRank(uid string, auth AuthTable) Rank {
	userRank, ok := auth[uid]
	if !ok {
		userRank = DEFAULT
	}
	return userRank
}
