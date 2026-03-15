package main

import (
	"log"
	"os"

	"github.com/vslpsl/tournament/app"
	"github.com/vslpsl/tournament/tg"
)

var (
	dataDir = "./data/media"
)

func main() {
	if err := os.MkdirAll(dataDir, 0777); err != nil {
		log.Fatal(err)
	}

	db, err := app.NewApp(dataDir)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	token := os.Getenv("TG_BOT_TOKEN")

	if err = tg.Run(token, db); err != nil {
		log.Fatal(err)
	}
}
