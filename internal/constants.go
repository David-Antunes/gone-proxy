package internal

import (
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
)

const QueueSize = 10000

func init() {
	Stop = make(chan os.Signal)
	signal.Notify(Stop, os.Interrupt, syscall.SIGTERM)
}

func ShutdownAndLog(err error) {
	fmt.Println(err)
	debug.PrintStack()
	Stop <- os.Signal(syscall.SIGTERM)
}

var Stop chan os.Signal
