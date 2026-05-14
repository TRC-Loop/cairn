<img src="https://github.com/TRC-Loop/cairn/blob/main/.github/cairnbanner-rawest-v1-font-embed-safe.webp"/>

<p align="center">Self-hosted uptime monitoring with incident management and status pages.</p>

<p align="center">
  <a href="https://github.com/TRC-Loop/cairn/releases"><img src="https://img.shields.io/github/v/release/TRC-Loop/cairn?sort=semver&style=for-the-badge&color=7DD3FC&labelColor=161B22" alt="Release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/TRC-Loop/cairn?style=for-the-badge&color=7DD3FC&labelColor=161B22" alt="License"></a>
  <a href="go.mod"><img src="https://img.shields.io/github/go-mod/go-version/TRC-Loop/cairn?style=for-the-badge&color=7DD3FC&labelColor=161B22&logo=go&logoColor=7DD3FC" alt="Go"></a>
  <a href="https://github.com/TRC-Loop/cairn/pkgs/container/cairn"><img src="https://img.shields.io/badge/ghcr.io-cairn-7DD3FC?style=for-the-badge&labelColor=161B22&logo=docker&logoColor=7DD3FC" alt="Image"></a>
  <a href="https://github.com/TRC-Loop/cairn/actions"><img src="https://img.shields.io/github/actions/workflow/status/TRC-Loop/cairn/release.yml?branch=main&style=for-the-badge&color=7DD3FC&labelColor=161B22" alt="Build"></a>
</p>

<p align="center">
  <a href="#run">Run</a> ·
  <a href="#features">Features</a> ·
  <a href="https://cairn.arne.sh">Docs</a> ·
  <a href="https://status.arne.sh">Showcase</a>
</p>

## Features

- HTTP, TCP, and push-based monitors
- Incidents with auto-open on failure, auto-resolve, and configurable reopen behaviour
- Status pages with components, custom domains, password protection, and embeddable widgets
- Scheduled maintenance windows that suppress alerts and show on the status page
- Notifications via webhook, email, Slack, and Discord
- 30-day history grid and 90-day component history on the public page
- Single Go binary, single SQLite database, no external services required

## Run

```sh
docker run -d \
  --name cairn \
  -p 8080:8080 \
  -v cairn-data:/data \
  -e CAIRN_ENCRYPTION_KEY="$(openssl rand -base64 32)" \
  ghcr.io/trc-loop/cairn:latest
```

Then open http://localhost:8080 and finish setup.

A `docker-compose.yml` is included for a more complete setup.

## Links

- Docs: https://cairn.arne.sh
- Showcase: https://status.arne.sh
- [Contributing](CONTRIBUTING.md)
- [License](LICENSE) (AGPL-3.0-or-later)
- [Releases](https://github.com/TRC-Loop/cairn/releases)
