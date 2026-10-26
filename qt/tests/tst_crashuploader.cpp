#include "api.h"
#include "crashuploader.h"
#include <QFile>
#include <QJsonDocument>
#include <QJsonObject>
#include <QSignalSpy>
#include <QSet>
#include <QTcpServer>
#include <QTcpSocket>
#include <QTemporaryDir>
#include <QtTest>
#include <memory>

class CrashUploaderTest : public QObject {
    Q_OBJECT
private slots:
    void fullRejectedSpoolStopsDrain_data() {
        QTest::addColumn<int>("status");
        QTest::newRow("validation-rejected") << 422;
        QTest::newRow("service-unavailable") << 503;
    }
    void fullRejectedSpoolStopsDrain() {
        QFETCH(int, status);
        QTemporaryDir spool;
        QVERIFY(spool.isValid());
        const QByteArray report = "schema=1\nkind=exit\nstack=unavailable\n";
        for (int i = 0; i < 64; ++i) {
            QFile file(spool.filePath(QStringLiteral("exit-%1.txt").arg(i)));
            QVERIFY(file.open(QIODevice::WriteOnly));
            QCOMPARE(file.write(report), report.size());
        }
        QTcpServer server;
        QVERIFY(server.listen(QHostAddress::LocalHost));
        int diagnostics = 0;
        QList<QJsonObject> operations;
        int mailBodies = 0;
        connect(&server, &QTcpServer::newConnection, &server, [&] {
            auto *socket = server.nextPendingConnection();
            auto buffer = std::make_shared<QByteArray>();
            connect(socket, &QTcpSocket::readyRead, socket, [&, socket, buffer] {
                buffer->append(socket->readAll());
                const int boundary = buffer->indexOf("\r\n\r\n");
                if (boundary < 0) return;
                int length = 0;
                for (const auto &header : buffer->left(boundary).split('\n'))
                    if (header.toLower().startsWith("content-length:")) length = header.mid(15).trimmed().toInt();
                if (buffer->size() < boundary + 4 + length) return;
                const bool diagnostic = buffer->startsWith("POST /v1/diagnostics/crashes ");
                if (diagnostic) ++diagnostics;
                else if (buffer->startsWith("POST /v1/ops "))
                    operations.append(QJsonDocument::fromJson(buffer->mid(boundary + 4)).object());
                else if (buffer->startsWith("GET /v1/cards/mail-1/body ")) ++mailBodies;
                const QByteArray body = diagnostic ? "{\"error\":\"rejected\"}" : "{}";
                socket->write("HTTP/1.1 " + QByteArray::number(diagnostic ? status : 200)
                    + " Result\r\nContent-Type: application/json\r\nContent-Length: "
                    + QByteArray::number(body.size()) + "\r\nConnection: close\r\n\r\n" + body);
                socket->disconnectFromHost();
            });
            connect(socket, &QTcpSocket::disconnected, socket, &QObject::deleteLater);
        });
        Api api;
        api.setBaseUrl(QStringLiteral("http://127.0.0.1:%1").arg(server.serverPort()));
        api.setToken("enrolled-device");
        CrashUploader uploader(api);
        uploader.setDirectory(spool.path());
        QSignalSpy completed(&uploader, &CrashUploader::finished);
        QSignalSpy finished(&api, &Api::requestFinished);
        QSignalSpy failed(&api, &Api::requestFailed);
        const QJsonObject operation{{"op_id", "stable-operation"}, {"card_id", "mail-1"}, {"type", "archive"}};
        uploader.setOnline(true);
        const int mailId = api.get("/v1/cards/mail-1/body");
        const int opId = api.post("/v1/ops", operation);
        QTRY_COMPARE(completed.size(), 1);
        QTRY_COMPARE(finished.size(), 2);
        QCOMPARE(diagnostics, 1); // No immediate 64-request rejection burst.
        QCOMPARE(mailBodies, 1);
        QCOMPARE(operations.size(), 1);
        QCOMPARE(operations.first(), operation);
        QCOMPARE(failed.size(), 1);
        QVERIFY(uploader.ownsRequest(failed.first().at(0).toInt()));
        QSet<int> ordinaryIds;
        for (const auto &reply : finished) ordinaryIds.insert(reply.at(0).toInt());
        QCOMPARE(ordinaryIds, QSet<int>({mailId, opId}));
        QCOMPARE(completed.first().at(0).toInt(), 0);
        QCOMPARE(completed.first().at(1).toInt(), 1);
        // Every report remains byte-identical, not only the first rejection.
        for (int i = 0; i < 64; ++i) {
            QFile file(spool.filePath(QStringLiteral("exit-%1.txt").arg(i)));
            QVERIFY(file.open(QIODevice::ReadOnly));
            QCOMPARE(file.readAll(), report);
            QVERIFY(!QFile::exists(spool.filePath(QStringLiteral("exit-%1.sent").arg(i))));
        }
    }
    void rejectedReceiptRetainsExactBytes_data() {
        QTest::addColumn<int>("status");
        QTest::addColumn<QByteArray>("body");
        QTest::addColumn<bool>("truncated");
        QTest::addColumn<QString>("sessionChange");
        const QByteArray ack = "{\"ack\":\"exit-123-45\"}";
        QTest::newRow("truncated-matching-200") << 200 << ack << true << QString();
        QTest::newRow("wrong-ack") << 200 << QByteArray("{\"ack\":\"other\"}") << false << QString();
        QTest::newRow("malformed-json") << 200 << QByteArray("{") << false << QString();
        QTest::newRow("201-not-receipt") << 201 << ack << false << QString();
        QTest::newRow("204-not-receipt") << 204 << QByteArray() << false << QString();
        QTest::newRow("422") << 422 << ack << false << QString();
        QTest::newRow("503") << 503 << ack << false << QString();
        QTest::newRow("logout-inflight") << 200 << ack << false << QString("logout");
        QTest::newRow("token-changed-inflight") << 200 << ack << false << QString("token");
        QTest::newRow("server-changed-inflight") << 200 << ack << false << QString("server");
    }
    void rejectedReceiptRetainsExactBytes() {
        QFETCH(int, status);
        QFETCH(QByteArray, body);
        QFETCH(bool, truncated);
        QFETCH(QString, sessionChange);
        QTemporaryDir spool;
        QVERIFY(spool.isValid());
        const QByteArray original = "schema=1\nkind=exit\nstatus=11\nstack=unavailable\n";
        QFile pending(spool.filePath("exit-123-45.txt"));
        QVERIFY(pending.open(QIODevice::WriteOnly));
        QCOMPARE(pending.write(original), original.size());
        pending.close();
        QTcpServer server;
        QVERIFY(server.listen(QHostAddress::LocalHost));
        Api api;
        api.setBaseUrl(QStringLiteral("http://127.0.0.1:%1").arg(server.serverPort()));
        api.setToken("original-session");
        int requests = 0;
        connect(&server, &QTcpServer::newConnection, &server, [&] {
            auto *socket = server.nextPendingConnection();
            auto buffer = std::make_shared<QByteArray>();
            connect(socket, &QTcpSocket::readyRead, socket, [&, socket, buffer] {
                buffer->append(socket->readAll());
                const int boundary = buffer->indexOf("\r\n\r\n");
                if (boundary < 0) return;
                int length = 0;
                for (const auto &header : buffer->left(boundary).split('\n'))
                    if (header.toLower().startsWith("content-length:")) length = header.mid(15).trimmed().toInt();
                if (buffer->size() < boundary + 4 + length) return;
                ++requests;
                QVERIFY(buffer->contains("Authorization: Bearer original-session"));
                if (sessionChange == "logout") api.setToken({});
                if (sessionChange == "token") api.setToken("replacement-session");
                if (sessionChange == "server") api.setBaseUrl("http://127.0.0.1:1");
                socket->write("HTTP/1.1 " + QByteArray::number(status) + " Result\r\nContent-Type: application/json\r\nContent-Length: "
                    + QByteArray::number(body.size() + (truncated ? 100 : 0)) + "\r\nConnection: close\r\n\r\n" + body);
                socket->disconnectFromHost();
            });
            connect(socket, &QTcpSocket::disconnected, socket, &QObject::deleteLater);
        });
        CrashUploader uploader(api);
        uploader.setDirectory(spool.path());
        QSignalSpy completed(&uploader, &CrashUploader::finished);
        QSignalSpy failed(&api, &Api::requestFailed);
        uploader.setOnline(true);
        QTRY_COMPARE(completed.size(), 1);
        QCOMPARE(requests, 1);
        QCOMPARE(completed.first().at(0).toInt(), 0);
        QCOMPARE(completed.first().at(1).toInt(), 1);
        QVERIFY(pending.open(QIODevice::ReadOnly));
        QCOMPARE(pending.readAll(), original);
        QVERIFY(!QFile::exists(spool.filePath("exit-123-45.sent")));
        if (truncated) {
            QCOMPARE(failed.size(), 1);
            QCOMPARE(failed.first().at(1).toInt(), 200);
        }
    }
    void restartOfflineRejectionAndExactAuthenticatedAck() {
        QTemporaryDir spool;
        QVERIFY(spool.isValid());
        const QString pending = spool.filePath("exit-123-45.txt");
        const QByteArray original = "schema=1\npackage=org.qtproject.example.litterbox_qt\nandroid=35\n"
            "version=unknown\nbuild=unknown\nversion_scope=unknown-historical-process\n"
            "timestamp=123\nkind=exit\nreason=5\nstatus=11\nstack=unavailable\n";
        QFile file(pending);
        QVERIFY(file.open(QIODevice::WriteOnly));
        QCOMPARE(file.write(original), original.size());
        file.close();
        QTcpServer server;
        QVERIFY(server.listen(QHostAddress::LocalHost));
        int status = 503;
        QString ack = "exit-123-45";
        QList<QByteArray> requests;
        connect(&server, &QTcpServer::newConnection, &server, [&] {
            auto *socket = server.nextPendingConnection();
            auto buffer = std::make_shared<QByteArray>();
            connect(socket, &QTcpSocket::readyRead, socket, [&, socket, buffer] {
                buffer->append(socket->readAll());
                const int boundary = buffer->indexOf("\r\n\r\n");
                if (boundary < 0) return;
                int length = 0;
                for (const QByteArray &header : buffer->left(boundary).split('\n'))
                    if (header.toLower().startsWith("content-length:")) length = header.mid(15).trimmed().toInt();
                if (buffer->size() < boundary + 4 + length) return;
                requests.append(*buffer);
                const QByteArray response = QJsonDocument(QJsonObject{{"ack", ack}}).toJson(QJsonDocument::Compact);
                socket->write("HTTP/1.1 " + QByteArray::number(status) + " Result\r\nContent-Type: application/json\r\nContent-Length: "
                    + QByteArray::number(response.size()) + "\r\nConnection: close\r\n\r\n" + response);
                socket->disconnectFromHost();
            });
            connect(socket, &QTcpSocket::disconnected, socket, &QObject::deleteLater);
        });
        Api api;
        api.setBaseUrl(QStringLiteral("http://127.0.0.1:%1").arg(server.serverPort()));
        api.setToken("private-session");
        {
            CrashUploader uploader(api);
            uploader.setDirectory(spool.path());
            uploader.setUploaderBuild("35");
            QSignalSpy completed(&uploader, &CrashUploader::finished);
            uploader.start(); // Offline is not a request or a deletion.
            QCOMPARE(requests.size(), 0);
            QVERIFY(QFile::exists(pending));
            uploader.setOnline(true);
            QTRY_COMPARE(completed.size(), 1);
            QCOMPARE(completed.last().at(1).toInt(), 1);
            QVERIFY(QFile::exists(pending));
        }
        // Restart the actual Api consumer against the same private spool.
        CrashUploader restarted(api);
        restarted.setDirectory(spool.path());
        restarted.setUploaderBuild("35");
        QSignalSpy completed(&restarted, &CrashUploader::finished);
        status = 422; // Permanent validation rejection still cannot destroy evidence.
        restarted.setOnline(true);
        QTRY_COMPARE(completed.size(), 1);
        QVERIFY(QFile::exists(pending));
        status = 200;
        ack = "wrong-id";
        restarted.start();
        QTRY_COMPARE(completed.size(), 2);
        QVERIFY(QFile::exists(pending));
        file.setFileName(pending);
        QVERIFY(file.open(QIODevice::ReadOnly));
        QCOMPARE(file.readAll(), original);
        file.close();
        ack = "exit-123-45";
        restarted.setOnline(false);
        restarted.setOnline(true); // The same reconnect entry point used by main.cpp.
        QTRY_COMPARE(completed.size(), 3);
        QCOMPARE(completed.last().at(0).toInt(), 1);
        QVERIFY(!QFile::exists(pending));
        QVERIFY(QFile::exists(spool.filePath("exit-123-45.sent")));
        QCOMPARE(requests.size(), 4);
        for (const QByteArray &request : requests) {
            QVERIFY(request.startsWith("POST /v1/diagnostics/crashes HTTP/1.1"));
            QVERIFY(request.contains("Authorization: Bearer private-session"));
            const auto body = QJsonDocument::fromJson(request.mid(request.indexOf("\r\n\r\n") + 4)).object();
            QCOMPARE(body.value("report_id").toString(), QStringLiteral("exit-123-45"));
            QCOMPARE(body.value("body").toString().toUtf8(), original);
            QCOMPARE(body.value("uploader_build").toString(), QStringLiteral("35"));
        }
    }
    void noEnrollmentNeverUploads() {
        QTemporaryDir spool;
        QFile file(spool.filePath("java-1.txt"));
        QVERIFY(file.open(QIODevice::WriteOnly));
        file.write("kind=java\n");
        file.close();
        Api api;
        api.setBaseUrl("http://127.0.0.1:1");
        CrashUploader uploader(api);
        uploader.setDirectory(spool.path());
        QSignalSpy finished(&uploader, &CrashUploader::finished);
        QSignalSpy failed(&api, &Api::requestFailed);
        uploader.setOnline(true);
        QTest::qWait(20); // Let queued work run: missing enrollment still must not issue HTTP.
        QCOMPARE(finished.size(), 0);
        QCOMPARE(failed.size(), 0);
        QVERIFY(QFile::exists(file.fileName()));
    }
};
QTEST_GUILESS_MAIN(CrashUploaderTest)
#include "tst_crashuploader.moc"
