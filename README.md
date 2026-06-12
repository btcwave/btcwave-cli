# btcwave-cli

Deterministic installer and manager for Bitcoin Wave nodes.

The CLI is the workhorse of Bitcoin Wave — it detects hardware, generates a machine-specific configuration from the [btcwave-node](https://github.com/btcwave/btcwave-node) profile, downloads and verifies Bitcoin Knots binaries, and orchestrates the full stack installation.

## Design principles

- **Deterministic**: same inputs produce the same stack
- **Idempotent and resumable**: safe to re-run at any point; persists a setup state machine
- **Agent-friendly**: `--json` flag on every command for structured output
- **Human-drivable**: works without an agent — interactive prompts guide you through

## Usage

```sh
# Initial setup with a license key
btcwave setup --key WAVE-FULL-7K3M-XXXX

# Check node status
btcwave status

# Run diagnostics
btcwave doctor

# All commands support JSON output for agents
btcwave status --json
```

## Setup flow

1. **Detect** — hardware/environment detection (CPU, RAM, disk, network)
2. **Configure** — generate bitcoin.conf from the btcwave profile, tuned for the hardware
3. **Install** — download, verify, and install Bitcoin Knots
4. **Sync** — initial block download (4–20+ hours, resumable)
5. **Seed ceremony** — human-only step, agent looks away
6. **Stack** — LND, Fulcrum, BTCPay in dependency order
7. **Complete** — MCP server installed, dashboard live

## Building

```sh
go build -o btcwave ./cmd/btcwave/
```

Cross-compile for Raspberry Pi:
```sh
GOOS=linux GOARCH=arm64 go build -o btcwave-arm64 ./cmd/btcwave/
```

## Related repos

- [btcwave-node](https://github.com/btcwave/btcwave-node) — config profile template
- [btcwave-dashboard](https://github.com/btcwave/btcwave-dashboard) — local web dashboard
- [btcwave-skill](https://github.com/btcwave/btcwave-skill) — Claude Code skill

## License

MIT
