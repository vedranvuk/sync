// Copyright 2019 Vedran Vuk. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

// Package runner implements an async goroutine runner with cancelation
// suppport and error control.
package runner

import (
	"context"
	"sync"
	"time"

	"github.com/vedranvuk/errorex"
)

// RunFunc is the prototype of a function that can be run with Runner.
//
// name will be the name under which run function was registered.
//
// stop will be a channel which when becomes readable indicates that
// RunFunc must stop until the deadline specified in the Context of the
// *StopRequest read from the StopChan after which it will be aborted.
//
// result will be a channel where run function must send its operation
// result. A nil value indicates success while an error will abort
// all routines currently running alongside the goroutine that errored.
type RunFunc func(name string, stop StopChan, result ResultChan)

// StopChan is a chan of *StopRequest.
// It is passed to a RunFunc which must stop when StopChan becomes readable.
type StopChan chan *StopRequest

// StopRequest is a stop request sent to a RunFunc.
type StopRequest struct {
	// StopReason is the reason for stopping the RunFunc.
	StopReason
	// Context will contain a timeout up until which RunFunc must
	// stop or it will be discarded and its' ResultChan not read.
	Context context.Context
}

// ResultChan is a chan of error.
// It is written to by a RunFunc when it exits. A non-nil value indicates an
// error and causes Runner to stop all other running RunFuncs.
type ResultChan chan error

// StopReason is the reason why a RunFunc is being requested to stop.
// RunFuncs can use this value to determine how to shut down.
type StopReason byte

const (
	// ReasonInvalid is the undefined/invalid reason.
	ReasonInvalid StopReason = iota
	// ReasonUserAbort specifies that the user requested the Runner to stop.
	ReasonUserAbort
	// ReasonError specifies that the RunFunc must stop due to an error.
	// In this case RunFunc should shut down as quickly as possible.
	ReasonError
)

// String implements Stringer interface on StopReason.
func (sr StopReason) String() (result string) {
	switch sr {
	case ReasonInvalid:
		result = "Invalid"
	case ReasonUserAbort:
		result = "UserAbort"
	case ReasonError:
		result = "Error"
	}
	return
}

var (
	// ErrRunner is the base error of runner package.
	ErrRunner = errorex.New("runner")
	// ErrRun is returned if Run fails due to an error from one of RunFuncs.
	ErrRun = ErrRunner.Wrap("run error")
	// ErrStop is returned if an error occurs during a Stop call.
	ErrStop = ErrRunner.Wrap("stop error")
	// ErrRunBusy is returned when a Run function is called while busy.
	ErrRunBusy = ErrRunner.Wrap("run is busy")
	// ErrStopBusy is returned when a Stop function is called while busy.
	ErrStopBusy = ErrRunner.Wrap("run is busy")
	// ErrInvalidName is returned when a RunFunc with an invalid name
	// is being registered with Runner.
	ErrInvalidName = ErrRunner.WrapFormat("invalid name '%s'")
	// ErrDuplicateName is returned when a RunFunc is being registered under
	// a duplicate name with Runner.
	ErrDuplicateName = ErrRunner.WrapFormat("duplicate name '%s'")
	// ErrInvalidRunFunc is returned if a nil RunFunc is specified.
	ErrInvalidRunFunc = ErrRunner.Wrap("invalid runfunc")
)

// runFuncInfo is info about a running RunFunc.
type runFuncInfo struct {
	StopChan
	RunFunc
}

// Runner runs multiple RunFuncs in parallel as a single process.
type Runner struct {
	mu         sync.Mutex
	running    bool
	stopReason StopReason
	timeout    <-chan time.Time
	errors     chan error
	funcs      map[string]*runFuncInfo
}

// New returns a new Runner.
func New() *Runner {
	return &Runner{
		sync.Mutex{},
		false,
		ReasonInvalid,
		make(<-chan time.Time),
		make(chan error),
		make(map[string]*runFuncInfo),
	}
}

// Register registers a RunFunc f under specified name or returns
// an error if one occurs.
// name must be unique in Runner.
func (r *Runner) Register(name string, f RunFunc) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if f == nil {
		return ErrInvalidRunFunc
	}
	if name == "" {
		return ErrDuplicateName.WrapArgs(name)
	}
	if _, exists := r.funcs[name]; exists {
		return ErrDuplicateName.WrapArgs(name)
	}
	r.funcs[name] = &runFuncInfo{nil, f}
	return nil
}

// run is the implementation of Run.
func (r *Runner) run(ctx context.Context) (err error) {

	if ctx == nil {
		var cancel func()
		ctx, cancel = context.WithTimeout(context.Background(), 1*time.Minute)
		defer cancel()
	}

	wgroup := &sync.WaitGroup{}
	wgdone := make(chan struct{})
	results := make(ResultChan)
	for fname, finfo := range r.funcs {
		finfo.StopChan = make(StopChan)
		wgroup.Add(1)
		go finfo.RunFunc(fname, finfo.StopChan, results)
	}
	go func() {
		wgroup.Wait()
		wgdone <- struct{}{}
	}()

	var errors []error
	for r.running = true; r.running; {
		select {
		case result := <-results:
			wgroup.Done()
			if result == nil {
				continue
			}
			if r.stopReason == ReasonInvalid {
				err = result
				r.stopReason = ReasonError
				deadline, ok := ctx.Deadline()
				if ok {
					r.timeout = time.After(time.Until(deadline))
				}
				r.stop(ctx)
			}
			errors = append(errors, result)
		case <-wgdone:
			r.running = false
		case <-r.timeout:
			r.running = false
		}
	}

	if err != nil {
		var eex *errorex.ErrorEx
		switch r.stopReason {
		case ReasonUserAbort:
			eex = ErrStop.WrapCause("", err)
			defer func() {
				r.errors <- err
			}()
		case ReasonError:
			eex = ErrRun.WrapCause("", err)
		}
		for _, e := range errors {
			eex.Extra(e)
		}
		err = eex
	} else {
		if r.stopReason == ReasonUserAbort {
			defer func() { r.errors <- nil }()
		}
	}
	r.stopReason = ReasonInvalid

	return
}

// Run runs registered funcs, blocks and eventually returns on error
// or nil when all funcs have completed successfully, either due to
// completion or implicit Stop() call.
//
// If a Stop call is made, Run will return a nil error and any errors
// that occur during the shutdown will be passed to Stop return value.
//
// If specified, ctx should contain a timeout for RunFuncs to shut
// down in case of a run error after which they are terminated
// forcefully. If ctx is nil, timeout defaults to one minute.
//
// In case of an error an *ErrorEx is returned containing the error
// that caused Run to fail and any errors that occured during RunFunc
// shutdowns in the Extra fields of the resulting error.
func (r *Runner) Run(ctx context.Context) error {
	r.mu.Lock()
	if r.running {
		defer r.mu.Unlock()
		return ErrRunBusy.Wrap("already running")
	}
	if r.stopReason != ReasonInvalid {
		defer r.mu.Unlock()
		return ErrRunBusy.Wrap("cannot run, stopping")
	}
	r.mu.Unlock()
	return r.run(ctx)
}

// stop is the implementation of Stop.
func (r *Runner) stop(ctx context.Context) {
	if ctx == nil {
		newctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		defer cancel()
		ctx = newctx
	}
	deadline, ok := ctx.Deadline()
	if ok {
		r.timeout = time.After(time.Until(deadline))
	} else {
		panic("no context deadline set")
	}
	for _, info := range r.funcs {
		go func(stop StopChan) {
			stop <- &StopRequest{r.stopReason, ctx}
		}(info.StopChan)
	}
}

// Stop issues an async stop request to all RunFuncs and returns an
// error if one occured during the shutdown.
//
// If specified, ctx should contain a timeout for RunFuncs to shut
// down in case of a run error after which they are terminated
// forcefully. If ctx is nil, timeout defaults to one minute.
//
// In case of an error an *ErrorEx is returned containing all errors
// in Extras() that occured during the shutdown.
func (r *Runner) Stop(ctx context.Context) (err error) {
	r.mu.Lock()
	if r.stopReason != ReasonInvalid {
		defer r.mu.Unlock()
		return ErrStopBusy.Wrap("already stopping")
	}
	if !r.running {
		defer r.mu.Unlock()
		return ErrStopBusy.Wrap("already stopped")
	}
	r.stopReason = ReasonUserAbort
	r.mu.Unlock()
	r.stop(ctx)
	err = <-r.errors
	return
}

// Running returns if runner is running.
func (r *Runner) Running() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running
}
