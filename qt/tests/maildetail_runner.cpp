#include "maildocumentprofile.h"
#ifdef LB_RESIZE_STORE
#include "CardStore.h"
#include <QTemporaryDir>
#endif

#include <QElapsedTimer>
#include <QFile>
#include <QQmlContext>
#include <QQmlEngine>
#include <QTimer>
#include <QVariantList>
#include <QtQuickTest/quicktest.h>
#include <QtWebEngineQuick>
#include <time.h>

class MailFixtures : public QObject
{
    Q_OBJECT
public:
    explicit MailFixtures(QObject *parent = nullptr) : QObject(parent)
    {
        m_timer.setTimerType(Qt::PreciseTimer);
        m_timer.setInterval(1);
        connect(&m_timer, &QTimer::timeout, this, [this] {
            const qint64 now = m_clock.nsecsElapsed();
            recordGap(now);
        });
    }
    Q_INVOKABLE QString read(const QString &name)
    {
        QFile file(QStringLiteral(LB_MAIL_FIXTURES_DIR) + QStringLiteral("/mail-") + name + QStringLiteral(".html"));
        if (!file.open(QIODevice::ReadOnly)) qFatal("Missing mail fixture");
        return QString::fromUtf8(file.readAll());
    }
    Q_INVOKABLE void startHeartbeat()
    {
        m_longest = m_last = 0;
        m_gaps.clear();
        m_clock.start();
        m_timer.start();
    }
    Q_INVOKABLE double stopHeartbeat()
    {
        recordGap(m_clock.nsecsElapsed());
        m_timer.stop();
        return m_longest / 1000000.0;
    }
    Q_INVOKABLE QVariantList heartbeatBlocks() const { return m_gaps; }
private:
    void recordGap(qint64 now)
    {
        const qint64 gap = now - m_last;
        m_longest = qMax(m_longest, gap);
        if (gap > 50000000)
            m_gaps.append(QVariantMap{{QStringLiteral("atMs"), now / 1000000.0},
                                      {QStringLiteral("gapMs"), gap / 1000000.0}});
        m_last = now;
    }
    QVariantList m_gaps;
    QTimer m_timer;
    QElapsedTimer m_clock;
    qint64 m_last = 0, m_longest = 0;
};

class MailTestSetup : public QObject
{
    Q_OBJECT
public slots:
    void qmlEngineAvailable(QQmlEngine *engine)
    {
        engine->rootContext()->setContextProperty(QStringLiteral("mailFixtures"), new MailFixtures(engine));
#ifdef LB_RESIZE_STORE
        auto *store = new CardStore(engine);
        if (!m_database.isValid() || !store->open(m_database.filePath(QStringLiteral("cache.sqlite"))))
            qFatal("Cannot open disposable resize cache");
        engine->rootContext()->setContextProperty(QStringLiteral("resizeStore"), store);
#endif
    }
private:
#ifdef LB_RESIZE_STORE
    QTemporaryDir m_database;
#endif
};

int main(int argc, char **argv)
{
    MailDocumentProfile::initialize();
    MailTestSetup setup;
    return quick_test_main_with_setup(argc, argv, "maildetail", nullptr, &setup);
}

#include "maildetail_runner.moc"
