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
- No pagination params observed on `order_history`; scrolling did not trigger a follow-up request.
- If large histories exist, pagination may be encoded in headers or a future payload field.
