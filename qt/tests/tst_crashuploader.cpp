#include "api.h"
#include "crashuploader.h"
#include <QFile>
#include <QJsonDocument>
#include <QJsonObject>
#include <QSignalSpy>
#include <QTcpServer>
#include <QTcpSocket>
#include <QTemporaryDir>
#include <QtTest>
#include <memory>

class CrashUploaderTest : public QObject {
    Q_OBJECT
private slots:
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
