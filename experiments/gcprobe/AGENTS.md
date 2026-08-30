# GC probe notes

Read `README.md` here and the `Memory management and lifecycle` section in
`docs/architecture.md` before changing this experiment.

## Package contract

This command is an experiment, not production runtime code. It records what was
observed from Go's collector, weak pointers, cleanups, and finalizers under a
named Go version and platform.

Do not import this package from Bullsnake's compiler or runtime. Move an idea
into production only when an implemented Python feature needs it and has its own
runtime tests.

A forced collection and a timeout make observations easier; they do not promise
that a cleanup or finalizer will run. Report missing callbacks as inconclusive or
not observed when Go permits either result.

Keep Python execution out of cleanup and finalizer callbacks. The experiment may
send a plain notification to another goroutine, but the architecture requires
Python work to happen later under runtime control.

## Verification

Run the ordinary command and the race detector when changing the probe:

```sh
go run ./experiments/gcprobe
go run -race ./experiments/gcprobe
```

If the observations or conclusions change, update both this package's `README.md`
and the recorded results in `docs/architecture.md`. Include the Go version,
platform, run count, and any inconclusive result.
