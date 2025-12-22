# blinkcli

An **unofficial** command-line tool for Blinkit order history.
Not affiliated with Blinkit or Blink Commerce Private Limited.

## Install

Build from source:

```bash
go build -o blinkcli ./cmd/blinkcli
```

Homebrew (once releases are published):

```bash
brew tap maheshrijal/tap
brew install blinkcli
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

This is a personal/community project. It uses internal Blinkit web endpoints
and may break if the site changes.
