#include "timerules.h"

#include <QtTest/QtTest>

class TimeRulesTest : public QObject {
    Q_OBJECT
private slots:
    void untimedHasNoFabricatedTime() {
        const QDateTime now = QDateTime::fromString("2026-09-28T12:00:00+04:00", Qt::ISODate);
        QCOMPARE(TimeRules::format({}, now), QString());
        QCOMPARE(TimeRules::format(QStringLiteral("not a timestamp"), now), QString());
    }
    void sameDayUses24HourClock() {
        const QDateTime now = QDateTime::fromString("2026-09-28T12:00:00+04:00", Qt::ISODate);
        QCOMPARE(TimeRules::format(QStringLiteral("2026-09-28T17:05:00+04:00"), now), QStringLiteral("17:05"));
        QCOMPARE(TimeRules::format(QStringLiteral("2026-09-28T09:05:00+04:00"), now), QStringLiteral("09:05"));
    }
    void timezoneDeterminesDayBoundary() {
        const QDateTime now = QDateTime::fromString("2026-09-28T23:00:00+04:00", Qt::ISODate);
        const QString nextDay = TimeRules::format(QStringLiteral("2026-09-28T21:30:00Z"), now);
        QVERIFY(nextDay.startsWith(QStringLiteral("29 ")));
        QVERIFY(nextDay.endsWith(QStringLiteral("01:30")));
    }
};
QTEST_MAIN(TimeRulesTest)
#include "tst_timerules.moc"
