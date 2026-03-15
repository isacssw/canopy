# Contributing to Canopy

## Reporting Bugs

Open a [GitHub Issue](https://github.com/isacssw/canopy/issues) using the bug report template. Include your OS, tmux version, and steps to reproduce.

## Submitting a PR

1. Fork the repo and create a branch from `main`
2. Make your changes
3. Open a pull request against `main`

For large or breaking changes, please open an issue first to discuss the approach before writing code.

## Local Dev Setup

```sh
git clone https://github.com/isacssw/canopy
cd canopy
go build ./cmd/canopy
./canopy
```

## Code Style

- Run `go fmt ./...` before committing
- Run `go vet ./...` to catch common mistakes
- Run `golangci-lint run` if you have it installed
