# Belarusian terminology alignment with KDE compendium

Scope: Litterbox Qt/Desktop Belarusian catalogs containing `Акаўнт`:
- `qt/i18n/screens_be.ts` (224 source messages)
- `qt/src/qml/actions_be.ts` (35 source messages)

Flutter `app/lib/l10n/app_be.arb` has no `Акаўнт` strings and was left untouched.

## Compendium sources

Installed `/usr/share/locale/be/LC_MESSAGES/*.mo`, read with `msgunfmt`:
- KDE `plasma_applet_org.kde.plasma.networkmanagement.mo`: `Connect` → `Падлучыцца`, `Disconnect` → `Адлучыцца`
- KDE `plasmanetworkmanagement-kcm.mo`: `Connect` → `Падлучыцца`, `Disconnect` → `Адлучыцца`, `Connected` → `Падлучана`
- KDE `kcm_bolt.mo`: `Connected` → `Падлучана`, `Connected & Authorized` → `Падлучана і аўтарызавана`, `Authorize` → `Аўтарызаваць`
- KDE `libmessagelist6.mo` (KMail lib): `Sender` → `Адпраўнік`
- KDE `dolphin.mo`, `konsole.mo`: `Settings` → `Налады`
- KDE `kwin.mo`, `systemsettings.mo`: `Back` → `Назад`
- KDE `kcm_autostart.mo`, `kio5/6.mo`: `Refresh` → `Абнавіць`
- KDE `ark.mo`, `gwenview.mo`, `kio5/6.mo`: `Retry` → `Паўтарыць`
- KDE `dolphin.mo`, `kate.mo`: `Close` → `Закрыць`
- KDE `dolphin.mo`, `elisa.mo`: `Cancel` → `Скасаваць`
- KDE `keditbookmarks.mo`, `libktorrent6.mo`: `OK` → `Добра`
- KDE `kcm_users.mo`: `Done` → `Завершана`
- KDE `konsole.mo`, `kcm_users.mo`, `kcmkamera.mo`: `Add` → `Дадаць`
- KDE `kio5.mo`: connection noun → `злучэнне` (`Connection to Server Closed Unexpectedly` → `Злучэнне з серверам нечакана закрылася`)
- KDE `glib20.mo`: `Connect to the session/system bus` → `Падлучыцца да шыны сеанса/сістэмнай шыны`
- GNOME `gnome-online-accounts.mo` (online-account concept): `%s Account` → `Уліковы запіс %s`, `Online Accounts` → `Сеткавыя ўліковыя запісы`
- `gvfs.mo`: `Accounts are unsupported` → `Уліковыя запісы не падтрымліваюцца`
- `evolution-data-server.mo`: `Reconnecting to LDAP server...` → `Перазлучэнне з LDAP-серверам...`
- `openbox.mo` (transitive verb): `Disconnect` → `Адлучыць`
- Note: KDE `kcm_users.mo` uses `акаўнт/акаўнты`, but only for local system-user accounts (`Your Account`, `Other Accounts`). Gmail/online service accounts follow the online-account wording (`уліковы запіс`), which also resolves the «Акаўнты» complaint.
- invent.kde.org lookup was attempted for PIM/be PO files, but raw file access redirects to a sign-in wall; no PO content was retrieved. Installed compiled KDE catalogs above are the recorded compendium evidence.

## Changed strings

| English | old | KDE | source file + msgid | new |
|---|---|---|---|---|
| Gmail Accounts | Акаўнты Gmail | Уліковы запіс %s → Уліковы запіс %s | gnome-online-accounts.mo + msgid "%s Account" | Уліковыя запісы Gmail |
| Could not load the account list. | Не ўдалося загрузіць спіс акаўнтаў. | Уліковыя запісы (plural pattern) | gvfs.mo + msgid "Accounts are unsupported" | Не ўдалося загрузіць спіс уліковых запісаў. |
| Could not start the Gmail connection. | Не ўдалося пачаць падключэнне Gmail. | Злучэнне з серверам… | kio5.mo + msgid "Connection to Server Closed Unexpectedly" | Не ўдалося пачаць злучэнне з Gmail. |
| Could not disconnect the account. | Не ўдалося адключыць акаўнт. | Адлучыцца / Адлучыць | plasma networkmanagement.mo + msgid "Disconnect"; openbox.mo + msgid "Disconnect" | Не ўдалося адлучыць уліковы запіс. |
| Connect account… | Падключыць акаўнт… | Падлучыцца | plasma networkmanagement.mo + msgid "Connect" | Падлучыць уліковы запіс… |
| Connected | Падключаны | Падлучана | kcm_bolt.mo + msgid "Connected" | Падлучана |
| Reconnect | Падключыць нанова | Перазлучэнне | evolution-data-server.mo + msgid "Reconnecting to LDAP server..." | Перазлучыцца |
| Disconnect | Адключыць | Адлучыцца | plasma networkmanagement.mo + msgid "Disconnect" | Адлучыцца |
| No accounts connected. | Няма падключаных акаўнтаў. | Падлучана + Уліковыя запісы | plasmanetworkmanagement-kcm.mo + msgid "Connected"; gvfs.mo account plural | Няма падлучаных уліковых запісаў. |
| Open this authorization link … that account … another Google account … | …гэтага акаўнта … з іншым акаўнтам Google… | Уліковы запіс %s | gnome-online-accounts.mo + msgid "%s Account" | …гэтага ўліковага запісу … з іншым уліковым запісам Google… |
| Accounts | Акаўнты | Уліковыя запісы | gvfs.mo + msgid "Accounts are unsupported" | Уліковыя запісы |
| Gmail access for %1 expired. Reconnect it in Accounts → Connect account… | …Падключыце яго нанова ў Акаўнтах → Падключыць акаўнт… | Перазлучэнне; Падлучыцца; Уліковыя запісы | evolution-data-server.mo; plasma networkmanagement.mo; gvfs.mo | …Перазлучыце яго ва ўліковых запісах → Падлучыць уліковы запіс… |
| Reconnect | Падключыць нанова | Перазлучэнне | evolution-data-server.mo + msgid "Reconnecting to LDAP server..." | Перазлучыцца |
| %n account | %n акаўнт / %n акаўнты / %n акаўнтаў | Уліковы запіс / Уліковыя запісы / Уліковых запісаў | gnome-online-accounts.mo + msgid "%s Account"; gvfs.mo plural | %n уліковы запіс / %n уліковыя запісы / %n уліковых запісаў |
| Done | Гатова | Завершана | kcm_users.mo + msgid "Done" | Завершана |
| Try another subject, sender, account or source, or clear the search to return to your inbox. | …адпраўніка, акаўнт або крыніцу… | Уліковы запіс | gnome-online-accounts.mo + msgid "%s Account" | …адпраўніка, уліковы запіс або крыніцу… |
| Message unavailable. Reconnect to cache it; opening it never archives it. | …Падключыцеся, каб захаваць яго ў кэш… | Перазлучэнне | evolution-data-server.mo + msgid "Reconnecting to LDAP server..." | …Перазлучыцеся, каб захаваць яго ў кэш… |
| Account details unavailable while offline. | Даныя акаўнта недаступныя афлайн. | Уліковы запіс | gnome-online-accounts.mo + msgid "%s Account" | Даныя ўліковага запісу недаступныя афлайн. |
| Use Enroll device in the inbox menu to connect. | Каб падключыцца, … | Падлучыцца | plasma networkmanagement.mo + msgid "Connect" | Каб падлучыцца, … |
| Connected accounts | Падключаныя акаўнты | Падлучана; Уліковыя запісы | plasmanetworkmanagement-kcm.mo + msgid "Connected"; gvfs.mo | Падлучаныя ўліковыя запісы |
| No accounts connected. | Няма падключаных акаўнтаў. | Падлучана; Уліковыя запісы | plasmanetworkmanagement-kcm.mo + msgid "Connected"; gvfs.mo | Няма падлучаных уліковых запісаў. |
| Connect to Server | Падключыцца да сервера | Падлучыцца | plasma networkmanagement.mo + msgid "Connect"; glib20.mo + msgid "Connect to the session bus" | Падлучыцца да сервера |
| Can't reach the server at %1. … | Не ўдаецца падключыцца да сервера %1. … | Падлучыцца | plasma networkmanagement.mo + msgid "Connect" | Не ўдаецца падлучыцца да сервера %1. … |
| OK | ОК | Добра | keditbookmarks.mo + msgid "OK"; libktorrent6.mo + msgid "OK" | Добра |

Changed: 24 source strings (26 translation lines/forms) across `qt/i18n/screens_be.ts` (23 sources / 25 lines) and `qt/src/qml/actions_be.ts` (1 source).

## Terms verified against KDE and left unchanged

Back → Назад; Settings → Налады; Refresh → Абнавіць; Retry → Паўтарыць; Send → Адправіць; Today → Сёння; Yesterday → Учора; Task → Задача; Navigation → Навігацыя; Clear search → Ачысціць пошук; Save → Захаваць; Cancel → Скасаваць; Create → Стварыць; Title → Назва; Details → Падрабязнасці; Message → Паведамленне; Close → Закрыць; Pin → Замацаваць; Dismiss → Адхіліць; Hide → Схаваць; Never → Ніколі; More actions → Яшчэ дзеянні; Year/Month/Day/Time → Год/Месяц/Дзень/Час; Add card → Дадаць картку (Add → Дадаць); Sender photos → Фота адпраўнікоў (Sender → Адпраўнік); Authorize/Authorization → Аўтарызаваць/аўтарызацыя; sync → сінхранізацыя. Agents → Агенты and Inbox → Уваходныя have no installed KDE be counterpart and were kept.
