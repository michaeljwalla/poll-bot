package authorize

const (
	DEFAULT Rank = iota
	MANAGE
	PROMOTER
	MANAGER
)
const NUM_RANKS = MANAGER + 1

var rankStrings = map[Rank]string{
	DEFAULT:  "DEFAULT",
	MANAGE:   "MANAGE",
	PROMOTER: "PROMOTER",
	MANAGER:  "MANAGER",
}

func Stringify(rank Rank) string {
	if name, ok := rankStrings[rank]; ok {
		return name
	}
	return "UNKNOWN"
}
