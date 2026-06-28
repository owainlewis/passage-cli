# AGENTS.md

Guidance for agents and contributors working in this repository.

## What This Is

`passage-cli` is the public Go CLI for passage.md.

It should build a binary named `passage`.

The private app repo is `owainlewis/passage.md`.

## Product Principles

- Keep terminal output plain and scriptable.
- Prefer useful text by default.
- Add JSON output where scripts and agents need stable data.
- Keep API behavior aligned with the passage.md document API contract.

## Engineering Principles

- Make the smallest complete change.
- Prefer the Go standard library unless a dependency clearly pays for itself.
- Keep command behavior easy to test without a live server.
- Do not add auth, document, or sharing behavior before the issue that owns it.

## Verification

Run these checks before opening a PR:

```sh
go test ./...
go build ./cmd/passage
```

