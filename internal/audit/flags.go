package audit

import (
	"fmt"
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
	ErrorIsPanic:      0b00000001, //e1
	ErrorIsWarn:       0b00000010, //e1
	ErrorIsPass:       0b00000100, //e1
	WriteMemory:       0b00001000, //i2
	WriteFile:         0b00010000, //i2
	FileAppend:        0b00100000, //e3
	FileOverwrite:     0b01000000, //e3
	FileErrorNotFound: 0b10000000,
}

func init() {
	LogFlag.Default = LogFlag.ErrorIsPass | LogFlag.WriteMemory
}

var exclusive map[int]string = map[int]string{
	LogFlag.ErrorIsPanic | LogFlag.ErrorIsPass | LogFlag.ErrorIsWarn: "ErrorIsPanic/Warn/Pass",
	LogFlag.FileAppend | LogFlag.FileOverwrite:                       "FileAppend/Overwrite",
}
var inclusive map[int]string = map[int]string{
	LogFlag.WriteMemory | LogFlag.WriteFile: "WriteMemory/File",
}

func hasFlag(cmp int, f int) bool {
	return f&cmp == f
}
func consume(exc int, cmp int) (count int) {
	check := exc & cmp
	for check != 0 {
		if hasFlag(check, 0b1) {
			count++
		}
		check >>= 1
	}
	return
}
func checkFlags(in int) (err error) {
	for flag, reason := range exclusive {
		if consume(flag, in) == 1 {
			continue
		}
		err = fmt.Errorf("Specify flag: %s", reason)
		return
	}
	for flag, reason := range inclusive {
		if consume(flag, in) != 0 {
			continue
		}
		err = fmt.Errorf("Specify flag(s): %s", reason)
		return
	}
	return
}
