#pragma once

#include <QDateTime>
#include <QObject>

// Card instants are absolute RFC3339 values. Display them in the product's
// Asia/Tbilisi zone with a 24-hour Belarusian date, never fabricating a
// time for untimed cards.
class TimeRules : public QObject {
    Q_OBJECT
public:
    explicit TimeRules(QObject *parent = nullptr) : QObject(parent) {}
    Q_INVOKABLE QString display(const QString &at) const { return format(at, QDateTime::currentDateTime()); }
    static QString format(const QString &at, const QDateTime &now);
};
