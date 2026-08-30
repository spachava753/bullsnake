# Bullsnake

Bullsnake is an experimental implementation of a subset of the Python
programming language in Go.

The project is named after the bullsnake, another name for the Pacific gopher
snake. Gopher snakes commonly eat gophers as part of their prey, making the
name a nod to both Python and Go.

## Design

The [runtime design](docs/architecture.md) describes Bullsnake's goals,
compatibility boundaries, invariants, and architecture. The living
[implementation notes](docs/impl.md) record the decisions reflected in the Go
code as the interpreter pipeline is built.

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
