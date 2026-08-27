# Go GC probe

This command explores whether Bullsnake can leave Python object memory
management to Go 1.27's runtime.

Run it from the repository root:

```sh
go run ./experiments/gcprobe
```

The probe covers:

- Collection of an unreachable cycle stored through interface fields
- Weak pointers and cleanup-based weak-reference notification
- Cleanup execution on a separate goroutine
- Resurrection of an acyclic object through `runtime.SetFinalizer`
- The limitation on finalizers attached to reference cycles

The command forces collections to make observations practical. Its timeouts do
not turn finalizers, cleanups, or weak pointers into deterministic APIs. Go's
documentation explicitly says those events may happen arbitrarily late and may
never happen in some cases. The probe should therefore report `inconclusive` or
`not observed` rather than treating every missing callback as a test failure.

This is an architectural experiment, not part of the Bullsnake runtime. The
recorded design conclusions live in
[the runtime design](../../docs/architecture.md#go-gc-probe-results).
