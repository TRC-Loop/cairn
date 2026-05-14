<img src="https://github.com/TRC-Loop/cairn/blob/main/.github/cairnbanner-rawest-v1-font-embed-safe.webp"/>

<p align="center">Self-hosted uptime monitoring with incident management and status pages.</p>

<p align="center">
  <a href="https://github.com/TRC-Loop/cairn/releases"><img src="https://img.shields.io/github/v/release/TRC-Loop/cairn?sort=semver" alt="Release"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/TRC-Loop/cairn" alt="License"></a>
  <a href="go.mod"><img src="https://img.shields.io/github/go-mod/go-version/TRC-Loop/cairn" alt="Go"></a>
  <a href="https://github.com/TRC-Loop/cairn/pkgs/container/cairn"><img src="https://img.shields.io/badge/ghcr.io-cairn-blue" alt="Image"></a>
  <a href="https://github.com/TRC-Loop/cairn/actions"><img src="https://img.shields.io/github/actions/workflow/status/TRC-Loop/cairn/release.yml?branch=main" alt="Build"></a>
</p>

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
- [Contributing](CONTRIBUTING.md)
- [License](LICENSE) (AGPL-3.0-or-later)
- [Releases](https://github.com/TRC-Loop/cairn/releases)
