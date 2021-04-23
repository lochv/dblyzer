package dblyzer

import (
	"dblyzer/internal/dbio"
)

type dblyzer struct {
	kill chan int
	*engine
}

func New(filePath string, in chan dbio.Receive, out chan dbio.Report) *dblyzer {

	return &dblyzer{
		engine: newEngine(filePath, in, out),
		kill:   make(chan int),
	}
}

func (this *dblyzer) Run(worker int) {
	go func() {
		for i := 0; i < worker; i++ {
			this.worker()
		}
	}()
}
func (this *dblyzer) Wait() {
	<-this.kill
}