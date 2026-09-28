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
    const QLocale belarusian(QLocale::Belarusian, QLocale::Belarus);
    const QString clock = belarusian.toString(local.time(), QStringLiteral("HH:mm"));
    if (local.date() == reference.date()) return clock;
    return belarusian.toString(local.date(), QStringLiteral("d MMM")) + QLatin1Char(' ') + clock;
}
