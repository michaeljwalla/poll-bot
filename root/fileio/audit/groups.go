package audit

import (
	"fmt"
	"math"
	"poll-bot/root/datas/set"
	"slices"
	"strings"
)

var LogGroup = struct {
	PANIC    string
	ERR      string
	WARN     string
	LOG      string
	INIT     string
	AUDIT    string
	BOT      string
	INTERACT string
}{
	PANIC:    "PANIC",
	ERR:      "ERR",
	WARN:     "WARN",
	LOG:      "LOG",
	INIT:     "INIT",
	AUDIT:    "AUDIT",
	BOT:      "BOT",
	INTERACT: "INTERACT",
}
var logGroupPriorityMap = map[string]int{
	"UNKNOWN":  math.MaxInt,
	"PANIC":    0,
	"ERR":      1,
	"WARN":     2,
	"LOG":      3,
	"INIT":     4,
	"AUDIT":    5,
	"BOT":      6,
	"INTERACT": 7,
}

func FormatGroupings(groups ...string) string {
	if len(groups) == 0 {
		return "[NONE]"
	}
	groups = *set.New(groups...).Items()
	slices.SortFunc(groups, func(g1 string, g2 string) int {
		p1, ok := logGroupPriorityMap[g1]
		if !ok {
			p1 = logGroupPriorityMap["UNKNOWN"]
		}
		p2, ok := logGroupPriorityMap[g2]
		if !ok {
			p2 = logGroupPriorityMap["UNKNOWN"]
		}
		return p1 - p2
	})
	return fmt.Sprintf("[%s]", strings.Join(groups, "] ["))
}
