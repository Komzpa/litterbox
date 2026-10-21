#include "timerules.h"

#include <QLocale>
#include <QTimeZone>

QString TimeRules::format(const QString &at, const QDateTime &now)
{
    const QDateTime instant = QDateTime::fromString(at, Qt::ISODate);
    if (!instant.isValid()) return {};
    const QTimeZone zone("Asia/Tbilisi");
    const QDateTime local = instant.toTimeZone(zone);
    const QDateTime reference = now.toTimeZone(zone);
    // R30: display uses the system locale. The product zone stays
    // Asia/Tbilisi for day boundaries; only the formatting locale follows
    // the system (was hardcoded Belarusian, so ru_RU still showed
    // Belarusian month names).
    const QLocale locale = QLocale::system();
    const QString clock = locale.toString(local.time(), QStringLiteral("HH:mm"));
    if (local.date() == reference.date()) return clock;
    return locale.toString(local.date(), QStringLiteral("d MMM")) + QLatin1Char(' ') + clock;
}
