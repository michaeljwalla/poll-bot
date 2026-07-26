package audit

//TODO logflags

import (
	"fmt"
	"log"
	"os"
	"time"
)

type iLog interface {
	Panic(s string)
	Warn(s string) (err error)
	Add(s string) (err error)
	Close(s string) (err error)
}
type Log struct {
	Path  string
	File  *os.File
	flags int
	iLog
}

func New(path string, flags int) (log *Log, err error) {
	if err = checkFlags(flags); err != nil {
		return
	}

	if !hasFlag(flags, LogFlag.WriteFile) {
		log = &Log{
			flags: flags,
		}
		log.Add("Instance Ready")
		return
	}

	// file flags
	var fflag int = os.O_WRONLY
	if hasFlag(fflag, LogFlag.FileAppend) {
		fflag |= os.O_APPEND
	}
	if !hasFlag(fflag, LogFlag.FileErrorNotFound) {
		fflag |= os.O_CREATE
	}

	file, err := os.OpenFile(path, fflag, 0644)
	if err != nil {
		return
	}
	log = &Log{
		Path:  path,
		File:  file,
		flags: flags,
	}
	log.Add("File Ready")
	return
}

// assumes exclusivity; mutates flags on a copy to operate
func handleError(e error, log Log) (err error) {
	if hasFlag(log.flags, LogFlag.ErrorIsPass) {
		return e
	}
	if hasFlag(log.flags, LogFlag.ErrorIsWarn) {
		log.flags = log.flags&LogFlag.ErrorIsWarn | LogFlag.ErrorIsPass
		log.Warn(fmt.Sprintf("[ERR] %s", e))
		return
	}
	log.flags = log.flags&LogFlag.ErrorIsPanic | LogFlag.ErrorIsPass
	log.Panic(fmt.Sprintf("[ERR] %s", e))
	return
}

// will not pass errors, always writes to memory
func (l Log) Panic(s string) {
	timeNow := time.Now().Format("yyyy/mm/dd hh:mm:ss")
	output := fmt.Sprintf("[PANIC] %s %s\n", timeNow, s)
	if hasFlag(l.flags, LogFlag.WriteFile) {
		l.File.WriteString(fmt.Sprintf("%s %s", timeNow, output))
	}
	//
	log.Panic(output)
}
func (l Log) Warn(s string) (err error) {
	timeNow := time.Now().Format("yyyy/mm/dd hh:mm:ss")
	output := fmt.Sprintf("[WARN] %s %s\n", timeNow, s)
	//
	if hasFlag(l.flags, LogFlag.WriteFile) {
		if _, err = fmt.Fprintf(l.File, "%s %s", timeNow, output); err != nil {
			return handleError(err, l)
		}
	}
	//
	if hasFlag(l.flags, LogFlag.WriteMemory) {
		log.Print(output)
	}
	return
}
func (l Log) Add(s string) (err error) {
	timeNow := time.Now().Format("yyyy/mm/dd hh:mm:ss")
	output := fmt.Sprintf("[LOG] %s %s\n", timeNow, s)
	//
	if hasFlag(l.flags, LogFlag.WriteFile) {
		if _, err = fmt.Fprintf(l.File, "%s %s", timeNow, output); err != nil {
			return handleError(err, l)
		}
	}
	//
	if hasFlag(l.flags, LogFlag.WriteMemory) {
		log.Print(output)
	}
	return
}
