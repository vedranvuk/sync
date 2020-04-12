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

const unit = 100 * time.Millisecond

var errTest = fmt.Errorf("test error")

func makeBlockFunc(name string, duration time.Duration) (string, RunFunc) {
	return name, func(name string, stop StopChan, result ResultChan) {
		fmt.Printf("-> Blocker(%s) for %v\n", name, duration)
		time.Sleep(duration)
		fmt.Printf("<- Blocker(%s) after %v\n", name, duration)
		result <- nil
	}
}

func makeAbortFunc(name string, duration time.Duration) (string, RunFunc) {
	return name, func(name string, stop StopChan, result ResultChan) {
		start := time.Now()
		fmt.Printf("-> Abortable(%s) for %v\n", name, duration)
		select {
		case <-stop:
			fmt.Printf("<- Aborting(%s) after %v\n", name, time.Since(start))
			result <- fmt.Errorf("!! Listener (%s) done.", name)
		case <-time.After(duration):
			fmt.Printf("<- Abortable job(%s) after %v\n", name, duration)
			result <- nil
		}
	}
}

func makeErrorFunc(name string, duration time.Duration) (string, RunFunc) {
	return name, func(name string, stop StopChan, result ResultChan) {
		fmt.Printf("-> Erroring(%s) in %v\n", name, duration)
		time.Sleep(duration)
		fmt.Printf("!! Error(%s) after %v\n", name, duration)
		result <- fmt.Errorf("<- Error(%s): %w", name, errTest)
	}
}

func TestBlockingRun(t *testing.T) {
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

func TestAbortableRun(t *testing.T) {
	runner := New()
	runner.Register(makeAbortFunc("1", 1*unit))
	runner.Register(makeAbortFunc("2", 2*unit))
	runner.Register(makeAbortFunc("3", 3*unit))
	runner.Register(makeAbortFunc("4", 4*unit))
	runner.Register(makeAbortFunc("5", 5*unit))
	if err := runner.Run(nil); err != nil {
		t.Fatal(err)
	}
}

func TestMixedRun(t *testing.T) {
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
	if err := runner.Run(nil); err != nil {
		t.Fatal(err)
	}
}

func TestBlockingRunError(t *testing.T) {
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

func TestAbortableRunErrorin3(t *testing.T) {
	runner := New()
	runner.Register(makeAbortFunc("1", 1*unit))
	runner.Register(makeAbortFunc("2", 2*unit))
	runner.Register(makeAbortFunc("3", 3*unit))
	runner.Register(makeAbortFunc("4", 4*unit))
	runner.Register(makeAbortFunc("5", 5*unit))
	runner.Register(makeErrorFunc("0", 3*unit))
	if err := runner.Run(nil); !errors.Is(err, errTest) {
		t.Fatal(err)
	}
}

func TestMixedRunErrorin3(t *testing.T) {
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
	runner.Register(makeErrorFunc("0", 3*unit))
	if err := runner.Run(nil); !errors.Is(err, errTest) {
		t.Fatal(err)
	}
}

func TestBlockingRunErrorWithErrorIn2TimeoutIn2(t *testing.T) {
	runner := New()
	runner.Register(makeBlockFunc("1", 1*unit))
	runner.Register(makeBlockFunc("2", 2*unit))
	runner.Register(makeBlockFunc("3", 3*unit))
	runner.Register(makeBlockFunc("4", 4*unit))
	runner.Register(makeBlockFunc("5", 5*unit))
	runner.Register(makeErrorFunc("0", 2*unit))

	ctx, cancel := context.WithTimeout(context.Background(), 2*unit)
	defer cancel()
	if err := runner.Run(ctx); !errors.Is(err, errTest) {
		t.Fatal(err)
	}
}

func TestBlockingStopIn3(t *testing.T) {
	runner := New()
	runner.Register(makeBlockFunc("1", 1*unit))
	runner.Register(makeBlockFunc("2", 2*unit))
	runner.Register(makeBlockFunc("3", 3*unit))
	runner.Register(makeBlockFunc("4", 4*unit))
	runner.Register(makeBlockFunc("5", 5*unit))
	go func() {
		time.Sleep(3 * unit)
		fmt.Println("!! Stop")
		if err := runner.Stop(nil); err != nil {
			t.Fatal(err)
		}
	}()
	if err := runner.Run(nil); err != nil {
		t.Fatal(err)
	}
}

func TestAbortableStopIn3(t *testing.T) {
	runner := New()
	runner.Register(makeAbortFunc("1", 1*unit))
	runner.Register(makeAbortFunc("2", 2*unit))
	runner.Register(makeAbortFunc("3", 3*unit))
	runner.Register(makeAbortFunc("4", 4*unit))
	runner.Register(makeAbortFunc("5", 5*unit))
	go func() {
		time.Sleep(3 * unit)
		fmt.Println("!! Stop")
		if err := runner.Stop(nil); err != nil {
			t.Fatal(err)
		}
	}()
	if err := runner.Run(nil); err != nil {
		t.Fatal(err)
	}
}

func TestMixedStopIn3(t *testing.T) {
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
		time.Sleep(3 * unit)
		fmt.Println("!! Stop")
		if err := runner.Stop(nil); err != nil {
			t.Fatal(err)
		}
	}()
	if err := runner.Run(nil); err != nil {
		t.Fatal(err)
	}
}

func TestMixedStopIn2WithTimeoutIn1(t *testing.T) {
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
		time.Sleep(2 * unit)
		ctx, cancel := context.WithTimeout(context.Background(), 1*unit)
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
	runner.Register(makeBlockFunc("1", 1*unit))
	go func() {
		time.Sleep(2 * unit)
		if err := runner.Run(nil); err != nil && !errors.Is(err, ErrRunBusy) {
			t.Fatal(err)
		}
	}()
	if err := runner.Run(nil); err != nil {
		t.Fatal(err)
	}
}

func TestBusyStop(t *testing.T) {
	runner := New()
	runner.Register(makeBlockFunc("1", 3*unit))
	go func() {
		time.Sleep(1 * unit)
		if err := runner.Stop(nil); err != nil && !errors.Is(err, ErrStopBusy) {
			t.Fatal(err)
		}
	}()
	go func() {
		time.Sleep(2 * unit)
		if err := runner.Stop(nil); err != nil && !errors.Is(err, ErrStopBusy) {
			t.Fatal(err)
		}
	}()
	if err := runner.Run(nil); err != nil {
		t.Fatal(err)
	}
}
