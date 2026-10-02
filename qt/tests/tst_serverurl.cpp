#include "api.h"

#include <QtTest/QtTest>

// Server URL precedence: test profile > LB_SERVER env (non-empty) >
// QSettings server_url saved at enrollment > compiled LB_DEFAULT_SERVER_URL.
class ServerUrlTest : public QObject {
    Q_OBJECT
private slots:
    void testProfileWinsOverEverything() {
        QCOMPARE(Api::resolveBaseUrl(true, QStringLiteral("http://test:1"),
                                     QStringLiteral("http://env:2"), QStringLiteral("http://saved:3"),
                                     QStringLiteral("http://compiled:4")),
                 QStringLiteral("http://test:1"));
    }
    void envBeatsSavedAndCompiled() {
        QCOMPARE(Api::resolveBaseUrl(false, QString(), QStringLiteral("http://env:2"),
                                     QStringLiteral("http://saved:3"), QStringLiteral("http://compiled:4")),
                 QStringLiteral("http://env:2"));
    }
    void savedBeatsCompiled() {
        QCOMPARE(Api::resolveBaseUrl(false, QString(), QString(),
                                     QStringLiteral("http://saved:3"), QStringLiteral("http://compiled:4")),
                 QStringLiteral("http://saved:3"));
    }
    void compiledDefaultIsLastResortAndNeverLocalhost() {
        const QString resolved = Api::resolveBaseUrl(false, QString(), QString(), QString(),
                                                     QStringLiteral("http://192.168.100.74:8081"));
        QCOMPARE(resolved, QStringLiteral("http://192.168.100.74:8081"));
        QVERIFY(!resolved.contains(QStringLiteral("127.0.0.1")));
        QVERIFY(!resolved.contains(QStringLiteral("localhost")));
    }
    void emptyEnvFallsThroughToSaved() {
        QCOMPARE(Api::resolveBaseUrl(false, QString(), QString(),
                                     QStringLiteral("http://saved:3"), QStringLiteral("http://compiled:4")),
                 QStringLiteral("http://saved:3"));
    }
};
QTEST_MAIN(ServerUrlTest)
#include "tst_serverurl.moc"
