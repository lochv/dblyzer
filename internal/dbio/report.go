package dbio

import (
	"dblyzer/internal/config"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type WebApp struct {
	AppName string   `json:"appname,omitempty"`
	Version string   `json:"version,omitempty"`
	Implies []string `json:"implies,omitempty"`
}

type Report struct {
	Host    string   `json:"host"`
	Ip      string   `json:"ip"`
	Port    int      `json:"port"`
	Service string   `json:"service"`
	Banner  string   `json:"banner,omitempty"`
	CName   string   `json:"cname,omitempty"`
	Favicon string   `json:"favicon,omitempty"`
	Apps    []WebApp `json:"apps,omitempty"`
	Domains []string `json:"domains,omitempty"`
}

func (this *Report) IsSetApp(field string) bool {
	for _, a := range this.Apps {
		if field == a.AppName {
			return true
		}
	}
	return false
}

func NewRp() chan Report {
	in := make(chan Report, 32)
	switch config.Conf.ReportMode {
	case "console":
		go func() {
			for {
				mess := <-in
				if !strings.Contains(mess.Service, "http") {
					continue
				}
				fmt.Printf("\n%s://%s:%d use %s", mess.Service, mess.Host, mess.Port, mess.Apps)
			}
		}()
	case "remote":
	//todo impl
	case "file":
		go func() {
			for {
				mess := <-in
				f, err := os.OpenFile(config.Conf.OutputFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
				if err != nil {
					continue
				}
				bytes, _ := json.Marshal(mess)
				f.WriteString("\n")
				f.Write(bytes)
				f.Close()
			}
		}()
	}
	return in
}
