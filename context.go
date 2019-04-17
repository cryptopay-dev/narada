package tuktuk

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func NewShutdownContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

		<-ch
		cancel()
	}()

	return ctx, cancel
}
