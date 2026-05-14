# podstack

Official command-line interface for [podstack.ai](https://podstack.ai) — the
ML cloud platform.

This release ships peer-to-peer file transfer for moving model weights,
datasets, and other large artifacts between machines. More podstack.ai
features (model deployment, GPU lease management, run logs, billing) will
land in future releases.

Supports macOS and Linux (amd64 and arm64).

## Install

### One-liner

```sh
curl -fsSL https://github.com/Podstack-ai/podstack-cli/releases/latest/download/install.sh | sh
```

The installer detects your OS (`Darwin` / `Linux`) and CPU architecture
(`amd64` / `arm64`), downloads the matching tarball, verifies its sha256,
and drops `podstack` into `/usr/local/bin` (or `~/.local/bin` if that's not
writable).

### Binary download

Grab a release tarball from
<https://github.com/Podstack-ai/podstack-cli/releases> and place `podstack`
on your `PATH`.

## Usage

### Send a file

On the sending machine:

```sh
podstack send ./model.bin
# prints a code phrase
```

### Send a directory or multiple paths

```sh
podstack send ./checkpoints/ ./config.yaml
```

### Send a text message

```sh
podstack send --text "training finished, model.bin coming next"
```

### Receive

On the receiving machine, run the code phrase the sender printed:

```sh
podstack receive <code-phrase>
podstack receive --out ./downloads <code-phrase>
podstack receive --yes <code-phrase>   # auto-accept
```

### Custom code phrase

```sh
podstack send --code my-shared-code ./big.zip
podstack receive my-shared-code
```

### Resume

If a transfer is interrupted, re-run the same `podstack receive` command in
the same output directory. The partial file is detected by hash and the
transfer resumes where it left off.

## License

MIT — see [LICENSE](./LICENSE).
