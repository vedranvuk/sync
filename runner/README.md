# Runner

Package runner implements a `Runner` of multiple `RunFunc` implementors simultaneously as a single process whose success depends on success of all executed `RunFunc` instances.

It is similar to `golang.org/x/sync/errgroup` but implements a mechanism and a pattern to gracefully terminate routines with a reason and timeout using contexts as well as thorough error handling.


```
// RunFunc is the prototype of a function that can be run with Runner.
//
// name will be the name under which RunFunc was registered.
//
// stop will be a channel which when becomes readable indicates that
// RunFunc must stop until the deadline specified in the Context of the
// *StopRequest read from the StopChan after which it will be aborted.
//
// result will be a channel where run function must send its operation
// result. A nil value indicates success while an error will abort
// all routines currently running alongside the goroutine that errored.
type RunFunc func(name string, stop StopChan, result ResultChan)
```


## Example

Say you have a few http servers to manage.

```
httpserver := http.Server{Addr: ":80"}
httpsserver := http.Server{Addr: ":443"}

runner := New()

// Register a http server runner func.
runner.Register("http", func(name string, stop StopChan, result ResultChan){
	errchan := make(chan error)
	go func() {
		errchan <- httpserver.ListenAndServe()
	}()
	select {
		case req := <-stop:
			result <- httpserver.Shutdown(req.ctx)
		case err := <-errchan:
			result <- err
	}
})

// Register a https server runner func.
runner.Register("https", func(name string, stop StopChan, result ResultChan){
	errchan := make(chan error)
	go func() {
		errchan <- httpsserver.ListenAndServeTls(nil)
	}()
	select {
		case req := <-stop:
			result <- httpsserver.Shutdown(req.ctx)
		case err := <-errchan:
			result <- err
	}
})

// Stop all servers in 5 seconds.
go func() {
	ctx, err := context.WithTimeout(5 * time.Second)
	defer cancel()
	if err := runner.Stop(ctx); err != nil {
		log.Fatal(err)
	}
}()

// Run all servers.
if err := runner.Run(); err != nil {
	log.Fatal(err)
}
```

## License

MIT. See included LICENSE file.