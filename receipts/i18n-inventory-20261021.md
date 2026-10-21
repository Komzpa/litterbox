# i18n inventory — 2026-10-21

System locale at fix time: `LANG=be_BY.UTF-8` (user report suggested ru; pipeline covers any system locale with English fallback).

## Defect sweep (one pass, all found)

- User-visible QML/C++ strings not wrapped in `qsTr`/`tr`:
  - `qt/qml/InboxView.qml` note dialog `title: "Card note"` — raw literal, now `qsTr("Card note")`.
  - `qt/qml/InboxView.qml` window `title: "Litterbox"` — brand, intentionally unwrapped (exempt in guard test).
  - `qt/pages/*`, `qt/src/qml/CardActions.qml`: all other display strings already used `qsTr`; C++ user errors (`androidupdater.cpp`) stay `QStringLiteral` (not QML-visible, out of scope for this pass, noted).
- `.ts` missing or stale (`lupdate` counts):
  - BEFORE: all-sources baseline `239` source texts. `qt/i18n/screens_ru.ts` held only `43` finished (pages subset, no InboxView coverage); `qt/src/qml/actions_ru.ts` held `10` finished. Missing: `162` screens + `25` actions. InboxView (`~120` strings: nav, search, density, bundle, empty states, create-card) had zero Russian coverage.
  - AFTER split regeneration (`-no-obsolete`): `screens` (`qt/qml`+`qt/pages`) = `205` sources; `actions` (`qt/src/qml`) = `35` sources; total `240` (was 239: +`Card note` wrap, +`%1 email` `%n` plural replacing `%1 %2`).
  - AFTER fill: `screens_ru.ts` `205 finished / 0 unfinished`; `actions_ru.ts` `35 finished / 0 unfinished` (`lrelease` confirms both).
- `.qm` not compiled / not loaded at startup:
  - BEFORE: no `qt_add_translations` in `qt/CMakeLists.txt`, no `QTranslator`/`installTranslator` in `qt/src/main.cpp`; no `:​/i18n/*.qm` resources.
  - AFTER: `qt_add_translations(... TS_FILES i18n/screens_ru.ts i18n/screens_be.ts src/qml/actions_ru.ts src/qml/actions_be.ts RESOURCE_PREFIX "/i18n")`; `main.cpp` installs `screens`+`actions` translators for `QLocale::system()` (`ru_RU`→`ru`, `be_BY`→`be`, full name then language-only), English fallback when no catalog matches.
- Dates not localized:
  - BEFORE: `TimeRules::format` hardcoded `QLocale(Belarusian, Belarus)` + `Asia/Tbilisi` zone for formatting, so `ru_RU` still showed Belarusian month names.
  - AFTER: formatting uses `QLocale::system()`; product zone stays `Asia/Tbilisi` for day boundaries per `timerules.h` contract.
- Plurals not localized:
  - BEFORE: `emailCountText` built `qsTr("%1 %2").arg(n).arg(n==1?qsTr("email"):qsTr("emails"))` — English-only 2-form logic, breaks ru/be 3-form rules. Other bundle/change/collapse strings (`Collapse %1, %2 emails`, `%1 accounts`, `1 change waiting`/`%1 changes waiting`, important-email archive notes) use fixed singular/plural literals without `%n` (documented, not reworked in this pass except the central counter).
  - AFTER: `emailCountText` uses `qsTr("%1 email","%1 emails",value).arg(value)` with ru `numerusform` triple (`%1 письмо`/`%1 письма`/`%1 писем`); remaining fixed-plural strings translated literally and listed above for a follow-up `%n` pass.
- Mixed-language UI:
  - BEFORE: `be_BY`/`ru_RU` system showed full-English UI (no translator) with Belarusian-formatted dates and English dialog buttons.
  - AFTER: ru system shows fully Russian screens+actions; be system loads be catalog where present, English fallback elsewhere; dates follow system locale.

## Fix summary

- `qt/qml/InboxView.qml`: wrap `Card note`; `%n` plural for email count.
- `qt/src/main.cpp`: `QLocale`/`QTranslator` includes; install `screens`+`actions` translators for system locale before engine load, English fallback.
- `qt/CMakeLists.txt`: `LinguistTools` + `qt_add_translations` (ru+be, screens+actions, `/i18n`); new `tst_i18n` guard.
- `qt/src/timerules.cpp`: system-locale date formatting.
- `qt/i18n/screens_ru.ts` (205), `qt/src/qml/actions_ru.ts` (35): complete Russian translations, 0 unfinished.
- `qt/tests/tst_i18n.cpp`: fails on unwrapped user-visible QML literals (brand/technical allowlist) and on any unfinished ru entry; asserts key screens (`Inbox→Входящие`, `Settings→Настройки`, `Card note→Заметка к карточке`, `Archive in Gmail→Архивировать в Gmail`) and the 3-form `%n` triple.

## Proof

- `lrelease screens_ru → 205 finished / 0 unfinished`; `actions_ru → 35 finished / 0 unfinished`.
- `tst_i18n`: 4 passed, 0 failed.
- Xvfb client screenshots (disposable profile): see paths in task receipt below.
