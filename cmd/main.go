package main

import (
	"dblyzer"
	"dblyzer/internal/config"
	"dblyzer/internal/dbio"
)

func main() {
	recvChan := dbio.NewRecv()
	rpChan := dbio.NewRp()
	db := dblyzer.New("./apps.json", recvChan, rpChan)
	db.Run(config.Conf.Workers)
	db.Wait()
}
