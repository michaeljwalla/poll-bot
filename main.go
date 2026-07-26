package main

import (
	"log"
	"os"
	"os/signal"
	"poll-bot/internal/bot"
	"poll-bot/internal/commands"
	"syscall"

	"github.com/joho/godotenv"
)

func persist() {
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}
func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Couldn't load .env")
	}
	TOKENID := os.Getenv("TOKENID")
	if TOKENID == "" {
		log.Fatal("Error: TOKENID environment variable is not set")
	}
	log.Println("Got token")

	log.Println("Starting bot")
	commands := commands.MainCommands
	session, err := bot.Start(TOKENID, commands)
	if err != nil {
		log.Fatalf("Error creating Discord session: %v", err)
	}
	log.Println("Ready")

	defer func() {
		log.Println("Shutting down")
		bot.EndSession(session)
	}()
	persist()
}
