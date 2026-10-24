# blinkcli

An **unofficial** command-line tool for Blinkit order history.

## Install

Homebrew (once releases are published):

```bash
brew install maheshrijal/tap/blinkcli
```

## Auth (browser login)

```bash
blinkcli auth login
```

A Chrome/Chromium window opens. Log in and select your address. The CLI captures
session data automatically.

Check status:

```bash
blinkcli auth status
```

Log out (local session only):

```bash
blinkcli auth logout
```

## Sync orders

```bash
blinkcli sync
```

Sync runs the request inside a headless Chrome tab, so Chrome/Chromium must be
installed. Blinkit sits behind Cloudflare, which blocks non-browser HTTP clients
by their TLS fingerprint.

Optional flags:

```bash
blinkcli sync --pages 0 --sleep-ms 350   # 0 = all pages
blinkcli sync --http   # call the API directly (no Chrome; may get 403)
```

## View orders

```bash
blinkcli orders
```

## Stats

```bash
blinkcli stats
```

## Data storage

- Config (session data):
  - macOS: `~/Library/Application Support/blinkcli/config.json`
  - Linux: `$XDG_CONFIG_HOME/blinkcli/config.json`
- Orders cache: `orders.json` in the same directory as `config.json`.

## Disclaimer

This is a personal project. It uses internal Blinkit web endpoints
and may break if the site changes. This is **unofficial** and **not affiliated**
with Blinkit. Use at your own risk. Please don’t sue me 🙏.
