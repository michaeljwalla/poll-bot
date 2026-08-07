package audit

import (
	"fmt"
	"strings"
)

var LogFlag = struct {
	Default           int
	ErrorIsPanic      int
	ErrorIsWarn       int
	ErrorIsPass       int
	WriteMemory       int
	WriteFile         int
	FileAppend        int
	FileOverwrite     int
	FileErrorNotFound int
}{
	Default:           0,
	ErrorIsPanic:      1 << 0, //e1
	ErrorIsWarn:       1 << 1, //e1
	ErrorIsPass:       1 << 2, //e1
	WriteMemory:       1 << 3, //i2
	WriteFile:         1 << 4, //i2
	FileAppend:        1 << 5, //e3
	FileOverwrite:     1 << 6, //e3
	FileErrorNotFound: 1 << 7,
}

var logStrings = map[int]string{
	LogFlag.Default:           "LogFlag.Default",
	LogFlag.ErrorIsPanic:      "LogFlag.ErrorIsPanic",
	LogFlag.ErrorIsWarn:       "LogFlag.ErrorIsWarn",
	LogFlag.ErrorIsPass:       "LogFlag.ErrorIsPass",
	LogFlag.WriteMemory:       "LogFlag.WriteMemory",
	LogFlag.WriteFile:         "LogFlag.WriteFile",
	LogFlag.FileAppend:        "LogFlag.FileAppend",
	LogFlag.FileOverwrite:     "LogFlag.FileOverwrite",
	LogFlag.FileErrorNotFound: "LogFlag.FileErrorNotFound",
}

func Stringify(flag int) string {
	if val, ok := logStrings[flag]; ok {
		return val
	}
	return fmt.Sprintf("Unknown %b", flag)
}
func StringFlags(flags int) string {
	sFlags := []string{}
	for flag, str := range logStrings {
		if flag == LogFlag.Default || !hasFlag(flags, flag) {
			continue
		}
		sFlags = append(sFlags, str)
	}
	return fmt.Sprintf("Flags 0b%b -> %s", flags, strings.Join(sFlags, "|"))
}
func init() {
	LogFlag.Default = LogFlag.ErrorIsPanic | LogFlag.WriteMemory | LogFlag.WriteFile | LogFlag.FileAppend
}
func hasFlag(cmp int, f int) bool {
	return f&cmp == f
}
func checkFlags(flags int) error {
	if flags&(LogFlag.WriteFile|LogFlag.WriteMemory) == 0 {
		return fmt.Errorf("flag configuration missing Write: %b", flags)
	}
	mod := flags
	for flag := range logStrings {
		mod &= ^flag
	}
	if mod != 0 {
		return fmt.Errorf("unknown flag configuration: %b", flags)
	}
	return nil
}
