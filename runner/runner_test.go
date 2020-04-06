// Copyright 2019 Vedran Vuk. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package runner

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

const unit = 10 * time.Millisecond

var errTest = fmt.Errorf("test error")

func makeBlockFunc(name string, duration time.Duration) (string, RunFunc) {
	return name, func(name string, stop StopChan, result ResultChan) {
		fmt.Printf("-> Blocking(%s) for %v\n", name, duration)
		time.Sleep(duration)
		fmt.Printf("<- Blocking(%s) for %v\n", name, duration)
		result <- nil
	}
}

func makeAbortFunc(name string, duration time.Duration) (string, RunFunc) {
	return name, func(name string, stop StopChan, result ResultChan) {
		fmt.Printf("-> Listening(%s) for %v\n", name, duration)
		select {
		case <-stop:
			fmt.Printf("<- Aborting(%s) for %v\n", name, duration)
			result <- fmt.Errorf("!! Listener (%s) aborted.", name)
		case <-time.After(duration):
			fmt.Printf("<- Listening(%s) for %v\n", name, duration)
			result <- nil
		}
	}
}

func makeErrorFunc(name string, duration time.Duration) (string, RunFunc) {
	return name, func(name string, stop StopChan, result ResultChan) {
		fmt.Printf("-> Erroring(%s) for %v\n", name, duration)
		time.Sleep(duration)
		fmt.Printf("!! Error(%s) for %v\n", name, duration)
		result <- fmt.Errorf("!! Error(%s): %w", name, errTest)
	}
}

func TestRun(t *testing.T) {
	runner := New()
	runner.Register(makeBlockFunc("1", 1*unit))
	runner.Register(makeBlockFunc("2", 2*unit))
	runner.Register(makeBlockFunc("3", 3*unit))
	runner.Register(makeBlockFunc("4", 4*unit))
	runner.Register(makeBlockFunc("5", 5*unit))
	if err := runner.Run(nil); err != nil {
		t.Fatal(err)
	}
}

func TestRunError(t *testing.T) {
	runner := New()
	runner.Register(makeBlockFunc("1", 1*unit))
	runner.Register(makeBlockFunc("2", 2*unit))
	runner.Register(makeBlockFunc("3", 3*unit))
	runner.Register(makeBlockFunc("4", 4*unit))
	runner.Register(makeBlockFunc("5", 5*unit))
	runner.Register(makeErrorFunc("0", 3*unit))
	if err := runner.Run(nil); !errors.Is(err, errTest) {
		t.Fatal(err)
	}
}

func TestRunErrorWithTimeout(t *testing.T) {
	runner := New()
	runner.Register(makeBlockFunc("1", 1*unit))
	runner.Register(makeBlockFunc("2", 2*unit))
	runner.Register(makeBlockFunc("3", 3*unit))
	runner.Register(makeBlockFunc("4", 4*unit))
	runner.Register(makeBlockFunc("5", 5*unit))
	runner.Register(makeErrorFunc("0", 3*unit))

	ctx, cancel := context.WithTimeout(context.Background(), 1*unit)
	defer cancel()
	if err := runner.Run(ctx); !errors.Is(err, errTest) {
		t.Fatal(err)
	}
}

func TestStop(t *testing.T) {
	runner := New()
	runner.Register(makeBlockFunc("1", 1*unit))
	runner.Register(makeBlockFunc("2", 2*unit))
	runner.Register(makeBlockFunc("3", 3*unit))
	runner.Register(makeBlockFunc("4", 4*unit))
	runner.Register(makeBlockFunc("5", 5*unit))
	runner.Register(makeAbortFunc("6", 1*unit))
	runner.Register(makeAbortFunc("7", 2*unit))
	runner.Register(makeAbortFunc("8", 3*unit))
	runner.Register(makeAbortFunc("9", 4*unit))
	runner.Register(makeAbortFunc("10", 5*unit))
	go func() {
		time.Sleep(1 * unit)
		fmt.Println("!! Stop")
		if err := runner.Stop(nil); err != nil {
			t.Fatal(err)
		}
	}()
	if err := runner.Run(nil); err != nil {
		t.Fatal(err)
	}
}

func TestStopWithTimeout(t *testing.T) {
	runner := New()
	runner.Register(makeBlockFunc("1", 1*unit))
	runner.Register(makeBlockFunc("2", 2*unit))
	runner.Register(makeBlockFunc("3", 3*unit))
	runner.Register(makeBlockFunc("4", 4*unit))
	runner.Register(makeBlockFunc("5", 5*unit))
	runner.Register(makeAbortFunc("6", 1*unit))
	runner.Register(makeAbortFunc("7", 2*unit))
	runner.Register(makeAbortFunc("8", 3*unit))
	runner.Register(makeAbortFunc("9", 4*unit))
	runner.Register(makeAbortFunc("10", 5*unit))
	go func() {
		time.Sleep(1 * unit)
		ctx, cancel := context.WithTimeout(context.Background(), 2*unit)
		defer cancel()
		fmt.Println("!! Stop")
		if err := runner.Stop(ctx); err != nil {
			t.Fatal(err)
		}
	}()
	if err := runner.Run(nil); err != nil {
		t.Fatal(err)
	}
}

func TestBusyRun(t *testing.T) {
	runner := New()
	runner.Register(makeBlockFunc("1", 10*unit))
	go func() {
		time.Sleep(5 * unit)
		if err := runner.Run(nil); !errors.Is(err, ErrRunBusy) {
			t.Fatal()
		}
	}()
	if err := runner.Run(nil); err != nil {
		t.Fatal(err)
	}
}

func TestBusyStop(t *testing.T) {
	runner := New()
	runner.Register(makeBlockFunc("1", 10*unit))
	go func() {
		time.Sleep(3 * unit)
		if err := runner.Stop(nil); err != nil {
			t.Fatal(err)
		}
	}()
	go func() {
		time.Sleep(6 * unit)
		if err := runner.Stop(nil); !errors.Is(err, ErrStopBusy) {
			t.Fatal(err)
		}
	}()
	if err := runner.Run(nil); err != nil {
		t.Fatal(err)
	}
}
