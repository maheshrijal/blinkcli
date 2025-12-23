# blinkcli

An **unofficial** command-line tool for Blinkit order history.

## Install

Homebrew (once releases are published):

```bash
brew install --cask maheshrijal/tap/blinkcli
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

Optional flags:

```bash
blinkcli sync --pages 1 --page-size 0 --sleep-ms 350
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
