# Litterbox Flutter app

The Android and Linux Flutter client. The local Drift database stores inbox
cards in `lib/db/database.dart`; `lib/main.dart` displays open cards ordered by
`sort_at`.

From this directory:

```sh
flutter pub get
flutter analyze
flutter test
flutter build apk --debug
```

Drift's generated schema is `lib/db/database.g.dart`; after changing database
tables, regenerate it with:

```sh
dart run build_runner build --delete-conflicting-outputs
```
