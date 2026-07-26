package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"poll-bot/internal/audit"
	"poll-bot/internal/bot"
	"poll-bot/internal/commands"
	"syscall"

	"github.com/joho/godotenv"
)

var logger *audit.Log
var TOKEN string

var LogFlag = audit.LogFlag

const LOG_PATH = "./data/logs/"

var f = fmt.Sprintf

func init() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Couldn't load .env")
	}

	TOKEN = os.Getenv("TOKENID")
	if TOKEN == "" {
		log.Fatal("Error: TOKENID environment variable is not set")
	}

	lg, err := audit.New(LOG_PATH, LogFlag.Default)
	if err != nil {
		log.Fatal(f("Couldn't init logs: %v", err))
	}
	logger = lg
}
func persist() {
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}
func main() {
	//init()
	commands := commands.MainCommands
	session, err := bot.Start(bot.StartInstructions{
		Token:         TOKEN,
		Commands:      commands,
		Logger:        logger,
		TargetAliases: nil,
	})
	if err != nil {
		logger.Panic(f("Error creating Discord session: %v", err))
	}

	defer func() {
		bot.EndSession(session)
	}()
	persist()
}
