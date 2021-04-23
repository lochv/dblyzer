package dbio

import (
	"bufio"
	"dblyzer/internal/config"
	"encoding/json"
	"os"
)

type Receive struct {
	Host    string `json:"host"`
	Ip      string `json:"ip"`
	Port    int    `json:"port"`
	Service string `json:"service"`
	Banner  string `json:"banner,omitempty"`
}

func NewRecv() chan Receive {
	outChan := make(chan Receive, 16)
	switch config.Conf.ReceiveMode {
	case "file":
		go func() {
			file, err := os.Open(config.Conf.InputFile)
			if err != nil {
				panic(err.Error())
			}
			defer file.Close()
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				r := Receive{}
				err := json.Unmarshal(scanner.Bytes(), &r)
				if err != nil {
					continue
				}
				outChan <- r
			}
		}()
	case "remote":
		//todo impl
	}
	return outChan
}
