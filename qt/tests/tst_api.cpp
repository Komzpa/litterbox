#include "api.h"

#include <QJSEngine>
#include <QTcpServer>
#include <QTcpSocket>
#include <QtTest/QtTest>
#include <QTimer>
#include <QElapsedTimer>
#include <QPointer>
#include <memory>

class ApiTest : public QObject {
    Q_OBJECT
private slots:
    void completeJsonWithBrokenFraming_data() {
        QTest::addColumn<bool>("truncated");
        QTest::newRow("clean-200") << false;
        QTest::newRow("truncated-200") << true;
    }
    void completeJsonWithBrokenFraming() {
        QFETCH(bool, truncated);
        QTcpServer server;
        QVERIFY(server.listen(QHostAddress::LocalHost));
        connect(&server, &QTcpServer::newConnection, &server, [&] {
            auto *socket = server.nextPendingConnection();
            auto buffer = std::make_shared<QByteArray>();
            connect(socket, &QTcpSocket::readyRead, socket, [&, socket, buffer] {
                buffer->append(socket->readAll());
                if (!buffer->contains("\r\n\r\n")) return;
                const QByteArray body = "{\"ack\":\"exit-123-45\"}";
                socket->write("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: "
                    + QByteArray::number(body.size() + (truncated ? 100 : 0))
                    + "\r\nConnection: close\r\n\r\n" + body);
                socket->disconnectFromHost();
            });
            connect(socket, &QTcpSocket::disconnected, socket, &QObject::deleteLater);
        });
        Api api;
        QJSEngine engine;
        api.setEngine(&engine);
        api.setBaseUrl(QStringLiteral("http://127.0.0.1:%1").arg(server.serverPort()));
        engine.evaluate("var called = false; var errorSeen = false; var responseSeen = false;");
        QSignalSpy finished(&api, &Api::requestFinished);
        QSignalSpy failed(&api, &Api::requestFailed);
        const int id = api.get("/ack", engine.evaluate(
            "(function(error, response) { called = true; errorSeen = !!error; responseSeen = !!response; })"));
        QTRY_VERIFY(engine.globalObject().property("called").toBool());
        QCOMPARE(engine.globalObject().property("errorSeen").toBool(), truncated);
        QCOMPARE(engine.globalObject().property("responseSeen").toBool(), !truncated);
        QCOMPARE(failed.size(), truncated ? 1 : 0);
        QCOMPARE(finished.size(), truncated ? 0 : 1);
        const auto result = truncated ? failed.first() : finished.first();
        QCOMPARE(result.at(0).toInt(), id);
        QCOMPARE(result.at(truncated ? 1 : 2).toInt(), 200);
        if (truncated) QVERIFY(!result.at(2).toString().isEmpty());
        else QCOMPARE(qvariant_cast<QJsonValue>(result.at(1)).toObject().value("ack").toString(), QString("exit-123-45"));
    }
    void stalledBodyAbortsActualReplyWithoutTimingOutMetadata() {
        QTcpServer server;
        QVERIFY(server.listen(QHostAddress::LocalHost, 0));
        QHash<QByteArray, QPointer<QTcpSocket>> sockets;
        connect(&server, &QTcpServer::newConnection, &server, [&] {
            while (server.hasPendingConnections()) {
                auto *socket = server.nextPendingConnection();
                auto bytes = std::make_shared<QByteArray>();
                connect(socket, &QTcpSocket::readyRead, socket, [&, socket, bytes] {
                    bytes->append(socket->readAll());
                    if (!bytes->contains("\r\n\r\n")) return;
                    sockets.insert(bytes->left(bytes->indexOf("\r\n")).split(' ').value(1), socket);
                    // Hold both real HTTP requests open without sending bytes.
                });
                connect(socket, &QTcpSocket::disconnected, socket, &QObject::deleteLater);
            }
        });
        Api api;
        api.setBaseUrl(QStringLiteral("http://127.0.0.1:%1").arg(server.serverPort()));
        QSignalSpy failed(&api, &Api::requestFailed);
        QSignalSpy finished(&api, &Api::requestFinished);
        QElapsedTimer elapsed;
        elapsed.start();
        const int bodyId = api.get(QStringLiteral("/v1/cards/stalled/body"));
        const int metadataId = api.get(QStringLiteral("/v1/cards"));
        QTRY_COMPARE(sockets.size(), 2);
        QTRY_COMPARE_WITH_TIMEOUT(failed.size(), 1, 65000);
        // Allow Qt timer precision variance, but reject a shortened retry delay.
        QVERIFY(elapsed.elapsed() >= 55000);
        QCOMPARE(failed.first().at(0).toInt(), bodyId);
        QCOMPARE(failed.first().at(1).toInt(), 0);
        QCOMPARE(finished.size(), 0);
        QTRY_VERIFY(sockets.value("/v1/cards/stalled/body").isNull() ||
            sockets.value("/v1/cards/stalled/body")->state() == QAbstractSocket::UnconnectedState);
        // Neighboring negative control: non-body GET has no new timeout and
        // remains a usable live request after the body transfer was aborted.
        auto metadata = sockets.value("/v1/cards");
        QVERIFY(metadata);
        QCOMPARE(metadata->state(), QAbstractSocket::ConnectedState);
        metadata->write("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 2\r\nConnection: close\r\n\r\n{}");
        metadata->disconnectFromHost();
        QTRY_COMPARE(finished.size(), 1);
        QCOMPARE(finished.first().at(0).toInt(), metadataId);
        QCOMPARE(failed.size(), 1);
    }
    void listThenOpenChatDuringBodyPrefetch() {
        QTcpServer server;
        QVERIFY(server.listen(QHostAddress::LocalHost, 0));
        connect(&server, &QTcpServer::newConnection, &server, [&] {
            auto *socket = server.nextPendingConnection();
            connect(socket, &QTcpSocket::readyRead, socket, [socket] {
                const QByteArray request = socket->readAll();
                if (!request.contains("\r\n\r\n")) return;
                const bool background = request.contains("/body ");
                const QByteArray payload = background ? "{}"
                    : request.contains("/messages?") ? "{\"messages\":[{\"text\":\"reply\"}]}"
                    : "{\"threads\":[{\"id\":\"claude\"}]}";
                QTimer::singleShot(background ? 1000 : 0, socket, [socket, payload] {
                    socket->write("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: "
                        + QByteArray::number(payload.size()) + "\r\nConnection: close\r\n\r\n" + payload);
                    socket->disconnectFromHost();
                });
            });
            connect(socket, &QTcpSocket::disconnected, socket, &QObject::deleteLater);
        });
        Api api;
        QJSEngine engine;
        api.setEngine(&engine);
        api.setBaseUrl(QStringLiteral("http://127.0.0.1:%1").arg(server.serverPort()));
        engine.evaluate("var listed = false; var messages = null; var failure = null;");
        api.get(QStringLiteral("/v1/a2a/threads"), engine.evaluate(
            "(function(error, response) { failure = error; listed = response.body.threads[0].id === 'claude'; })"));
        QTRY_VERIFY(engine.globalObject().property("listed").toBool());
        // CardStore's signal-only body prefetch must not bury a newly opened chat.
        for (int i = 0; i < 24; ++i)
            api.get(QStringLiteral("/v1/cards/%1/body").arg(i));
        QTest::qWait(100); // Occupy the HTTP/1 connection slots before opening the chat.
        api.get(QStringLiteral("/v1/a2a/threads/claude/messages?after=0&limit=200"), engine.evaluate(
            "(function(error, response) { failure = error; messages = response.body.messages; })"));
        QTRY_VERIFY_WITH_TIMEOUT(!engine.globalObject().property("messages").isNull(), 2000);
        QVERIFY(engine.globalObject().property("failure").isNull());
        QCOMPARE(engine.evaluate("messages[0].text").toString(), QStringLiteral("reply"));
    }
    void structuredFailureBodySurvivesTransport() {
        QTcpServer server;
        QVERIFY(server.listen(QHostAddress::LocalHost, 0));
        connect(&server, &QTcpServer::newConnection, &server, [&] {
            auto *socket = server.nextPendingConnection();
            connect(socket, &QTcpSocket::readyRead, socket, [socket] {
                socket->readAll();
                const QByteArray payload = "{\"code\":\"gmail_reconnect_required\",\"account_address\":\"expired@example.test\"}";
                socket->write("HTTP/1.1 422 Unprocessable Entity\r\nContent-Type: application/json\r\nContent-Length: "
                    + QByteArray::number(payload.size()) + "\r\nConnection: close\r\n\r\n" + payload);
                socket->disconnectFromHost();
            });
        });
        Api api;
        api.setBaseUrl(QStringLiteral("http://127.0.0.1:%1").arg(server.serverPort()));
        QSignalSpy failed(&api, &Api::requestFailed);
        api.post(QStringLiteral("/v1/ops"), {});
        QTRY_COMPARE(failed.size(), 1);
        QCOMPARE(failed.first().at(1).toInt(), 422);
        const QJsonObject body = qvariant_cast<QJsonValue>(failed.first().at(3)).toObject();
        QCOMPARE(body.value("code").toString(), QString("gmail_reconnect_required"));
        QCOMPARE(body.value("account_address").toString(), QString("expired@example.test"));
    }
    void bearerAndEmptySuccessResponses() {
        QTcpServer server;
        QVERIFY(server.listen(QHostAddress::LocalHost, 0));
        QList<QByteArray> requests;
        connect(&server, &QTcpServer::newConnection, &server, [&] {
            QTcpSocket *socket = server.nextPendingConnection();
            auto *buffer = new QByteArray;
            connect(socket, &QTcpSocket::readyRead, socket, [&, socket, buffer] {
                buffer->append(socket->readAll());
                const qsizetype end = buffer->indexOf("\r\n\r\n");
                if (end < 0) return;
                const QByteArray headers = buffer->left(end);
                const qsizetype lengthPos = headers.indexOf("Content-Length:");
                int length = 0;
                if (lengthPos >= 0) length = headers.mid(lengthPos + 15).split('\n').first().trimmed().toInt();
                if (buffer->size() < end + 4 + length) return;
                requests.append(*buffer);
                const bool get = headers.startsWith("GET ");
                const bool denied = headers.contains("/denied");
                const QByteArray payload = denied ? "unauthorized" : get ? "{\"now\":[],\"later\":[],\"missed\":[]}" : "";
                const QByteArray status = denied ? "401 Unauthorized" : get ? "200 OK" : "204 No Content";
                socket->write("HTTP/1.1 " + status + "\r\nContent-Type: application/json\r\nContent-Length: "
                    + QByteArray::number(payload.size()) + "\r\nConnection: close\r\n\r\n" + payload);
                socket->disconnectFromHost();
                delete buffer;
            });
        });
        Api api;
        QJSEngine engine;
        api.setEngine(&engine);
        api.setBaseUrl(QStringLiteral("http://127.0.0.1:%1").arg(server.serverPort()));
        api.setToken(QStringLiteral("secret-token"));
        engine.evaluate("var lastError = null; var lastResponse = null;");
        const QJSValue callback = engine.evaluate("(function(error,response) { lastError = error ? error.message : null; lastResponse = response; })");
        QVERIFY(callback.isCallable());
        QSignalSpy finished(&api, &Api::requestFinished);
        QSignalSpy failed(&api, &Api::requestFailed);

        api.get(QStringLiteral("/v1/cards"), callback);
        QTRY_COMPARE(requests.size(), 1);
        QTRY_VERIFY(finished.size() + failed.size() > 0);
        QVERIFY2(failed.isEmpty(), failed.isEmpty() ? "" : qPrintable(failed.first().at(2).toString()));
        QTRY_COMPARE(engine.globalObject().property("lastResponse").property("status").toInt(), 200);
        QCOMPARE(engine.globalObject().property("lastResponse").property("body").property("now").toString(), QString());
        QVERIFY(requests.constLast().contains("Authorization: Bearer secret-token"));
        QVERIFY(requests.constLast().contains("X-Litterbox-Api: 1"));

        api.post(QStringLiteral("/v1/cards/id/dismiss"), {}, callback);
        QTRY_COMPARE(engine.globalObject().property("lastResponse").property("status").toInt(), 204);
        QVERIFY(engine.globalObject().property("lastResponse").property("body").toBool());
        QVERIFY(requests.constLast().startsWith("POST "));

        api.del(QStringLiteral("/v1/example"), callback);
        QTRY_COMPARE(requests.size(), 3);
        QVERIFY(requests.constLast().startsWith("DELETE "));

        api.get(QStringLiteral("/denied"), callback);
        QTRY_COMPARE(requests.size(), 4);
        QTRY_VERIFY(engine.globalObject().property("lastResponse").isNull());
        QCOMPARE(failed.constLast().at(1).toInt(), 401);
        QVERIFY(!engine.globalObject().property("lastError").toString().isEmpty());
    }
    void topLevelArrayBodyIsJsArray() {
        QTcpServer server;
        QVERIFY(server.listen(QHostAddress::LocalHost, 0));
        connect(&server, &QTcpServer::newConnection, &server, [&] {
            QTcpSocket *socket = server.nextPendingConnection();
            auto *buffer = new QByteArray;
            connect(socket, &QTcpSocket::readyRead, socket, [&, socket, buffer] {
                buffer->append(socket->readAll());
                if (buffer->indexOf("\r\n\r\n") < 0) return;
                const bool empty = buffer->contains("/empty");
                const QByteArray payload = empty ? "[]" : "[{\"id\":\"x\",\"address\":\"a@b.c\"}]";
                socket->write("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: "
                    + QByteArray::number(payload.size()) + "\r\nConnection: close\r\n\r\n" + payload);
                socket->disconnectFromHost();
                delete buffer;
            });
        });
        Api api;
        QJSEngine engine;
        api.setEngine(&engine);
        api.setBaseUrl(QStringLiteral("http://127.0.0.1:%1").arg(server.serverPort()));
        engine.evaluate("var lastError = null; var lastResponse = null;");
        const QJSValue callback = engine.evaluate("(function(error,response) { lastError = error ? error.message : null; lastResponse = response; })");
        QVERIFY(callback.isCallable());

        api.get(QStringLiteral("/v1/gmail/accounts"), callback);
        QTRY_VERIFY(!engine.globalObject().property("lastResponse").isNull());
        QVERIFY(engine.globalObject().property("lastError").isNull());
        QCOMPARE(engine.globalObject().property("lastResponse").property("status").toInt(), 200);
        QVERIFY2(engine.evaluate("Array.isArray(lastResponse.body)").toBool(), "top-level object array must be a JS Array");
        QCOMPARE(engine.evaluate("lastResponse.body.length").toInt(), 1);
        QCOMPARE(engine.evaluate("lastResponse.body[0].address").toString(), QString("a@b.c"));

        engine.evaluate("lastResponse = null; lastError = null;");
        api.get(QStringLiteral("/empty"), callback);
        QTRY_VERIFY(!engine.globalObject().property("lastResponse").isNull());
        QVERIFY2(engine.evaluate("Array.isArray(lastResponse.body)").toBool(), "empty list must still be a JS Array");
        QCOMPARE(engine.evaluate("lastResponse.body.length").toInt(), 0);
    }
};
QTEST_MAIN(ApiTest)
#include "tst_api.moc"
