# podstack

`podstack` is a thin CLI for sending and receiving large files between any two
machines, powered by [croc](https://github.com/schollz/croc). Croc is bundled
into the binary — no separate install required.

It defaults to the Podstack-operated relay
(`relay.cloud.podstack.ai:9009`) so transfers don't depend on third-party
infrastructure, but you can use croc's public relay or any custom relay you
operate.

Supports macOS and Linux (`amd64` and `arm64`).

## Install

### One-liner

```sh
curl -fsSL https://podstack.ai/install.sh | sh
```

### Homebrew

```sh
brew install podstack/tap/podstack
```

### Binary download

Download a release tarball from <https://github.com/podstack/podstack-cli/releases>
and place `podstack` on your `PATH`.

## Usage

### Send a file

```sh
podstack send ./episode-001.wav
# prints a code phrase for the receiver
```

### Send a directory or multiple paths

```sh
podstack send ./assets/ ./notes.md
```

### Send a text message

```sh
podstack send --text "see you at 3pm"
```

### Receive

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

### Pick a different relay

```sh
# croc's public relay
podstack send --relay-default ./big.zip

# custom relay
podstack send --relay myrelay.example.com:9009 ./big.zip

# via environment
PODSTACK_RELAY=myrelay.example.com podstack send ./big.zip
```

Precedence: `--relay` > `--relay-default` > `PODSTACK_RELAY` env > default
(`relay.cloud.podstack.ai`).

### Resume

If a transfer is interrupted, re-run the same `podstack receive` command in the
same output directory. Croc detects the partial file by hash and resumes
from where it left off.

## License

MIT — see [LICENSE](./LICENSE).
