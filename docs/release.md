# Release Process

Use this when cutting a Passage CLI release.

Releases are created from tags.

Pushing a tag that matches `v*` runs the GitHub Actions release workflow.

The workflow tests the CLI, builds binaries for macOS, Linux, and Windows, uploads checksums, and creates a GitHub release.

## Before Release

Check the working tree:

```sh
git status --short
```

Run the checks:

```sh
go test ./...
go build ./cmd/passage
```

Pick a version:

```text
v0.1.0
```

Use semantic versioning.

For now:

- patch for fixes;
- minor for new commands or useful behavior;
- major only for breaking CLI changes.

## Write Release Notes

Create a release notes file named after the tag:

```sh
mkdir -p .github/release-notes
cp .github/release-notes/TEMPLATE.md .github/release-notes/v0.1.0.md
```

Edit the file.

Keep notes short and user-facing.

Use this shape:

```md
# passage-cli v0.1.0

First public CLI release for passage.md.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/owainlewis/passage-cli/main/install.sh | bash
```

## Included

- Login with a Passage API token.
- Create, list, read, update, append, and replace documents.
- Share and unshare documents.
- Print raw Markdown URLs for shared documents.
- JSON output for agent and script workflows.
```

Commit the release notes before tagging.

## Tag And Push

Create an annotated tag:

```sh
git tag -a v0.1.0 -m "passage-cli v0.1.0"
```

Push the tag:

```sh
git push origin v0.1.0
```

The release workflow will create the GitHub release.

Watch it:

```sh
gh run list --workflow Release --limit 5
```

## Verify The Release

After the workflow completes:

```sh
curl -fsSL https://raw.githubusercontent.com/owainlewis/passage-cli/main/install.sh | env PASSAGE_VERSION=v0.1.0 bash
passage version
```

Or test the public install command:

```sh
curl -fsSL https://raw.githubusercontent.com/owainlewis/passage-cli/main/install.sh | bash
passage version
```

Then check the release page:

```sh
gh release view v0.1.0 --web
```
