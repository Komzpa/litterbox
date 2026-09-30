#include "androidupdater.h"

#include <QCoreApplication>
#include <QCryptographicHash>
#include <QDir>
#include <QJsonDocument>
#include <QFile>
#include <QFileInfo>
#include <QJsonObject>
#include <QSignalSpy>
#include <QStandardPaths>
#include <QTcpServer>
#include <QTcpSocket>
#include <QtTest/QtTest>

namespace {
constexpr auto kPackageId = "org.qtproject.example.litterbox_qt";
constexpr auto kToken = "oracle-device-token";
const QByteArray kApkBytes("APK\0payload\xffv4", 14);

QByteArray manifest(const QString &version, const QString &package,
                    const QString &sha256, const QString &downloadPath)
{
    return QJsonDocument(QJsonObject{
        {QStringLiteral("version"), version},
        {QStringLiteral("package"), package},
        {QStringLiteral("sha256"), sha256},
        {QStringLiteral("download_path"), downloadPath}
    }).toJson(QJsonDocument::Compact);
}

class UpdateServer : public QObject
{
    Q_OBJECT
public:
    explicit UpdateServer(QObject *parent = nullptr)
        : QObject(parent)
    {
        connect(&m_server, &QTcpServer::newConnection, this, [this] {
            while (QTcpSocket *socket = m_server.nextPendingConnection()) {
                auto *buffer = new QByteArray;
                connect(socket, &QTcpSocket::readyRead, socket, [this, socket, buffer] {
                    buffer->append(socket->readAll());
                    const qsizetype headerEnd = buffer->indexOf("\r\n\r\n");
                    if (headerEnd < 0)
                        return;

                    const QByteArray headers = buffer->left(headerEnd);
                    const QByteArray requestLine = headers.split('\n').value(0).trimmed();
                    requests.append(headers);
                    const QByteArray path = requestLine.split(' ').value(1);
                    int status = 404;
                    QByteArray reason = "Not Found";
                    QByteArray body;
                    QByteArray contentType = "application/json";
                    if (path == "/v1/android/update") {
                        status = manifestStatus;
                        reason = status == 200 ? "OK" : "Service Unavailable";
                        body = manifestBody;
                    } else if (path == "/v1/android/update.apk") {
                        status = downloadStatus;
                        reason = status == 200 ? "OK" : "Service Unavailable";
                        contentType = "application/vnd.android.package-archive";
                        body = apkBody;
                    }
                    const QByteArray response = "HTTP/1.1 " + QByteArray::number(status) + " " + reason
                        + "\r\nContent-Type: " + contentType
                        + "\r\nContent-Length: " + QByteArray::number(body.size())
                        + "\r\nConnection: close\r\n\r\n" + body;
                    socket->write(response);
                    socket->disconnectFromHost();
                });
                connect(socket, &QTcpSocket::disconnected, socket, [socket, buffer] {
                    delete buffer;
                    socket->deleteLater();
                });
            }
        });
    }

    bool listen() { return m_server.listen(QHostAddress::LocalHost, 0); }
    QString baseUrl() const
    {
        return QStringLiteral("http://127.0.0.1:%1").arg(m_server.serverPort());
    }
    int requestCount(const QByteArray &path) const
    {
        int count = 0;
        for (const QByteArray &request : requests) {
            if (request.split('\n').value(0).contains(path))
                ++count;
        }
        return count;
    }

    QByteArray manifestBody;
    QByteArray apkBody = kApkBytes;
    int manifestStatus = 200;
    int downloadStatus = 200;
    QList<QByteArray> requests;

private:
    QTcpServer m_server;
};

void configure(AndroidUpdater &updater, const UpdateServer &server)
{
    updater.setBaseUrl(server.baseUrl());
    updater.setToken(QString::fromLatin1(kToken));
    updater.setPackageId(QString::fromLatin1(kPackageId));
    updater.setInstalledVersionCode(3);
}

bool requestsAreAuthenticated(const QList<QByteArray> &requests)
{
    if (requests.isEmpty())
        return false;
    for (const QByteArray &request : requests) {
        if (!request.contains("Authorization: Bearer " + QByteArray(kToken))
            || !request.contains("X-Litterbox-Api: 1"))
            return false;
    }
    return true;
}
} // namespace

QString downloadedPath(const QString &version)
{
    return QDir(QStandardPaths::writableLocation(QStandardPaths::AppDataLocation))
        .filePath(QStringLiteral("update-%1.apk").arg(version));
}

class AndroidUpdaterTest : public QObject
{
    Q_OBJECT
private slots:
    void initTestCase()
    {
        QCoreApplication::setOrganizationName(QStringLiteral("LitterboxUpdaterOracle"));
        QCoreApplication::setApplicationName(QStringLiteral("AndroidUpdater"));
        QStandardPaths::setTestModeEnabled(true);
        QDir(QStandardPaths::writableLocation(QStandardPaths::AppDataLocation)).removeRecursively();
    }

    void cleanupTestCase()
    {
        QDir(QStandardPaths::writableLocation(QStandardPaths::AppDataLocation)).removeRecursively();
    }

    void newerReleaseIsReadyOnlyAfterAuthenticatedFullDigest()
    {
        UpdateServer server;
        QVERIFY(server.listen());
        const QByteArray digest = QCryptographicHash::hash(kApkBytes, QCryptographicHash::Sha256).toHex();
        server.manifestBody = manifest(QStringLiteral("4"), QString::fromLatin1(kPackageId),
                                       QString::fromLatin1(digest), QStringLiteral("/v1/android/update.apk"));

        AndroidUpdater updater;
        configure(updater, server);
        QSignalSpy ready(&updater, &AndroidUpdater::updateReady);
        QSignalSpy noUpdate(&updater, &AndroidUpdater::noUpdateAvailable);
        QSignalSpy errors(&updater, &AndroidUpdater::errorOccurred);
        updater.checkForUpdates();

        QTRY_COMPARE(ready.size(), 1);
        QCOMPARE(noUpdate.size(), 0);
        QCOMPARE(errors.size(), 0);
        QCOMPARE(ready.constFirst().at(0).toString(), QStringLiteral("4"));
        QCOMPARE(ready.constFirst().at(1).toString(), QString::fromLatin1(digest));
        QCOMPARE(server.requests.size(), 2);
        QCOMPARE(server.requestCount("/v1/android/update "), 1);
        QCOMPARE(server.requestCount("/v1/android/update.apk "), 1);
        QVERIFY(requestsAreAuthenticated(server.requests));
        QVERIFY(server.requests.at(0).startsWith("GET /v1/android/update HTTP/1.1"));
        QVERIFY(server.requests.at(1).startsWith("GET /v1/android/update.apk HTTP/1.1"));
        QFile downloaded(downloadedPath(QStringLiteral("4")));
        QVERIFY(downloaded.open(QIODevice::ReadOnly));
        QCOMPARE(downloaded.readAll(), kApkBytes);
        QVERIFY(!updater.busy());
    }

    void equalOrOlderReleaseDoesNotDownload_data()
    {
        QTest::addColumn<QString>("version");
        QTest::newRow("equal") << QStringLiteral("3");
        QTest::newRow("older") << QStringLiteral("2");
    }

    void equalOrOlderReleaseDoesNotDownload()
    {
        QFETCH(QString, version);
        UpdateServer server;
        QVERIFY(server.listen());
        const QByteArray digest = QCryptographicHash::hash(kApkBytes, QCryptographicHash::Sha256).toHex();
        server.manifestBody = manifest(version, QString::fromLatin1(kPackageId),
                                       QString::fromLatin1(digest), QStringLiteral("/v1/android/update.apk"));

        AndroidUpdater updater;
        configure(updater, server);
        QSignalSpy ready(&updater, &AndroidUpdater::updateReady);
        QSignalSpy noUpdate(&updater, &AndroidUpdater::noUpdateAvailable);
        updater.checkForUpdates();

        QTRY_COMPARE(noUpdate.size(), 1);
        QCOMPARE(ready.size(), 0);
        QCOMPARE(server.requestCount("/v1/android/update "), 1);
        QCOMPARE(server.requestCount("/v1/android/update.apk "), 0);
        QVERIFY(requestsAreAuthenticated(server.requests));
        QVERIFY(!updater.busy());
    }

    void invalidPackageOrDownloadPathDoesNotFetchApk_data()
    {
        QTest::addColumn<QString>("package");
        QTest::addColumn<QString>("downloadPath");
        QTest::newRow("wrong-package") << QStringLiteral("org.qtproject.example")
                                        << QStringLiteral("/v1/android/update.apk");
        QTest::newRow("external-path") << QString::fromLatin1(kPackageId)
                                        << QStringLiteral("https://attacker.invalid/update.apk");
        QTest::newRow("absolute-path") << QString::fromLatin1(kPackageId)
                                        << QStringLiteral("/v1/android/other.apk");
    }

    void invalidPackageOrDownloadPathDoesNotFetchApk()
    {
        QFETCH(QString, package);
        QFETCH(QString, downloadPath);
        UpdateServer server;
        QVERIFY(server.listen());
        const QByteArray digest = QCryptographicHash::hash(kApkBytes, QCryptographicHash::Sha256).toHex();
        server.manifestBody = manifest(QStringLiteral("4"), package, QString::fromLatin1(digest), downloadPath);

        AndroidUpdater updater;
        configure(updater, server);
        QSignalSpy ready(&updater, &AndroidUpdater::updateReady);
        QSignalSpy errors(&updater, &AndroidUpdater::errorOccurred);
        updater.checkForUpdates();

        QTRY_COMPARE(errors.size(), 1);
        QCOMPARE(ready.size(), 0);
        QCOMPARE(server.requests.size(), 1);
        QCOMPARE(server.requestCount("/v1/android/update.apk "), 0);
        QVERIFY(requestsAreAuthenticated(server.requests));
        QVERIFY(!updater.busy());
    }

    void checksumMismatchDoesNotMakeUpdateReady()
    {
        UpdateServer server;
        QVERIFY(server.listen());
        server.manifestBody = manifest(QStringLiteral("4"), QString::fromLatin1(kPackageId),
                                       QString(64, QLatin1Char('0')), QStringLiteral("/v1/android/update.apk"));

        AndroidUpdater updater;
        configure(updater, server);
        QSignalSpy ready(&updater, &AndroidUpdater::updateReady);
        QSignalSpy errors(&updater, &AndroidUpdater::errorOccurred);
        updater.checkForUpdates();

        QTRY_COMPARE(errors.size(), 1);
        QCOMPARE(ready.size(), 0);
        QVERIFY(errors.constFirst().at(0).toString().contains(QStringLiteral("checksum")));
        QCOMPARE(server.requestCount("/v1/android/update.apk "), 1);
        QVERIFY(requestsAreAuthenticated(server.requests));
        QVERIFY(!QFileInfo::exists(downloadedPath(QStringLiteral("4"))));
        QVERIFY(!updater.busy());
    }

    void downloadFailureDoesNotMakeUpdateReady()
    {
        UpdateServer server;
        QVERIFY(server.listen());
        const QByteArray digest = QCryptographicHash::hash(kApkBytes, QCryptographicHash::Sha256).toHex();
        server.manifestBody = manifest(QStringLiteral("4"), QString::fromLatin1(kPackageId),
                                       QString::fromLatin1(digest), QStringLiteral("/v1/android/update.apk"));
        server.downloadStatus = 503;

        AndroidUpdater updater;
        configure(updater, server);
        QSignalSpy ready(&updater, &AndroidUpdater::updateReady);
        QSignalSpy errors(&updater, &AndroidUpdater::errorOccurred);
        updater.checkForUpdates();

        QTRY_COMPARE(errors.size(), 1);
        QCOMPARE(ready.size(), 0);
        QCOMPARE(server.requestCount("/v1/android/update.apk "), 1);
        QVERIFY(requestsAreAuthenticated(server.requests));
        QVERIFY(!QFileInfo::exists(downloadedPath(QStringLiteral("4"))));
        QVERIFY(!updater.busy());
    }
};

QTEST_GUILESS_MAIN(AndroidUpdaterTest)
#include "tst_androidupdater.moc"
