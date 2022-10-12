package common

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

type SimpleFunc func()

func RegisterExitSignal(exitFunc SimpleFunc) {
	c := make(chan os.Signal, 5)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		for s := range c {
			switch s {
			case syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
				exitFunc()
			default:
				fmt.Println("其他信号:", s)
			}
		}
	}()
}
