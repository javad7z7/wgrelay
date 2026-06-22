# WgRelay

A cross-platform desktop app (macOS + Windows) that connects to a **WireGuard**
config and exposes it as a local **SOCKS5** and **HTTP** proxy — instead of
routing all your system traffic like the official WireGuard client does.

Only the apps you point at `127.0.0.1:<port>` go through the VPN. Everything
else on your machine is untouched. No admin/root, no `utun`/TUN driver, no
routing-table changes — the tunnel runs entirely in userspace
(`wireguard-go` + gVisor netstack).

## Prerequisites

- [Node.js](https://nodejs.org) 18+
- [Go](https://go.dev/dl/) 1.23+ (only needed to build the sidecar)

## Develop / run locally

```sh
npm install
npm start        # builds the sidecar for your OS, then launches Electron
```

## Package installers

```sh
npm run dist        # current OS
npm run dist:mac    # .dmg + .zip (arm64 + x64)
npm run dist:win    # NSIS installer + portable .exe (x64)
```

Output lands in `release/`. Note: you can only build macOS artifacts on macOS,
and Windows artifacts on Windows (or via the CI below).

## Publish GitHub Releases (CI)

`.github/workflows/release.yml` builds on both `macos-latest` and
`windows-latest` and uploads artifacts to a **draft** GitHub Release. It is
already wired to publish to `javad7z7/wgrelay` (see `build.publish` in
`package.json`). The built-in `GITHUB_TOKEN` is used automatically — no extra
secrets needed for plain (unsigned) releases.

To cut a release, bump the `version` in `package.json`, commit, then push a
matching tag:

```sh
git tag v0.2.0
git push origin v0.2.0
```

CI creates the draft with the `.dmg`, `.zip`, `.exe`, and the auto-update
metadata (`latest*.yml`). Review it on GitHub and hit Publish.

## Installing a release (unsigned)

These builds are **not code-signed**, so a freshly downloaded app shows a
security warning on first launch. Clear it once:

- **macOS**: right-click the app → **Open** → **Open**. If macOS says the app
  is *"damaged and can't be opened"*, the download was quarantined; clear it
  with:
  ```sh
  xattr -cr /Applications/WgRelay.app
  ```
- **Windows**: on the SmartScreen dialog, click **More info → Run anyway**.

## Usage

1. Click **browse** and select a standard WireGuard `.conf`.
2. Set the SOCKS5 / HTTP ports (defaults `1080` / `8080`).
3. **connect**. Point your apps at the proxy:

```sh
curl --socks5-hostname 127.0.0.1:1080 https://ifconfig.me
curl -x http://127.0.0.1:8080         https://ifconfig.me
```

Settings persist as JSON in Electron's `userData` directory
(`settings.json`).

## Limitations

- **TCP only** (SOCKS5 CONNECT + HTTP/HTTPS) — covers browsers, curl, most
  apps. No UDP-over-SOCKS.
- This is the same approach as [`wireproxy`](https://github.com/pufferffish/wireproxy);
  the sidecar is a compact implementation you fully own.

## License

MIT — see [`LICENSE`](./LICENSE).
