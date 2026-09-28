# Litterbox Flutter app

The Android and Linux Flutter client loads cards from the Litterbox server's
`GET /v1/cards` endpoint, displays title, summary and time, and dismisses cards
with `POST /v1/cards/{id}/dismiss`. Set the server URL at build time with
`--dart-define=LITTERBOX_URL="$LITTERBOX_URL"` (use `adb reverse` for an
Android device connected to the development host).

From this directory:

```sh
flutter pub get
flutter build apk --debug --dart-define=LITTERBOX_URL="$LITTERBOX_URL"
flutter build linux --dart-define=LITTERBOX_URL="$LITTERBOX_URL"
```

For local development, run the server with a database URL and the explicit
development-only `-dev-tenant-id` flag (or `LITTERBOX_DEV_TENANT_ID`). The
cards API is not exposed unless both are configured. Do not use that tenant
flag as production authentication.
