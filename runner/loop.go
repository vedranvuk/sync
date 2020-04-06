// Copyright 2019 Vedran Vuk. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package runner

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Loop
func Loop(ctx context.Context, runner *Runner) error {
	interrupt := make(chan os.Signal)
	signal.Notify(interrupt, os.Interrupt)

	go func() {
		if err := runner.Run(ctx); err != nil {
			interrupt <- syscall.SIGKILL
			// TODO Handle error.
			return
		}
		interrupt <- syscall.SIGQUIT
	}()

	sig := <-interrupt
	switch sig {
	case syscall.SIGINT:
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := runner.Stop(ctx); err != nil {
			// TODO Handle stop errors.
		}
	case syscall.SIGQUIT:
		// TODO All RunFuncs done.
	case syscall.SIGKILL:
		// TODO Run error.
	}

	return nil
}
