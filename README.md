# passage-cli

Go CLI for [passage.md](https://passage.md).

`passage` will let humans and agents work with hosted Passage Markdown documents from a terminal.

This repo is public.

The app and API server live in the private `owainlewis/passage.md` repo.

## Status

This is an early Phase 2 CLI.

It currently supports local auth config, help, and version output.

Document commands, sharing, and raw Markdown URLs are tracked in later Phase 2 issues.

## Development

Requirements:

- Go 1.26 or newer.

Run tests:

```sh
go test ./...
```

Build the CLI:

```sh
go build ./cmd/passage
```

Run locally:

```sh
./passage help
./passage login
./passage auth status
./passage auth status --check
./passage version
```

Config is stored in your user config directory at `passage/config.json`.

Environment variables override saved config:

```sh
PASSAGE_API_URL=http://localhost:8080 PASSAGE_TOKEN=psg_example ./passage auth status
```

## Commands

```text
passage login
passage auth status
passage auth status --check
passage help
passage version
passage --version
```

## Releases

Release artifacts are built by GitHub Actions when a tag matching `v*` is pushed.

Example:

```sh
git tag v0.1.0
git push origin v0.1.0
```

The release workflow builds these archive names:

- `passage_<version>_darwin_amd64.tar.gz`
- `passage_<version>_darwin_arm64.tar.gz`
- `passage_<version>_linux_amd64.tar.gz`
- `passage_<version>_linux_arm64.tar.gz`
- `passage_<version>_windows_amd64.zip`
- `passage_<version>_windows_arm64.zip`

Each archive has a matching `.sha256` checksum file.

Homebrew tap support is out of scope for the MVP.
