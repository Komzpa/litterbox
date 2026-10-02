#include "api.h"

#include <QJSEngine>
#include <QTcpServer>
#include <QTcpSocket>
#include <QtTest/QtTest>

class ApiTest : public QObject {
    Q_OBJECT
private slots:
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
