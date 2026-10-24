# Blinkit web notes (captured 2025-12-20)

These notes are from inspecting the logged-in web session via Chrome MCP.
Do NOT store real tokens/cookies in the repo.

## Login state indicators (UI + storage)
- UI: account menu shows phone number and `Logout` entry.
  - Orders page: `https://blinkit.com/account/orders`.
  - Nav link selector: `a[href="/account/orders"]` (also matches full URL).
  - Logout text appears in account nav.
- Storage:
  - `localStorage.auth` JSON contains `accessToken` and `phoneNumber`.
  - `localStorage.authKey` string.
  - `localStorage.deviceId` string.
  - `sessionStorage.sessionId` string.
  - Cookie `gr_1_accessToken` contains URL-encoded access token.

## Order history endpoint
- **POST** `https://blinkit.com/v1/layout/order_history`
  - Request body: empty (content-length 0).
  - Required headers observed:
    - `access_token` (matches `localStorage.auth.accessToken`)
    - `auth_key` (matches `localStorage.authKey` or `/v2/accounts/auth_key/`)
    - `device_id` (matches `localStorage.deviceId`)
    - `session_uuid` (matches `sessionStorage.sessionId`)
    - `lat`, `lon` (from location selection)
    - `app_client: consumer_web`, `platform: desktop_web`, `web_app_version`, `app_version`, `rn_bundle_version`
  - Cookies sent (not exhaustive):
    - `gr_1_deviceId`, `gr_1_accessToken`, `gr_1_lat`, `gr_1_lon`, `gr_1_locality`, `gr_1_landmark`
  - Response shape (high-level):
    - `is_success: true`
    - `response.snippets[]` (list of UI widgets)
      - `widget_type: order_history_container_vr` contains one order card.
      - `data.items[]` includes:
        - `image_text_vr_type_header` with title (`Arrived in X minutes`),
          left_underlined_subtitle (`₹...`), subtitle (`DD Mon, HH:MM am/pm`)
        - `horizontal_list` with `horizontal_item_list[]` where
          product name is in `image.accessibility_text.text`.
        - `vertical_text_image_snippet` with bottom CTA `Reorder`.
      - `tracking.common_attributes` includes:
        - `order_id`, `order_status`, and `deeplink` containing `order_id` and `cart_id`.

## Auth key endpoint
- **GET** `https://blinkit.com/v2/accounts/auth_key/`
  - Response: `{ "success": true, "auth_key": "..." }`
  - Requires cookie `gr_1_deviceId`.

## Order count endpoint
- **GET** `https://blinkit.com/v1/order_count`
  - Response: `{ data: { user:<id>: { order_traits_realtime: { delivered_orders, live_orders, cancelled_orders }}}}`
  - Uses same auth headers as order_history.

## Pagination / filters
- Cursor-based: `response.pagination.next_url` holds the next page path
  (`/v1/layout/order_history?offset=0&limit=10&cursor=...`); absent on the last page.
- POST it with an empty body; 10 orders per page. `page`/`page_size` body fields are ignored.
- `/v1/order_count` undercounts (reported 17 delivered vs 41 actual on 2026-10-24); don't rely on it.

## Date formats (subtitle of `image_text_vr_type_header`)
- `Today, 10:54 am` / `Yesterday, ...`
- `09 Oct, 9:32 am` (current year, no year shown)
- `19 Oct 2025` (older years, no time)

## Cloudflare TLS fingerprinting (2026-10-24)
- Same code got `403` when built with Go 1.25.5 and `200` with Go 1.27.3.
- Cloudflare scores the TLS ClientHello fingerprint (JA4); Go 1.25's is flagged as a bot.
  - go1.25.5: `t13d1312h2_f57a46bbacb6_ab7e3b40a677`
  - go1.27.3: `t13d1312h2_f57a46bbacb6_f50d94e863eb`
- Headless Chrome is challenged ("Just a moment...") while its UA says `HeadlessChrome`.
- `sync` therefore runs `fetch()` inside a headless Chrome tab with the UA override
  (`internal/blink/browser.go`), so the Go version no longer matters.
- `sync --http` keeps the direct client; it only works on Go versions whose fingerprint
  isn't blocked (see `.github/workflows/release.yml`).
