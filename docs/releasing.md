# Releasing

## How it works

Every push to `main` triggers `.github/workflows/release.yml`, which:

1. Runs `go test ./...`
2. Finds the highest existing `vMAJOR.MINOR.PATCH` tag and increments the patch number
3. Builds three binaries into `dist/`
4. Creates a GitHub release with that tag and attaches the binaries

The first release will be `v1.0.0`. Subsequent pushes produce `v1.0.1`, `v1.0.2`, etc.

## Bumping major or minor version

The workflow always increments the patch of the **highest existing tag**. To start a new
major or minor series, manually create and push a tag before your next push to `main`:

```bash
git tag v1.1.0
git push origin v1.1.0
```

The next push to `main` will then produce `v1.1.1`, `v1.1.2`, etc.

## Artifacts

| File                            | Platform              |
| ------------------------------- | --------------------- |
| claude-boot-darwin-arm64        | macOS (Apple Silicon) |
| claude-boot-linux-amd64         | Linux x86-64          |
| claude-boot-windows-amd64.exe   | Windows x86-64        |
| checksums.txt                   | SHA-256 checksums     |

Verify a download:

```bash
sha256sum -c checksums.txt
```

## Local cross-compile

```bash
make dist   # builds the same three binaries into dist/
```
