# Bullsnake

Bullsnake is an experimental implementation of a subset of the Python
programming language in Go.

The project is named after the bullsnake, another name for the Pacific gopher
snake. Gopher snakes commonly eat gophers as part of their prey, making the
name a nod to both Python and Go.

## Design

The [architecture](docs/architecture.md) explains Bullsnake's goals, design
choices, and compatibility boundaries. The [implementation guide](docs/impl.md)
describes the current packages, supported behavior, and visible gaps.

## Embedding

Host access is explicit and replaceable. `bullsnake.New` preserves exactly the
capabilities it receives, so a zero configuration has no filesystem, clock,
network, process, or stream authority:

```go
services := host.Default()
services.Network = nil

interpreter := bullsnake.New(bullsnake.Config{
    Host:             services,
    ModuleSearchPath: []string{"./python"},
})
module, err := interpreter.ExecuteModule("main", "main.py", "import sys\nanswer = 42\n")
```

Mocks and policy engines implement the narrow interfaces in `host`. Returning
`host.ErrDenied` rejects an operation. `bullsnake.NewDefault` is the convenience
constructor when full current-process access is intended.

## CPython test execution

Bullsnake pins compatibility evidence to CPython 3.14.7 commit
`823f0323ee6ec1402088b73bce1a38473cac36dc`. Its Go-backed `unittest` bootstrap
can discover `TestCase` subclasses, run their test methods and fixtures through
the VM, and report results through the configured standard-error capability.
The checked-in integration suite executes three unchanged CPython files and ten
of their tests end to end; this is a focused compatibility claim rather than a
claim that every `unittest` API is implemented.

## Development

Enable the repository's tracked pre-commit hook once per clone:

```sh
git config --local core.hooksPath .githooks
```

The hook runs every pinned LAAS analyzer across the Go module, excluding the
`experiments` package tree:

```sh
go tool laas -funcdoc.limit=5 -exclude-packages='(^|/)experiments($|/)' ./...
```

## License

Bullsnake is available under the [MIT License](LICENSE).
