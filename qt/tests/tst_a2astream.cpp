#include "a2astream.h"
#include "api.h"

#include <QElapsedTimer>
#include <QQmlComponent>
#include <QQmlContext>
#include <QQmlEngine>
#include <QTcpServer>
#include <QTcpSocket>
#include <QtTest>
#include <memory>

namespace {
QStringList logMessages;
void captureLog(QtMsgType, const QMessageLogContext &, const QString &message)
{
    logMessages.append(message);
}

class Fixture : public QObject
{
public:
    QTcpServer server;
    QList<QByteArray> requests;
    QPointer<QTcpSocket> stream;
    QByteArray status = "200 OK";
    qint64 nextAfter = 7;

    Fixture()
    {
        connect(&server, &QTcpServer::newConnection, this, [this] {
            auto *socket = server.nextPendingConnection();
            connect(socket, &QTcpSocket::disconnected, socket, &QObject::deleteLater);
            connect(socket, &QTcpSocket::readyRead, this, [this, socket, buffer = QByteArray()]() mutable {
                buffer.append(socket->readAll());
                if (!buffer.contains("\r\n\r\n")) return;
                requests.append(buffer);
                const bool events = buffer.startsWith("GET /v1/a2a/events ");
                buffer.clear();
                if (events) {
                    stream = socket;
                    socket->write("HTTP/1.1 " + status + "\r\nContent-Type: text/event-stream\r\nConnection: close\r\n\r\n");
                    if (status != "200 OK") {
                        socket->write("event: a2a\ndata: {\"seq\":42,\"echo\":\"fixture-private-bearer\"}\n\n");
                        socket->disconnectFromHost();
                    }
                } else {
                    const QByteArray body = "{\"messages\":[],\"next_after\":" + QByteArray::number(nextAfter) + "}";
                    socket->write("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: "
                        + QByteArray::number(body.size()) + "\r\nConnection: close\r\n\r\n" + body);
                    socket->disconnectFromHost();
                }
            });
        });
    }

    QString url() const { return QStringLiteral("http://127.0.0.1:%1/").arg(server.serverPort()); }
    int eventRequests() const {
        int count = 0;
        for (const auto &request : requests) if (request.startsWith("GET /v1/a2a/events ")) ++count;
        return count;
    }
};
}

class A2aStreamTest : public QObject
{
    Q_OBJECT
private slots:
    void fragmentedEventsAndNegativeControls()
    {
        Fixture fixture;
        QVERIFY(fixture.server.listen(QHostAddress::LocalHost));
        Api api;
        api.setBaseUrl(fixture.url());
        api.setToken(QStringLiteral("fixture-private-bearer"));
        A2aStream stream(&api);
        QSignalSpy changed(&stream, &A2aStream::changed);
        QSignalSpy connected(&stream, &A2aStream::reconnected);
        QTRY_COMPARE(connected.size(), 1);
        QVERIFY(fixture.requests.first().contains("Authorization: Bearer fixture-private-bearer\r\n"));
        QVERIFY(fixture.requests.first().contains("Accept: text/event-stream\r\n"));
        fixture.stream->write(": ping\r\n\r\nevent: cards\ndata: {\"seq\":1}\n\nevent: a2a\ndata: broken\n\n"
                              "event: a2a\ndata: {\"seq\":-1}\n\nevent: a2a\ndata: {\"seq\":1.5}\n\n"
                              "event: a2a\r\ndata: {\"seq\":\r\n");
        QTest::qWait(80);
        QCOMPARE(changed.size(), 0);
        fixture.stream->write("data: 4294967297}\r\n\r");
        QTest::qWait(80);
        QCOMPARE(changed.size(), 0);
        fixture.stream->write("\n");
        QTRY_COMPARE(changed.size(), 1);
        QCOMPARE(changed.first().first().toLongLong(), qint64(4294967297));
        // Event type must not leak across frame boundaries.
        fixture.stream->write("data: {\"seq\":99}\n\n");
        QTest::qWait(80);
        QCOMPARE(changed.size(), 1);
    }

    void droppedStreamReconnectsAndPageRefetchesItsCursor()
    {
        Fixture fixture;
        QVERIFY(fixture.server.listen(QHostAddress::LocalHost));
        Api api;
        api.setBaseUrl(fixture.url().chopped(1));
        api.setToken(QStringLiteral("fixture-private-bearer"));
        A2aStream stream(&api);
        QQmlEngine engine;
        api.setEngine(&engine);
        engine.rootContext()->setContextProperty("api", &api);
        engine.rootContext()->setContextProperty("a2aStream", &stream);
        QQmlComponent component(&engine);
        component.setData(R"(
            import QtQml
            QtObject {
                property int after: 0
                function refetch() {
                    api.get('/v1/a2a/threads/fixture/messages?after=' + after + '&limit=200',
                            function(error, response) { if (!error) after = response.body.next_after })
                }
                property Connections observer: Connections {
                    target: a2aStream
                    function onChanged(seq) { refetch() }
                    function onReconnected() { refetch() }
                }
            }
        )", QUrl());
        std::unique_ptr<QObject> page(component.create());
        QVERIFY2(page != nullptr, qPrintable(component.errorString()));
        QSignalSpy connected(&stream, &A2aStream::reconnected);
        QTRY_COMPARE(page->property("after").toInt(), 7);
        fixture.stream->write("event: a2a\ndata: {\"seq\":999}\n\n");
        QTRY_COMPARE(fixture.requests.size(), 3);
        QVERIFY(fixture.requests.last().startsWith("GET /v1/a2a/threads/fixture/messages?after=7&limit=200 "));
        QTRY_COMPARE(page->property("after").toInt(), 7);
        fixture.nextAfter = 9;
        QElapsedTimer elapsed;
        elapsed.start();
        fixture.stream->disconnectFromHost();
        QTRY_COMPARE_WITH_TIMEOUT(fixture.eventRequests(), 2, 12000);
        QVERIFY(elapsed.elapsed() >= 900);
        QVERIFY(elapsed.elapsed() < 11000);
        QTRY_COMPARE(connected.size(), 2);
        QTRY_COMPARE(page->property("after").toInt(), 9);
        QVERIFY(fixture.requests.last().startsWith("GET /v1/a2a/threads/fixture/messages?after=7&limit=200 "));
    }

    void repeatedFailuresBackOffAndCapAtTenSeconds()
    {
        Fixture fixture;
        QVERIFY(fixture.server.listen(QHostAddress::LocalHost));
        fixture.status = "503 Service Unavailable";
        Api api;
        api.setBaseUrl(fixture.url());
        api.setToken(QStringLiteral("fixture-private-bearer"));
        A2aStream stream(&api);
        QTRY_COMPARE(fixture.eventRequests(), 1);
        const QList<int> expectedSeconds{1, 2, 4, 8, 10, 10};
        int requests = 1;
        for (int seconds : expectedSeconds) {
            QElapsedTimer elapsed;
            elapsed.start();
            ++requests;
            QTRY_COMPARE_WITH_TIMEOUT(fixture.eventRequests(), requests, 12000);
            // Qt's coarse timer and the fixture polling can shift boundaries.
            QVERIFY(elapsed.elapsed() >= seconds * 900);
            QVERIFY(elapsed.elapsed() < seconds * 1000 + 1500);
        }
    }

    void deniedStreamNeverEmitsOrLogsBearerAndStopCancelsRetry()
    {
        Fixture fixture;
        QVERIFY(fixture.server.listen(QHostAddress::LocalHost));
        fixture.status = "401 Unauthorized";
        Api api;
        api.setBaseUrl(fixture.url());
        A2aStream stream(&api);
        QSignalSpy changed(&stream, &A2aStream::changed);
        QSignalSpy connected(&stream, &A2aStream::reconnected);
        QTest::qWait(80);
        QCOMPARE(fixture.eventRequests(), 0); // No token: no anonymous stream.
        logMessages.clear();
        const auto previous = qInstallMessageHandler(captureLog);
        api.setToken(QStringLiteral("fixture-private-bearer"));
        QTest::qWait(300);
        stream.stop();
        QTest::qWait(1200);
        qInstallMessageHandler(previous);
        QCOMPARE(fixture.eventRequests(), 1);
        QCOMPARE(changed.size(), 0);
        QCOMPARE(connected.size(), 0);
        for (const auto &message : logMessages) QVERIFY(!message.contains("fixture-private-bearer"));
        fixture.status = "200 OK";
        stream.start();
        QTRY_COMPARE(connected.size(), 1);
        api.setToken(QString());
        QTRY_VERIFY(fixture.stream.isNull() || fixture.stream->state() == QAbstractSocket::UnconnectedState);
        QTest::qWait(1200);
        QCOMPARE(fixture.eventRequests(), 2);
    }
};

QTEST_GUILESS_MAIN(A2aStreamTest)
#include "tst_a2astream.moc"
