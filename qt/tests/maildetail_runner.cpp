#include "maildocumentprofile.h"

#include <QElapsedTimer>
#include <QFile>
#include <QQmlContext>
#include <QQmlEngine>
#include <QTimer>
#include <QtQuickTest/quicktest.h>
#include <QtWebEngineQuick>

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
            m_longest = qMax(m_longest, now - m_last);
            m_last = now;
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
        m_clock.start();
        m_timer.start();
    }
    Q_INVOKABLE double stopHeartbeat()
    {
        m_longest = qMax(m_longest, m_clock.nsecsElapsed() - m_last);
        m_timer.stop();
        return m_longest / 1000000.0;
    }
private:
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
    }
};

int main(int argc, char **argv)
{
    QCoreApplication::setAttribute(Qt::AA_ShareOpenGLContexts);
    MailDocumentProfile::registerScheme();
    QtWebEngineQuick::initialize();
    qmlRegisterType<MailDocumentProfile>("Litterbox.Mail", 1, 0, "MailDocumentProfile");
    MailTestSetup setup;
    return quick_test_main_with_setup(argc, argv, "maildetail", nullptr, &setup);
}

#include "maildetail_runner.moc"
