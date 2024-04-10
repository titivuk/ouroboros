package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/titivuk/resurrector/watcher"
)

func main() {
	flag.Parse()
	args := flag.Args()

	ch := make(chan int)

	go watcher.StartWatcher(ch)

	termChan := make(chan os.Signal, 1)
	signal.Notify(termChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-termChan
		ch <- 0x2
	}()

	backgroundCtx := context.Background()
	ctx, cancel, cmd := startCmd(backgroundCtx, args)

	for {
		event := <-ch

		switch event {
		case 0x1:
			fmt.Println("Change event received")

			cancel()
			<-ctx.Done()

			err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			if err != nil {
				log.Fatal(err)
			}

			ctx, cancel, cmd = startCmd(backgroundCtx, args)
		case 0x2:
			fmt.Println("Termination event received")

			cancel()
			<-ctx.Done()

			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			os.Exit(1)
		}
	}
}

func startCmd(backgroundCtx context.Context, args []string) (context.Context, context.CancelFunc, *exec.Cmd) {
	ctx, cancel := context.WithCancel(backgroundCtx)
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	// set group id for every child process
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		log.Fatal(err)
	}

	return ctx, cancel, cmd
}
