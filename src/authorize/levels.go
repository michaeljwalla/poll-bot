package authorize

type Rank int

const (
	DEFAULT Rank = iota
	MANAGE
	PROMOTER
	MANAGER
)
