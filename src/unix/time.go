package unix

import (
	"log"
	"strconv"
	"time"
)

// this cast is safe only bits 0-63
func DiffNowEpochMillis(s string) int64 {
	return time.Now().UnixMilli() - int64(IdToEpochMillis(s))
}

func IdToEpochMillis(s string) uint64 {
	const EPOCH_MS uint64 = 1420070400000
	asNum, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		log.Fatal(err)
	}

	return asNum>>22 + EPOCH_MS
}
