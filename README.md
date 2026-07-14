# passage-cli

Go CLI for [passage.md](https://passage.md).

`passage` will let humans and agents work with hosted Passage Markdown documents from a terminal.

This repo is public.

The app and API server live in the private `owainlewis/passage.md` repo.

## Status

This is an early Phase 2 CLI.

It currently supports local auth config, document commands, sharing commands, help, and version output.

## Install

Install the latest release:

```sh
curl -fsSL https://raw.githubusercontent.com/owainlewis/passage-cli/main/install.sh | bash
```

Install a specific version:

```sh
curl -fsSL https://raw.githubusercontent.com/owainlewis/passage-cli/main/install.sh | env PASSAGE_VERSION=v0.1.0 bash
```

The installer supports macOS and Linux on `amd64` and `arm64`.

By default, it installs to `/usr/local/bin` when writable, otherwise `~/.local/bin`.

Set `PASSAGE_INSTALL_DIR` to choose another directory.

```sh
curl -fsSL https://raw.githubusercontent.com/owainlewis/passage-cli/main/install.sh | env PASSAGE_INSTALL_DIR="$HOME/bin" bash
```

Verify:

```sh
passage version
```

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
./passage new "Draft"
./passage list
./passage cat <doc-id>
./passage pull <doc-id>
./passage push <doc-id> ./draft.md
./passage append <doc-id> ./notes.md
./passage replace <doc-id> ./draft.md
./passage delete <doc-id>
./passage share <doc-id>
./passage raw <doc-id>
./passage unshare <doc-id>
./passage version
```

Auth config is stored at `~/.passage/auth.json`.

Environment variables override saved config:

```sh
PASSAGE_API_URL=http://localhost:8080 PASSAGE_TOKEN=psg_example ./passage auth status
```

Use `--json` with document commands when scripts need structured output:

```sh
./passage list --json
./passage cat --json <doc-id>
./passage delete --json <doc-id>
./passage share --json <doc-id>
./passage raw --json <doc-id>
```

Share output includes both `html_url` and `raw_url`.

Raw Markdown URLs are public to anyone with the link until you run `passage unshare <doc-id>`.

## Commands

```text
passage login
passage auth status
passage auth status --check
passage new "Draft"
passage list
passage cat <doc-id>
passage pull <doc-id>
passage push <doc-id> <file>
passage append <doc-id> <file>
passage replace <doc-id> <file>
passage delete <doc-id>
passage share <doc-id>
passage raw <doc-id>
passage unshare <doc-id>
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

See [docs/release.md](docs/release.md) for the full release process.
