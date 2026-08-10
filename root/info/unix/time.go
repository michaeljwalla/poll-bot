package unix

import (
	"fmt"
	"log"
	"strconv"
	"time"
)

// this cast is safe only bits 0-63
func DiffNowEpochMillis(s string) int64 {
	return time.Now().UnixMilli() - int64(IdToEpochMillis(s))
}

const EPOCH_MS uint64 = 1420070400000

func IdToEpochMillis(s string) uint64 {
	asNum, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		log.Fatal(err)
	}

	return asNum>>22 + EPOCH_MS
}
func TimeToSnowflake(t time.Time) string {
	unixMs := t.UnixNano() / int64(time.Millisecond)

	discordMs := unixMs - int64(EPOCH_MS)

	return fmt.Sprintf("%d", uint64(discordMs)<<22)
}
