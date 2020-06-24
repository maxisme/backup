package main

import (
	"log"
	"os"

	"github.com/maxisme/backup/utils"
	"github.com/robfig/cron/v3"
)

func backupAllServers(path string) error {
	servers, err := utils.FileToServers(path)
	if err != nil {
		return err
	}

	utils.BackupServers(servers)
	return nil
}

func main() {
	cronSpec := os.Getenv("CRON")
	if cronSpec == "" {
		cronSpec = "0 */12 * * *"
	}

	c := cron.New()
	isBackingUp := false
	_, err := c.AddFunc(cronSpec, func() {
		if !isBackingUp {
			isBackingUp = true
			if err := backupAllServers("/servers.json"); err != nil {
				panic(err)
			}
			isBackingUp = false
		} else {
			log.Println("Already backing up")
		}
	})
	if err != nil {
		panic(err)
	}
	c.Start()

	select {} // Keep alive
}
