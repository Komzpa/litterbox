// SPDX-License-Identifier: MIT
#include "CardStore.h"
#include "cardstoretransport.h"
#include "api.h"
#include "crashuploader.h"
#include <QPointer>
#include <QtTest>
#include <QJsonDocument>
#include <QJsonObject>
#include <QNetworkAccessManager>
#include <QNetworkReply>
#include <QNetworkRequest>
#include <QTcpServer>
#include <QTcpSocket>
#include <QTemporaryDir>
#include <QDateTime>
#include <QQmlComponent>
#include <QQmlEngine>
#include <QQuickItem>
#include <QQuickWindow>
#include <QTime>
#include <QUrl>
#include <QUuid>
#include <QSqlQuery>
#include <QProcess>
#include <QFile>
#include <QQmlExpression>
#include <csignal>

static const QString cardId = QStringLiteral("11111111-1111-4111-8111-111111111111");

// HTTP oracle for the /v1/ops contract. Records op_id/payload pairs and
// deliberately loses one ACK after applying an op. This proves the client's
// replay protocol, not the production server's PostgreSQL implementation.
class OpsServer : public QTcpServer {
public:
    QList<QVariantMap> received;
    QList<QByteArray> paths;
    QHash<QString,QByteArray> ledger;
    int applied = 0;
    bool loseNextAck = false;
    int reject = 0;
    QList<int> statusSequence;
    OpsServer() {
        connect(this, &QTcpServer::newConnection, this, [this] {
            while (hasPendingConnections()) {
                QTcpSocket *socket = nextPendingConnection();
                auto bytes = std::make_shared<QByteArray>();
                connect(socket, &QTcpSocket::readyRead, socket, [this,socket,bytes] {
                    bytes->append(socket->readAll());
                    int split = bytes->indexOf("\r\n\r\n");
                    if (split < 0) return;
                    int length = -1;
                    for (const QByteArray &line : bytes->left(split).split('\n'))
                        if (line.toLower().startsWith("content-length:")) length = line.mid(15).trimmed().toInt();
                    if (length < 0 || bytes->size() < split + 4 + length) return;
                    const QByteArray body = bytes->mid(split + 4, length);
                    const QVariantMap op = QJsonDocument::fromJson(body).object().toVariantMap();
                    received.append(op);
                    paths.append(bytes->left(bytes->indexOf("\r\n")).split(' ').value(1));
                    const QString id = op.value("op_id", op.value("client_msg_id")).toString();
                    int status = statusSequence.isEmpty() ? reject : statusSequence.takeFirst();
                    if (!status) {
                        if (ledger.contains(id) && ledger[id] != body) status = 409;
                        else if (!ledger.contains(id)) { ledger[id] = body; ++applied; }
                    }
                    if (loseNextAck) { loseNextAck = false; socket->disconnectFromHost(); return; }
                    if (!status) status = 200;
                    QByteArray response = status == 200 ? QByteArray("{\"ok\":true}") : QByteArray("{\"ok\":false}");
                    if (status == 200 && op.contains("client_msg_id")) {
                        const QVariantMap message{{"id", "server-message"}, {"client_msg_id", id}, {"text", op.value("text")},
                            {"direction", "out"}, {"seq", 1}, {"delivery_state", "delivered"}};
                        response = QJsonDocument(QJsonObject{{"message", QJsonObject::fromVariantMap(message)}}).toJson(QJsonDocument::Compact);
                    }
                    socket->write("HTTP/1.1 " + QByteArray::number(status) + " Result\r\nContent-Type: application/json\r\nContent-Length: " + QByteArray::number(response.size()) + "\r\nConnection: close\r\n\r\n" + response);
                    socket->disconnectFromHost();
                });
                connect(socket, &QTcpSocket::disconnected, socket, &QObject::deleteLater);
            }
        });
    }
};
class HttpTransport : public QObject, public cardstore::OpTransport {
public:
    QNetworkAccessManager network;
    quint16 port;
    explicit HttpTransport(quint16 p) : port(p) {}
    void postOp(const QString &path, const QVariantMap &body,
                std::function<void(int,const QVariantMap &)> completed) override {
        QNetworkRequest request(QUrl(QStringLiteral("http://127.0.0.1:%1%2").arg(port).arg(path)));
        request.setHeader(QNetworkRequest::ContentTypeHeader, QStringLiteral("application/json"));
        auto reply = network.post(request, QJsonDocument(QJsonObject::fromVariantMap(body)).toJson(QJsonDocument::Compact));
        connect(reply, &QNetworkReply::finished, reply, [reply,completed] {
            const int status = reply->attribute(QNetworkRequest::HttpStatusCodeAttribute).toInt();
            const QVariantMap body = QJsonDocument::fromJson(reply->readAll()).object().toVariantMap();
            completed(status, body); reply->deleteLater();
        });
    }
};
class ScriptedTransport : public QObject, public cardstore::OpTransport {
public:
    QList<QVariantMap> posted;
    QList<int> statuses;
    explicit ScriptedTransport(QList<int> results) : statuses(std::move(results)) {}
    void postOp(const QString &, const QVariantMap &body,
                std::function<void(int, const QVariantMap &)> completed) override {
        posted.append(body);
        const int status = statuses.isEmpty() ? 200 : statuses.takeFirst();
        QTimer::singleShot(0, this, [completed, status] {
            completed(status, {{QStringLiteral("ok"), status >= 200 && status < 300}});
        });
    }
};
// Latency fake transport: each POST completes independently after a fixed wire
// delay, like a real RTT. A serial pump pays it per op; a pipelined pump pays
// it once per flight. High-water mark proves the batching.
class LatencyTransport : public QObject, public cardstore::OpTransport {
public:
    QList<QVariantMap> posted;
    int latencyMs;
    int maxOutstanding = 0;
    int outstanding = 0;
    explicit LatencyTransport(int latency) : latencyMs(latency) {}
    void postOp(const QString &, const QVariantMap &body,
                std::function<void(int, const QVariantMap &)> completed) override {
        posted.append(body);
        ++outstanding;
        maxOutstanding = qMax(maxOutstanding, outstanding);
        QTimer::singleShot(latencyMs, this, [this, completed] {
            --outstanding;
            completed(200, {{QStringLiteral("ok"), true}});
        });
    }
};
// Drives the same request IDs and CardStore gate used by main.cpp.
class SnapshotConsumer {
public:
    CardStore &store;
    quint64 latestRequest = 0;
    QMetaObject::Connection connection;
    explicit SnapshotConsumer(CardStore &value) : store(value) {
        connection = QObject::connect(&store, &CardStore::requestCards, &store,
            [this](const QString &, quint64 requestId) { latestRequest = requestId; });
    }
    ~SnapshotConsumer() { QObject::disconnect(connection); }
    bool apply(quint64 requestId, const QVariantMap &sections) {
        return store.applyRemoteCards(sections, requestId);
    }
};

class CardStoreTest : public QObject {
    Q_OBJECT
private slots:
    void retrySyncBypassesOfflineRefreshGate() {
        QTemporaryDir directory;
        CardStore store;
        QVERIFY(store.open(directory.filePath("retry.sqlite")));
        store.reportOffline(QStringLiteral("disconnected"));
        QSignalSpy requests(&store, &CardStore::requestCards);
        store.refresh();
        QCOMPARE(requests.size(), 0);
        store.retrySync();
        QCOMPARE(requests.size(), 1);
        QCOMPARE(requests.first().at(0).toString(), QStringLiteral("/v1/cards"));
        const quint64 retryId = requests.first().at(1).toULongLong();
        store.setOnline(true);
        QCOMPARE(requests.size(), 2);
        QVERIFY(requests.last().at(1).toULongLong() > retryId);
        store.refresh();
        QCOMPARE(requests.size(), 3);
    }
    void diagnosticSpoolPreservesMailOutboxAndUndo_data() {
        QTest::addColumn<int>("diagnosticStatus");
        for (int status : {200, 422, 503})
            QTest::newRow(qPrintable(QString::number(status))) << status;
    }
    void diagnosticSpoolPreservesMailOutboxAndUndo() {
        QFETCH(int, diagnosticStatus);
        QTemporaryDir directory;
        QTemporaryDir spool;
        QVERIFY(directory.isValid());
        QVERIFY(spool.isValid());
        const QByteArray report = "schema=1\nkind=exit\nstack=unavailable\n";
        for (int i = 0; i < 64; ++i) {
            QFile file(spool.filePath(QStringLiteral("exit-%1.txt").arg(i)));
            QVERIFY(file.open(QIODevice::WriteOnly));
            QCOMPARE(file.write(report), report.size());
        }
        const QString bodyCard = QUuid::createUuid().toString(QUuid::WithoutBraces);
        const QVariantMap sections{{"now", QVariantList{
            QVariantMap{{"id", cardId}, {"source", "mail"}, {"state", "open"}},
            QVariantMap{{"id", bodyCard}, {"source", "mail"}, {"state", "open"}}}},
            {"later", QVariantList{}}, {"missed", QVariantList{}}};
        const QVariantMap originalBody{{"html", "<p>Exact original mail</p>"}, {"messages", QVariantList{}}};
        QTcpServer server;
        QVERIFY(server.listen(QHostAddress::LocalHost));
        QList<QVariantMap> operations;
        QPointer<QTcpSocket> heldOperation;
        int diagnosticRequests = 0;
        const auto respond = [](QTcpSocket *socket, int status, const QVariantMap &body) {
            const QByteArray bytes = QJsonDocument(QJsonObject::fromVariantMap(body)).toJson(QJsonDocument::Compact);
            socket->write("HTTP/1.1 " + QByteArray::number(status) + " Result\r\nContent-Type: application/json\r\nContent-Length: "
                + QByteArray::number(bytes.size()) + "\r\nConnection: close\r\n\r\n" + bytes);
            socket->disconnectFromHost();
        };
        connect(&server, &QTcpServer::newConnection, &server, [&] {
            while (server.hasPendingConnections()) {
                auto *socket = server.nextPendingConnection();
                auto bytes = std::make_shared<QByteArray>();
                connect(socket, &QTcpSocket::readyRead, socket, [&, socket, bytes] {
                    bytes->append(socket->readAll());
                    const int boundary = bytes->indexOf("\r\n\r\n");
                    if (boundary < 0) return;
                    int length = 0;
                    for (const auto &header : bytes->left(boundary).split('\n'))
                        if (header.toLower().startsWith("content-length:")) length = header.mid(15).trimmed().toInt();
                    if (bytes->size() < boundary + 4 + length) return;
                    QVERIFY(bytes->contains("Authorization: Bearer test-device"));
                    const QByteArray route = bytes->left(bytes->indexOf("\r\n")).split(' ').value(1);
                    const QVariantMap payload = QJsonDocument::fromJson(bytes->mid(boundary + 4)).object().toVariantMap();
                    if (route == "/v1/diagnostics/crashes") {
                        ++diagnosticRequests;
                        QCOMPARE(payload.value("body").toString().toUtf8(), report);
                        respond(socket, diagnosticStatus, {{"ack", payload.value("report_id")}});
                    } else if (route == "/v1/ops") {
                        operations.append(payload);
                        if (operations.size() == 1) heldOperation = socket;
                        else respond(socket, 200, {{"ok", true}});
                    } else if (route.endsWith("/body")) respond(socket, 200, originalBody);
                    else if (route == "/v1/cards") respond(socket, 200, sections);
                    else QFAIL("Unexpected consumer route");
                });
                connect(socket, &QTcpSocket::disconnected, socket, &QObject::deleteLater);
            }
        });
        Api api;
        api.setBaseUrl(QStringLiteral("http://127.0.0.1:%1").arg(server.serverPort()));
        api.setToken("test-device");
        CrashUploader uploader(api);
        uploader.setDirectory(spool.path());
        CardStore store;
        QVERIFY(store.open(directory.filePath("diagnostics-consumer.sqlite")));
        QVERIFY(store.applyRemoteCards(sections));
        CardStoreTransport transport(store, api); // Production operation binding.
        QHash<int, QString> downloads;
        QHash<int, quint64> snapshots;
        connect(&store, &CardStore::requestMailBodyGet, &api, [&](const QString &id, const QString &route) {
            downloads.insert(api.get(route), id);
        });
        connect(&store, &CardStore::requestCards, &api, [&](const QString &route, quint64 id) {
            snapshots.insert(api.get(route), id);
        });
        connect(&api, &Api::requestFinished, &store, [&](int id, const QJsonValue &body, int) {
            if (downloads.contains(id)) store.applyRemoteMailBody(downloads.take(id), body.toObject().toVariantMap());
            else if (snapshots.contains(id)) store.applyRemoteCards(body.toObject().toVariantMap(), snapshots.take(id));
        });
        // Same request ownership/failure routing as main.cpp, not helper calls.
        connect(&api, &Api::requestFailed, &store, [&](int id, int status, const QString &error, const QJsonValue &) {
            if (uploader.ownsRequest(id)) return;
            snapshots.remove(id);
            if (status == 0) store.reportOffline(error);
            if (downloads.contains(id)) store.reportMailBodyFailed(downloads.take(id), status);
        });
        QSignalSpy completed(&uploader, &CrashUploader::finished);
        QSignalSpy posted(&store, &CardStore::requestPost);
        QSignalSpy operationFailed(&store, &CardStore::operationFailed);
        QSignalSpy bodyFailed(&store, &CardStore::mailBodyFailed);
        const QString archive = store.enqueueOp(cardId, "archive");
        QVERIFY(!archive.isEmpty());
        uploader.setOnline(true);
        store.setOnline(true);
        QTRY_COMPARE_WITH_TIMEOUT(completed.size(), 1, 15000);
        QTRY_COMPARE(operations.size(), 1);
        QTRY_COMPARE(store.cachedMailBody(bodyCard).value("html"), originalBody.value("html"));
        QCOMPARE(diagnosticRequests, diagnosticStatus == 200 ? 64 : 1);
        QCOMPARE(posted.size(), 1);
        const QVariantMap exactOperation = posted.first().at(2).toMap();
        QCOMPARE(exactOperation.value("op_id").toString(), archive);
        QCOMPARE(operations.first(), exactOperation);
        QCOMPARE(store.pendingOps(), 1);
        QVERIFY(store.online());
        QVERIFY(!store.saveFailed());
        QCOMPARE(operationFailed.size(), 0);
        QCOMPARE(bodyFailed.size(), 0);
        QVERIFY(!store.cardIds().contains(cardId));
        QVERIFY(store.undoBundleArchive(archive));
        QCOMPARE(store.cardIds(), QStringList({cardId, bodyCard}));
        QVERIFY(heldOperation);
        respond(heldOperation, 200, {{"ok", true}});
        QTRY_COMPARE(operations.size(), 2);
        QTRY_COMPARE(store.pendingOps(), 0);
        QCOMPARE(operations.first(), exactOperation);
        QCOMPARE(operations.last().value("type").toString(), QString("gmail.label_add"));
        QCOMPARE(operations.last().value("card_id").toString(), cardId);
        QCOMPARE(operations.last().value("args").toMap().value("label").toString(), QString("INBOX"));
        QCOMPARE(store.cardIds(), QStringList({cardId, bodyCard}));
        QCOMPARE(operationFailed.size(), 0);
        QCOMPARE(bodyFailed.size(), 0);
        for (int i = 0; i < 64; ++i) {
            QFile file(spool.filePath(QStringLiteral("exit-%1.txt").arg(i)));
            const bool sent = QFile::exists(spool.filePath(QStringLiteral("exit-%1.sent").arg(i)));
            if (diagnosticStatus == 200) { QVERIFY(!file.exists()); QVERIFY(sent); }
            else {
                QVERIFY(file.open(QIODevice::ReadOnly));
                QCOMPARE(file.readAll(), report);
                QVERIFY(!sent);
            }
        }
    }
    void spamUndoRepeatAndRejection() {
        QTemporaryDir directory;
        CardStore store;
        QVERIFY(store.open(directory.filePath("spam.sqlite")));
        const QVariantMap card{{"id", cardId}, {"source", "mail"}, {"state", "open"}};
        QVERIFY(store.applyRemoteCards({{"now", QVariantList{card}}, {"later", QVariantList{}}, {"missed", QVariantList{}}}));
        ScriptedTransport transport({422});
        store.setTransport(&transport);
        const QString op = store.enqueueOp(cardId, "spam");
        QVERIFY(!op.isEmpty());
        QCOMPARE(store.rowCount(), 0);
        QCOMPARE(store.enqueueOp(cardId, "spam"), op);
        QCOMPARE(store.pendingOps(), 1);
        store.undoQueuedOp(op);
        QCOMPARE(store.rowCount(), 1);
        QCOMPARE(store.pendingOps(), 0);
        store.setOnline(true);
        QTest::qWait(100);
        QCOMPARE(transport.posted.size(), 0);
        QSignalSpy failed(&store, &CardStore::operationFailed);
        QVERIFY(!store.enqueueOp(cardId, "spam").isEmpty());
        QTRY_COMPARE(failed.size(), 1);
        QCOMPARE(failed.first().at(1).toInt(), 422);
        QCOMPARE(store.rowCount(), 1);
        QTRY_COMPARE(store.pendingOps(), 0);
        QCOMPARE(transport.posted.size(), 1);
        QCOMPARE(transport.posted.first().value("type").toString(), QStringLiteral("spam"));
    }
    void bundleSpamSkipsPinnedAndUndoRestores() {
        QTemporaryDir directory;
        CardStore store;
        QVERIFY(store.open(directory.filePath("bundle-spam.sqlite")));
        const QString pin = QUuid::createUuid().toString(QUuid::WithoutBraces);
        QVERIFY(store.applyRemoteCards({{"now", QVariantList{
            QVariantMap{{"id", cardId}, {"source", "mail"}, {"state", "open"}, {"bundle_id", "bundle"}},
            QVariantMap{{"id", pin}, {"source", "mail"}, {"state", "open"}, {"bundle_id", "bundle"}, {"pinned_rank", 1}}}},
            {"later", QVariantList{}}, {"missed", QVariantList{}}}));
        const QVariantMap args{{"bundle_id", "bundle"}};
        const QString op = store.enqueueOp(cardId, "bundle_spam", args);
        QVERIFY(!op.isEmpty());
        QCOMPARE(store.cardIds(), QStringList{pin});
        QCOMPARE(store.enqueueOp(cardId, "bundle_spam", args), op);
        QCOMPARE(store.pendingOps(), 1);
        store.undoQueuedOp(op);
        QCOMPARE(store.rowCount(), 2);
        QCOMPARE(store.pendingOps(), 0);
    }
    void archiveKeepsModelIndexes() {
        QTemporaryDir directory;
        CardStore store;
        QVERIFY(store.open(directory.filePath("archive.sqlite")));
        QVariantList cards;
        for (int i = 0; i < 1500; ++i)
            cards.append(QVariantMap{{"id", QUuid::createUuid().toString(QUuid::WithoutBraces)},
                {"title", QStringLiteral("Mail %1").arg(i)}, {"source", "mail"}, {"state", "open"}});
        QVERIFY(store.applyRemoteCards({{"now", cards}, {"later", QVariantList{}}, {"missed", QVariantList{}}}));
        QSignalSpy reset(&store, &QAbstractItemModel::modelReset);
        QSignalSpy removed(&store, &QAbstractItemModel::rowsRemoved);
        const QPersistentModelIndex untouched(store.index(749));
        const QString id = store.data(store.index(750), CardStore::CardIdRole).toString();
        QElapsedTimer timer;
        timer.start();
        QVERIFY(!store.enqueueOp(id, "archive").isEmpty());
        qInfo("archive GUI call: %.3f ms; model resets: %lld", timer.nsecsElapsed() / 1000000.0, reset.size());
        QCOMPARE(reset.size(), 0);
        QCOMPARE(removed.size(), 1);
        QVERIFY(untouched.isValid());
        QCOMPARE(untouched.row(), 749);
        QCOMPARE(store.rowCount(), 1499);
        // Neighbouring negative control: an invalid action cannot remove a row.
        QVERIFY(store.enqueueOp("not-a-uuid", "archive").isEmpty());
        QCOMPARE(removed.size(), 1);
        QCOMPARE(store.rowCount(), 1499);
    }
    void largeSnapshotCoalescesModelSignals() {
        // Re-cluster: bundle ids change for hundreds of cards at once. The GUI
        // thread must not process one model notification per row.
        QTemporaryDir directory;
        CardStore store;
        QVERIFY(store.open(directory.filePath("recluster.sqlite")));
        QStringList ids;
        QVariantList first;
        for (int i = 0; i < 600; ++i) {
            const QString id = QUuid::createUuid().toString(QUuid::WithoutBraces);
            ids.append(id);
            first.append(QVariantMap{{"id", id}, {"title", QStringLiteral("Mail %1").arg(i)},
                {"source", "mail"}, {"state", "open"}, {"bundle_id", QStringLiteral("bundle-%1").arg(i % 5)}});
        }
        QVERIFY(store.applyRemoteCards({{"now", first}, {"later", QVariantList{}}, {"missed", QVariantList{}}}));
        QVariantList second;
        for (int i = 0; i < 600; ++i)
            second.append(QVariantMap{{"id", ids[i]}, {"title", QStringLiteral("Mail %1").arg(i)},
                {"source", "mail"}, {"state", "open"}, {"bundle_id", QStringLiteral("reclustered-%1").arg(i % 7)}});
        QSignalSpy changed(&store, &QAbstractItemModel::dataChanged);
        QSignalSpy moved(&store, &QAbstractItemModel::rowsMoved);
        QSignalSpy inserted(&store, &QAbstractItemModel::rowsInserted);
        QSignalSpy removed(&store, &QAbstractItemModel::rowsRemoved);
        QSignalSpy reset(&store, &QAbstractItemModel::modelReset);
        QElapsedTimer timer;
        timer.start();
        QVERIFY(store.applyRemoteCards({{"now", second}, {"later", QVariantList{}}, {"missed", QVariantList{}}}));
        qInfo("recluster 600 cards: %.3f ms; dataChanged=%lld moved=%lld inserted=%lld removed=%lld reset=%lld",
            timer.nsecsElapsed() / 1000000.0, changed.size(), moved.size(), inserted.size(), removed.size(), reset.size());
        QCOMPARE(store.rowCount(), 600);
        // Adjacent flag-only changes coalesce into ranges, not one emission
        // per row; a full reorder collapses into a single reset.
        QVERIFY(changed.size() <= 5);
        QVERIFY(moved.size() + inserted.size() + removed.size() + reset.size() <= 5);
        QVariantList reversed;
        for (int i = 599; i >= 0; --i) reversed.append(second[i]);
        QSignalSpy moved2(&store, &QAbstractItemModel::rowsMoved);
        QSignalSpy reset2(&store, &QAbstractItemModel::modelReset);
        QVERIFY(store.applyRemoteCards({{"now", reversed}, {"later", QVariantList{}}, {"missed", QVariantList{}}}));
        qInfo("reverse 600 cards: moved=%lld reset=%lld", moved2.size(), reset2.size());
        QVERIFY(reset2.size() <= 1);
        QVERIFY(moved2.size() <= 5);
    }
    void archiveDoesNotWaitForDatabaseLock() {
        QTemporaryDir directory;
        const QString path = directory.filePath("locked.sqlite");
        {
            CardStore seed;
            QVERIFY(seed.open(path));
            QVERIFY(seed.applyRemoteCards({{"now", QVariantList{QVariantMap{{"id", cardId}, {"title", "Mail"}}}},
                {"later", QVariantList{}}, {"missed", QVariantList{}}}));
        }
        CardStore store;
        QVERIFY(store.open(path));
        const QString connection = QUuid::createUuid().toString();
        auto lock = QSqlDatabase::addDatabase("QSQLITE", connection);
        lock.setDatabaseName(path);
        QVERIFY(lock.open());
        bool heartbeat = false;
        {
            QSqlQuery query(lock);
            QVERIFY(query.exec("BEGIN EXCLUSIVE"));
            QTimer::singleShot(0, &store, [&] { heartbeat = true; });
            QElapsedTimer timer;
            timer.start();
            QVERIFY(!store.enqueueOp(cardId, "archive").isEmpty());
            qInfo("archive while SQLite locked: %.3f ms", timer.nsecsElapsed() / 1000000.0);
            // The GUI remains able to dispatch events while the writer waits.
            QTRY_VERIFY(heartbeat);
            QCOMPARE(store.rowCount(), 0);
            QVERIFY(query.exec("COMMIT"));
        }
        lock.close();
        lock = QSqlDatabase();
        QSqlDatabase::removeDatabase(connection);
    }
    void archiveFailureRestoresRowWithoutReset() {
        QTemporaryDir directory;
        OpsServer server;
        QVERIFY(server.listen(QHostAddress::LocalHost));
        server.statusSequence = {422, 200};
        HttpTransport transport(server.serverPort());
        CardStore store;
        QVERIFY(store.open(directory.filePath("rejected.sqlite")));
        const QString neighbor = QStringLiteral("22222222-2222-4222-8222-222222222222");
        QVERIFY(store.applyRemoteCards({{"now", QVariantList{
            QVariantMap{{"id", cardId}, {"title", "Archive me"}},
            QVariantMap{{"id", neighbor}, {"title", "Untouched"}}}},
            {"later", QVariantList{}}, {"missed", QVariantList{}}}));
        QSignalSpy reset(&store, &QAbstractItemModel::modelReset);
        QSignalSpy failed(&store, &CardStore::operationFailed);
        const QPersistentModelIndex untouched(store.index(1));
        store.setTransport(&transport);
        store.setOnline(true);
        QVERIFY(!store.enqueueOp(cardId, "archive").isEmpty());
        QVERIFY(!store.saveNote(neighbor, "Keep this later edit").isEmpty());
        QTRY_COMPARE(failed.size(), 1);
        QCOMPARE(store.rowCount(), 2);
        QCOMPARE(store.data(store.index(0), CardStore::CardIdRole).toString(), cardId);
        QCOMPARE(store.data(store.index(1), CardStore::CardRole).toMap().value("note").toString(), QStringLiteral("Keep this later edit"));
        QVERIFY(untouched.isValid());
        QCOMPARE(untouched.row(), 1);
        QCOMPARE(reset.size(), 0);
        QTRY_COMPARE(store.pendingOps(), 0);
        QCOMPARE(server.received.size(), 2);
    }
    void reminderSemanticContract() {
        const QString proof = qEnvironmentVariable("R26_API_PROOF");
        QVariantMap sections;
        if (!proof.isEmpty()) {
            QFile file(proof);
            QVERIFY(file.open(QIODevice::ReadOnly));
            sections = QJsonDocument::fromJson(file.readAll()).object().toVariantMap();
        } else {
            sections = {{"now", QVariantList{
                QVariantMap{{"id", "reminder"}, {"source", "research"}, {"source_kind", "reminder"}},
                QVariantMap{{"id", "research_result"}, {"source", "research"}, {"source_kind", "research_result"}}}},
                {"later", QVariantList{}}, {"missed", QVariantList{}}};
        }
        QTemporaryDir directory;
        const QString path = directory.filePath("semantic.sqlite");
        {
            CardStore store;
            QVERIFY(store.open(path));
            QVERIFY(store.applyRemoteCards(sections));
        }
        CardStore restored;
        QVERIFY(restored.open(path));
        QSet<QString> seen;
        for (int i = 0; i < restored.rowCount(); ++i) {
            const QVariantMap card = restored.data(restored.index(i), CardStore::CardRole).toMap();
            const QString kind = card.value("source_kind").toString();
            QCOMPARE(card.value("source").toString(), QStringLiteral("research"));
            QCOMPARE(restored.sourceLabel(card), kind == "reminder" ? QStringLiteral("reminder") : QStringLiteral("research"));
            seen.insert(kind);
        }
        QVERIFY(seen.contains("reminder"));
        QVERIFY(seen.contains("research_result"));
        QCOMPARE(restored.sourceLabel({{"source", "research"}}), QStringLiteral("research"));
        QCOMPARE(restored.sourceLabel({{"source", "reminder"}}), QStringLiteral("reminder"));
        QCOMPARE(restored.sourceLabel({{"source", "mail"}, {"source_kind", ""}}), QStringLiteral("mail"));
    }
    void senderNameRoleArrives() {
        QTemporaryDir directory;
        QVERIFY(directory.isValid());
        const QString path = directory.filePath("sender-name.sqlite");
        const QVariantMap card{{"id", cardId}, {"title", "LinkedIn"}, {"sender_name", "LinkedIn"}};
        const QVariantMap sections{{"now", QVariantList{card}}, {"later", QVariantList{}}, {"missed", QVariantList{}}};
        {
            CardStore store;
            QVERIFY(store.open(path));
            QVERIFY(store.applyRemoteCards(sections));
            QCOMPARE(store.data(store.index(0), CardStore::SenderNameRole).toString(), QStringLiteral("LinkedIn"));
            QCOMPARE(store.roleNames().value(CardStore::SenderNameRole), QByteArray("sender_name"));
        }
        CardStore restored;
        QVERIFY(restored.open(path));
        QCOMPARE(restored.data(restored.index(0), CardStore::SenderNameRole).toString(), QStringLiteral("LinkedIn"));
    }
    void cachedCardsSurviveRestart() {
        QTemporaryDir directory;
        QVERIFY(directory.isValid());
        const QString path = directory.filePath("cache.sqlite");
        const QVariantMap card{{"id",cardId},{"title","Offline card"},{"note","Durable note"},{"received_at","2026-10-04T07:43:00Z"},{"sender_address","welcome@cerebras.net"}};
        const QVariantMap sections{{"now",QVariantList{card}},{"later",QVariantList{}},{"missed",QVariantList{}}};
        {
            CardStore store;
            QVERIFY(store.open(path));
            QVERIFY(store.applyRemoteCards(sections));
            QCOMPARE(store.rowCount(), 1);
        }
        CardStore restored;
        QVERIFY(restored.open(path));
        QVERIFY(!restored.online());
        QCOMPARE(restored.data(restored.index(0),CardStore::CardRole).toMap().value("note").toString(), QStringLiteral("Durable note"));
        QCOMPARE(restored.data(restored.index(0),CardStore::CardRole).toMap().value("received_at").toString(), QStringLiteral("2026-10-04T07:43:00Z"));
        QCOMPARE(restored.data(restored.index(0),CardStore::CardRole).toMap().value("sender_address").toString(), QStringLiteral("welcome@cerebras.net"));
        QCOMPARE(restored.data(restored.index(0),CardStore::TitleRole).toString(), QStringLiteral("Offline card"));
        // A malformed response cannot destroy the offline snapshot.
        QVERIFY(!restored.applyRemoteCards({{"now",QVariantList{}}}));
        QCOMPARE(restored.rowCount(),1);
    }
    void rejectedHeadDoesNotBlockPersistedTailOps() {
        QTemporaryDir directory;
        const QString path = directory.filePath("rejected-head.sqlite");
        QString head, tail1, tail2;
        {
            CardStore seed;
            QVERIFY(seed.open(path));
            QVERIFY(seed.applyRemoteCards({{"now", QVariantList{QVariantMap{{"id", cardId}, {"title", "Archive me"}}}},
                {"later", QVariantList{}}, {"missed", QVariantList{}}}));
            head = seed.enqueueOp(cardId, "archive");
            tail1 = seed.saveNote(cardId, "Tail one");
            tail2 = seed.enqueueOp(cardId, "pin");
            QCOMPARE(seed.pendingOps(), 3);
        }

        CardStore store;
        QVERIFY(store.open(path));
        ScriptedTransport transport({422, 200, 200});
        QSignalSpy failed(&store, &CardStore::operationFailed);
        connect(&store, &CardStore::requestCards, &store, [&store](const QString &) {
            store.applyRemoteCards({{"now", QVariantList{QVariantMap{{"id", cardId}, {"title", "Archive me"}}}},
                {"later", QVariantList{}}, {"missed", QVariantList{}}});
        });
        store.setTransport(&transport);
        store.setOnline(true);
        QTRY_COMPARE(transport.posted.size(), 3);
        QTRY_COMPARE(store.pendingOps(), 0);
        QCOMPARE(transport.posted[0].value("op_id").toString(), head);
        QCOMPARE(transport.posted[1].value("op_id").toString(), tail1);
        QCOMPARE(transport.posted[2].value("op_id").toString(), tail2);
        QCOMPARE(failed.size(), 1);
        QCOMPARE(failed[0][0].toString(), head);
        QCOMPARE(failed[0][1].toInt(), 422);
        QTRY_COMPARE(store.rowCount(), 1);
        QCOMPARE(store.data(store.index(0), CardStore::CardIdRole).toString(), cardId);
    }
    void transientFailureRetriesSameHeadBeforeTail() {
        QTemporaryDir directory;
        CardStore store;
        QVERIFY(store.open(directory.filePath("transient.sqlite")));
        QVERIFY(store.applyRemoteCards({{"now", QVariantList{QVariantMap{{"id", cardId}, {"title", "Card"}}}},
            {"later", QVariantList{}}, {"missed", QVariantList{}}}));
        const QString head = store.saveNote(cardId, "First");
        const QString tail = store.enqueueOp(cardId, "pin");
        ScriptedTransport transport({0, 200, 200});
        QSignalSpy failed(&store, &CardStore::operationFailed);
        store.setTransport(&transport);
        store.setOnline(true);
        QTRY_COMPARE(transport.posted.size(), 3);
        QTRY_COMPARE(store.pendingOps(), 0);
        QCOMPARE(transport.posted[0].value("op_id").toString(), head);
        QCOMPARE(transport.posted[1].value("op_id").toString(), head);
        QCOMPARE(transport.posted[2].value("op_id").toString(), tail);
        QCOMPARE(failed.size(), 0);
    }
    void queuedOpFlushedExactlyOnce() {
        QTemporaryDir directory;
        const QString path = directory.filePath("cache.sqlite");
        QString first, second;
        {
            CardStore store;
            QVERIFY(store.open(path));
            first = store.enqueueOp(cardId,"pin");
            second = store.saveNote(cardId,"Queued note");
            QVERIFY(!QUuid(first).isNull());
            QCOMPARE(store.pendingOps(),2);
        }
        OpsServer server;
        QVERIFY(server.listen(QHostAddress::LocalHost));
        HttpTransport transport(server.serverPort());
        {
            CardStore store;
            QVERIFY(store.open(path));
            store.setTransport(&transport);
            store.setOnline(true);
            QTRY_COMPARE(store.pendingOps(),0);
            QCOMPARE(server.received.size(),2);
            QCOMPARE(server.received[0].value("op_id").toString(),first);
            QCOMPARE(server.received[1].value("op_id").toString(),second);
            QCOMPARE(server.received[1].value("args").toMap().value("note").toString(),QStringLiteral("Queued note"));
            store.setOnline(false); store.setOnline(true); store.flush();
            QTest::qWait(30);
            QCOMPARE(server.received.size(),2);
        }
        CardStore reopened;
        QVERIFY(reopened.open(path));
        QCOMPARE(reopened.pendingOps(),0);
        QCOMPARE(server.applied,2);
    }
    void replayedOpIdIsIdempotent() {
        QTemporaryDir directory;
        const QString path = directory.filePath("cache.sqlite");
        OpsServer server;
        QVERIFY(server.listen(QHostAddress::LocalHost));
        server.loseNextAck = true;
        HttpTransport transport(server.serverPort());
        CardStore store;
        QVERIFY(store.open(path));
        store.setTransport(&transport);
        const QString id = store.dismiss(cardId);
        QSignalSpy failed(&store, &CardStore::operationFailed);
        store.setOnline(true);
        QTRY_COMPARE(store.pendingOps(), 0);
        QCOMPARE(failed.size(), 0);
        QCOMPARE(server.received.size(), 2);
        QCOMPARE(server.received[0], server.received[1]);
        QCOMPARE(server.received[1].value("op_id").toString(), id);
        QCOMPARE(server.applied, 1);
    }
    void pendingRemovalSurvivesOwnerRetention_data() {
        QTest::addColumn<QString>("type");
        for (const auto &type : {"done", "bundle_done", "snooze", "archive", "spam"})
            QTest::newRow(type) << QString::fromLatin1(type);
    }
    void pendingRemovalSurvivesOwnerRetention() {
        QFETCH(QString, type);
        QTemporaryDir directory;
        const QString path = directory.filePath("pending-removal.sqlite");
        const QString neighbor = QStringLiteral("22222222-2222-4222-8222-222222222222");
        const QVariantMap original{{"id", cardId}, {"title", "Original"}, {"source", "mail"},
            {"state", "open"}, {"bundle_id", "bundle"}, {"note", "server old"}};
        const QVariantMap untouched{{"id", neighbor}, {"title", "Neighbor"}, {"source", "manual"}, {"state", "open"}};
        const auto snapshot = [&](QVariantMap card) {
            return QVariantMap{{"now", QVariantList{card, untouched}}, {"later", QVariantList{}}, {"missed", QVariantList{}}};
        };
        QVariantMap updated = original;
        updated.insert("title", "New remote title");
        const QVariantMap empty{{"now", QVariantList{untouched}}, {"later", QVariantList{}}, {"missed", QVariantList{}}};
        QVariantMap args;
        if (type == "snooze") args.insert("until", "2030-03-04T05:06:00.000Z");
        if (type == "bundle_done") args.insert("bundle_id", "bundle");
        QString firstEdit, latestEdit, removal;
        {
            CardStore store;
            QVERIFY(store.open(path));
            QVERIFY(store.applyRemoteCards(snapshot(original)));
            firstEdit = store.saveNote(cardId, "First edit");
            latestEdit = store.saveNote(cardId, "Latest owner edit");
            removal = store.enqueueOp(cardId, type, args);
            QVERIFY(!removal.isEmpty());
            QCOMPARE(store.cardIds(), QStringList{neighbor});
            QVERIFY(store.applyRemoteCards(snapshot(original)));
            QCOMPARE(store.cardIds(), QStringList{neighbor});
            QVERIFY(store.applyRemoteCards(empty));
            QCOMPARE(store.cardIds(), QStringList{neighbor});
            QVERIFY(store.applyRemoteCards(snapshot(updated)));
            QCOMPARE(store.cardIds(), QStringList{neighbor});
            QCOMPARE(store.pendingOps(), 3);
        }
        {
            CardStore store;
            QVERIFY(store.open(path));
            QCOMPARE(store.cardIds(), QStringList{neighbor});
            QVERIFY(store.applyRemoteCards(snapshot(updated)));
            QCOMPARE(store.cardIds(), QStringList{neighbor});
            QSignalSpy posted(&store, &CardStore::requestPost);
            store.setOnline(true);
            for (const auto &id : {firstEdit, latestEdit}) {
                QTRY_VERIFY(!posted.isEmpty());
                QCOMPARE(posted.last().at(0).toString(), id);
                store.reportPostResult(id, 200, {{"ok", true}});
                const int expected = id == firstEdit ? 2 : 3;
                QTRY_COMPARE(posted.size(), expected);
            }
            const QVariantMap expected{{"op_id", removal}, {"card_id", cardId}, {"type", type}, {"args", args}};
            QCOMPARE(posted.last().at(2).toMap(), expected);
            store.reportPostResult(QUuid::createUuid().toString(QUuid::WithoutBraces), 200, {{"ok", true}});
            QCOMPARE(store.pendingOps(), 1); // An unrelated ACK cannot clear the tombstone.
            QVERIFY(store.applyRemoteCards(snapshot(updated)));
            QCOMPARE(store.cardIds(), QStringList{neighbor});
            store.reportPostResult(removal, 200, {{"ok", true}});
            QTRY_COMPARE(store.pendingOps(), 0);
            QVERIFY(store.applyRemoteCards(snapshot(updated)));
            QCOMPARE(store.cardIds(), QStringList{neighbor});
            QVERIFY(store.applyRemoteCards(empty));
        }
        CardStore cold;
        QVERIFY(cold.open(path));
        QCOMPARE(cold.pendingOps(), 0);
        QCOMPARE(cold.cardIds(), QStringList{neighbor});
    }
    void snoozeRejectAndUndoPreserveLatestOwnerEdit() {
        QTemporaryDir directory;
        const QString path = directory.filePath("snooze-rejection.sqlite");
        const QVariantMap card{{"id", cardId}, {"title", "Mail"}, {"source", "mail"}, {"state", "open"}};
        const QVariantMap full{{"now", QVariantList{card}}, {"later", QVariantList{}}, {"missed", QVariantList{}}};
        const QVariantMap args{{"until", "2030-03-04T05:06:00.000Z"}};
        QString note, snooze;
        {
            CardStore store;
            QVERIFY(store.open(path));
            QVERIFY(store.applyRemoteCards(full));
            note = store.saveNote(cardId, "Latest owner edit");
            const QString cancelled = store.enqueueOp(cardId, "snooze", args);
            QCOMPARE(store.rowCount(), 0);
            store.undoQueuedOp(cancelled);
            QCOMPARE(store.cardIds(), QStringList{cardId});
            QCOMPARE(store.data(store.index(0), CardStore::CardRole).toMap().value("note").toString(), QString("Latest owner edit"));
            snooze = store.enqueueOp(cardId, "snooze", args);
            QCOMPARE(store.rowCount(), 0);
        }
        CardStore store;
        QVERIFY(store.open(path));
        QCOMPARE(store.rowCount(), 0);
        QVERIFY(store.applyRemoteCards(full));
        QCOMPARE(store.rowCount(), 0);
        QSignalSpy posted(&store, &CardStore::requestPost);
        QSignalSpy failed(&store, &CardStore::operationFailed);
        store.setOnline(true);
        QTRY_COMPARE(posted.size(), 1);
        QCOMPARE(posted.last().at(0).toString(), note);
        store.reportPostResult(note, 200, {{"ok", true}});
        QTRY_COMPARE(posted.size(), 2);
        const QVariantMap expected{{"op_id", snooze}, {"card_id", cardId}, {"type", "snooze"}, {"args", args}};
        QCOMPARE(posted.last().at(2).toMap(), expected);
        store.reportPostResult(snooze, 422, {{"ok", false}});
        QTRY_COMPARE(store.pendingOps(), 0);
        QCOMPARE(failed.size(), 1);
        QCOMPARE(store.cardIds(), QStringList{cardId});
        QCOMPARE(store.data(store.index(0), CardStore::CardRole).toMap().value("note").toString(), QString("Latest owner edit"));
    }
    void snoozeAndPinOrderSurviveColdRestartAndAcknowledgement() {
        QTemporaryDir directory;
        QVERIFY(directory.isValid());
        const QString path = directory.filePath("cache.sqlite");
        const QString firstPin = cardId;
        const QString secondPin = QStringLiteral("22222222-2222-4222-8222-222222222222");
        const QString snoozedCard = QStringLiteral("33333333-3333-4333-8333-333333333333");
        const QString until = QStringLiteral("2030-03-04T05:06:00.000Z");
        const QVariantMap sections{{"now", QVariantList{
            QVariantMap{{"id", firstPin}, {"title", "First"}, {"pinned_rank", 1}},
            QVariantMap{{"id", secondPin}, {"title", "Second"}, {"pinned_rank", 2}}}},
            {"later", QVariantList{QVariantMap{{"id", snoozedCard}, {"title", "Snooze"}}}},
            {"missed", QVariantList{}}};
        QString reorderId, snoozeId;
        {
            CardStore store;
            QVERIFY(store.open(path));
            QVERIFY(store.applyRemoteCards(sections));
            QCOMPARE(store.data(store.index(0), CardStore::SectionRole).toString(), QStringLiteral("pinned"));
            reorderId = store.enqueueOp(firstPin, QStringLiteral("reorder_pins"),
                {{QStringLiteral("cards"), QVariantList{secondPin, firstPin}}});
            snoozeId = store.enqueueOp(snoozedCard, QStringLiteral("snooze"),
                {{QStringLiteral("until"), until}});
            QVERIFY(!QUuid(reorderId).isNull());
            QVERIFY(!QUuid(snoozeId).isNull());
            QCOMPARE(store.pinnedCardIds(), QStringList({secondPin, firstPin}));
            QCOMPARE(store.pendingOps(), 2);
            QCOMPARE(store.rowCount(), 2);
        }
        OpsServer server;
        QVERIFY(server.listen(QHostAddress::LocalHost));
        HttpTransport transport(server.serverPort());
        {
            CardStore restored;
            QVERIFY(restored.open(path));
            QCOMPARE(restored.pinnedCardIds(), QStringList({secondPin, firstPin}));
            QCOMPARE(restored.rowCount(), 2);
            QCOMPARE(restored.data(restored.index(0), CardStore::CardRole).toMap().value("id").toString(), secondPin);
            restored.setTransport(&transport);
            restored.setOnline(true);
            QTRY_COMPARE(restored.pendingOps(), 0);
            QCOMPARE(server.received.size(), 2);
            QCOMPARE(server.received[0].value("op_id").toString(), reorderId);
            QCOMPARE(server.received[0].value("type").toString(), QStringLiteral("reorder_pins"));
            QCOMPARE(server.received[0].value("args").toMap().value("cards").toList(), QVariantList({secondPin, firstPin}));
            QCOMPARE(server.received[1].value("op_id").toString(), snoozeId);
            QCOMPARE(server.received[1].value("type").toString(), QStringLiteral("snooze"));
            QCOMPARE(server.received[1].value("args").toMap().value("until").toString(), until);
        }
        CardStore coldStart;
        QVERIFY(coldStart.open(path));
        QCOMPARE(coldStart.pendingOps(), 0);
        QCOMPARE(coldStart.pinnedCardIds(), QStringList({secondPin, firstPin}));
        QCOMPARE(coldStart.rowCount(), 2);
    }
    void archiveRejectionNotifiesAndRestoresCard() {
        QTemporaryDir directory;
        QVERIFY(directory.isValid());
        CardStore store;
        QVERIFY(store.open(directory.filePath("archive-rejection.sqlite")));
        const QString rejected = QStringLiteral("44444444-4444-4444-8444-444444444444");
        const QString accepted = QStringLiteral("55555555-5555-4555-8555-555555555555");
        QVERIFY(store.applyRemoteCards({{"now", QVariantList{
            QVariantMap{{"id", rejected}, {"source", "mail"}, {"state", "open"}},
            QVariantMap{{"id", accepted}, {"source", "mail"}, {"state", "open"}}}},
            {"later", QVariantList{}}, {"missed", QVariantList{}}}));
        ScriptedTransport transport({422, 200});
        QSignalSpy failed(&store, &CardStore::operationFailed);
        store.setTransport(&transport);
        store.setOnline(true);
        QQmlEngine engine;
        QQmlComponent component(&engine, QUrl::fromLocalFile(QStringLiteral(LB_SOURCE_ACTIONS_QML)));
        QVERIFY2(component.isReady(), qPrintable(component.errorString()));
        QObject *actions = component.createWithInitialProperties({
            {"store", QVariant::fromValue(static_cast<QObject *>(&store))}, {"cardKey", rejected}, {"source", "mail"}});
        QVERIFY2(actions, qPrintable(component.errorString()));
        QVERIFY(QMetaObject::invokeMethod(actions, "primaryAction"));
        QTRY_COMPARE(failed.size(), 1);
        QCOMPARE(failed.first().at(1).toInt(), 422);
        QCOMPARE(store.rowCount(), 2);
        QTRY_COMPARE(store.pendingOps(), 0);
        QCOMPARE(transport.posted.first().value("type").toString(), QStringLiteral("archive"));
        actions->setProperty("cardKey", accepted);
        QVERIFY(QMetaObject::invokeMethod(actions, "primaryAction"));
        QTRY_COMPARE(store.pendingOps(), 0);
        QCOMPARE(store.rowCount(), 1);
        QCOMPARE(failed.size(), 1);
        QCOMPARE(transport.posted.size(), 2);
        delete actions;
    }
    void archivedMailStaysOutOfInboxAcrossStaleSnapshots() {
        QTemporaryDir directory;
        QVERIFY(directory.isValid());
        CardStore store;
        QVERIFY(store.open(directory.filePath("archive-stale-snapshot.sqlite")));
        const QString id = QStringLiteral("66666666-6666-4666-8666-666666666666");
        const QVariantMap card{{"id", id}, {"source", "mail"}, {"state", "open"}};
        const QVariantMap stale{{"now", QVariantList{card}}, {"later", QVariantList{}}, {"missed", QVariantList{}}};
        const QVariantMap absent{{"now", QVariantList{}}, {"later", QVariantList{}}, {"missed", QVariantList{}}};
        QVERIFY(store.applyRemoteCards(stale));
        ScriptedTransport transport({200});
        QVERIFY(!store.enqueueOp(id, "archive").isEmpty());
        QCOMPARE(store.rowCount(), 0);
        QVERIFY(store.applyRemoteCards(stale));
        QCOMPARE(store.rowCount(), 0);
        store.setTransport(&transport);
        store.setOnline(true);
        QTRY_COMPARE(store.pendingOps(), 0);
        QCOMPARE(transport.posted.size(), 1);
        QCOMPARE(transport.posted.first().value("type").toString(), QStringLiteral("archive"));
        QVERIFY(store.applyRemoteCards(stale));
        QCOMPARE(store.rowCount(), 0);
        QVERIFY(store.applyRemoteCards(absent));
        QCOMPARE(store.rowCount(), 0);
    }
    void qmlActionsReachDurableOutboxAndHttpAck() {
        QTemporaryDir directory;
        QVERIFY(directory.isValid());
        const QString path = directory.filePath("cache.sqlite");
        const QString firstPin = cardId;
        const QString secondPin = QStringLiteral("22222222-2222-4222-8222-222222222222");
        const QString snoozedCard = QStringLiteral("33333333-3333-4333-8333-333333333333");
        const QString expectedUntil = QDateTime(QDate(2030, 3, 4), QTime(5, 6), Qt::LocalTime)
            .toUTC().toString(Qt::ISODateWithMs);
        const QVariantMap sections{{"now", QVariantList{
            QVariantMap{{"id", firstPin}, {"title", "First"}, {"pinned_rank", 1}},
            QVariantMap{{"id", secondPin}, {"title", "Second"}, {"pinned_rank", 2}}}},
            {"later", QVariantList{QVariantMap{{"id", snoozedCard}, {"title", "Snooze"}}}},
            {"missed", QVariantList{}}};
        OpsServer server;
        QVERIFY(server.listen(QHostAddress::LocalHost));
        HttpTransport transport(server.serverPort());
        {
            CardStore offline;
            QVERIFY(offline.open(path));
            QVERIFY(offline.applyRemoteCards(sections));
            QQmlEngine engine;
            QQmlComponent component(&engine, QUrl::fromLocalFile(QStringLiteral(LB_SOURCE_ACTIONS_QML)));
            QVERIFY2(component.isReady(), qPrintable(component.errorString()));
            QQuickWindow window;
            window.resize(520, 800);
            QVariantMap snoozeProperties{{"store", QVariant::fromValue(static_cast<QObject *>(&offline))},
                {"cardKey", snoozedCard}, {"pinnedRank", QVariant()}};
            QObject *snoozeActions = component.createWithInitialProperties(snoozeProperties);
            QVERIFY2(snoozeActions, qPrintable(component.errorString()));
            auto *snoozeItem = qobject_cast<QQuickItem *>(snoozeActions);
            QVERIFY(snoozeItem);
            snoozeItem->setParentItem(window.contentItem());
            window.show();
            QCoreApplication::processEvents();
            QVERIFY(QMetaObject::invokeMethod(snoozeActions, "captureSnooze",
                                              Q_ARG(QVariant, QVariant(QStringLiteral("2030-03-04 05:06")))));
            QCOMPARE(offline.pendingOps(), 1);
            QCOMPARE(offline.rowCount(), 2);

            QVERIFY(QMetaObject::invokeMethod(snoozeActions, "captureSnooze",
                                              Q_ARG(QVariant, QVariant(QStringLiteral("2030-02-30 12:00")))));
            QCOMPARE(offline.pendingOps(), 1);
            QVERIFY(snoozeActions->property("snoozeError").toString().size() > 0);

            QVariantMap pinProperties{{"store", QVariant::fromValue(static_cast<QObject *>(&offline))},
                {"cardKey", secondPin}, {"pinnedRank", 2}};
            QObject *pinActions = component.createWithInitialProperties(pinProperties);
            QVERIFY2(pinActions, qPrintable(component.errorString()));
            auto *pinItem = qobject_cast<QQuickItem *>(pinActions);
            QVERIFY(pinItem);
            pinItem->setParentItem(window.contentItem());
            QVERIFY(QMetaObject::invokeMethod(pinActions, "movePin", Q_ARG(QVariant, QVariant(-1))));
            QCOMPARE(offline.pinnedCardIds(), QStringList({secondPin, firstPin}));
            QCOMPARE(offline.pendingOps(), 2);
            QVERIFY(QMetaObject::invokeMethod(pinActions, "movePin", Q_ARG(QVariant, QVariant(-1))));
            QCOMPARE(offline.pendingOps(), 2);
        }
        {
            CardStore restored;
            QVERIFY(restored.open(path));
            QCOMPARE(restored.pinnedCardIds(), QStringList({secondPin, firstPin}));
            QCOMPARE(restored.rowCount(), 2);
            restored.setTransport(&transport);
            restored.setOnline(true);
            QTRY_COMPARE(restored.pendingOps(), 0);
            QCOMPARE(server.received.size(), 2);
            QCOMPARE(server.received[0].value("type").toString(), QStringLiteral("snooze"));
            QCOMPARE(server.received[0].value("args").toMap().value("until").toString(), expectedUntil);
            QCOMPARE(server.received[1].value("type").toString(), QStringLiteral("reorder_pins"));
            QCOMPARE(server.received[1].value("args").toMap().value("cards").toList(), QVariantList({secondPin, firstPin}));
        }
        CardStore coldStart;
        QVERIFY(coldStart.open(path));
        QCOMPARE(coldStart.pendingOps(), 0);
        QCOMPARE(coldStart.pinnedCardIds(), QStringList({secondPin, firstPin}));
        QCOMPARE(coldStart.rowCount(), 2);
    }
    void manualCreateAndReorderPersistAndSync() {
        QTemporaryDir directory;
        QVERIFY(directory.isValid());
        const QString path = directory.filePath("cache.sqlite");
        const QString first = cardId;
        const QString second = QStringLiteral("22222222-2222-4222-8222-222222222222");
        const QVariantMap sections{{"now", QVariantList{QVariantMap{{"id", first}, {"title", "First"}}, QVariantMap{{"id", second}, {"title", "Second"}}}}, {"later", QVariantList{}}, {"missed", QVariantList{}}};
        OpsServer server;
        QVERIFY(server.listen(QHostAddress::LocalHost));
        HttpTransport transport(server.serverPort());
        QString createdId;
        {
            CardStore store;
            QVERIFY(store.open(path));
            QVERIFY(store.applyRemoteCards(sections));
            QVERIFY(!store.createCard("   ", "details"));
            QCOMPARE(store.rowCount(), 2);
            QVERIFY(store.createCard("  Manual task  ", "details"));
            QCOMPARE(store.rowCount(), 3);
            createdId = store.data(store.index(0), CardStore::CardIdRole).toString();
            QCOMPARE(store.data(store.index(0), CardStore::CardRole).toMap().value("title").toString(), QStringLiteral("Manual task"));
            QVERIFY(store.moveCard(second, -1));
            QCOMPARE(store.data(store.index(1), CardStore::CardIdRole).toString(), second);
            QCOMPARE(store.pendingOps(), 2);
        }
        CardStore restored;
        QVERIFY(restored.open(path));
        QCOMPARE(restored.data(restored.index(0), CardStore::CardIdRole).toString(), createdId);
        QCOMPARE(restored.data(restored.index(1), CardStore::CardIdRole).toString(), second);
        restored.setTransport(&transport);
        restored.setOnline(true);
        QTRY_COMPARE(restored.pendingOps(), 0);
        QCOMPARE(server.received.size(), 2);
        QCOMPARE(server.received[0].value("type").toString(), QStringLiteral("create_card"));
        QCOMPARE(server.received[0].value("card_id").toString(), createdId);
        QCOMPARE(server.received[0].value("args").toMap().value("title").toString(), QStringLiteral("Manual task"));
        QCOMPARE(server.received[1].value("type").toString(), QStringLiteral("reorder_cards"));
        const QVariantList expectedOrder{createdId, second, first};
        QCOMPARE(server.received[1].value("args").toMap().value("cards").toList(), expectedOrder);
    }
    void rejectedWriteDoesNotBlockUnrelatedActionsOrSendDependentEdits() {
        QTemporaryDir directory;
        CardStore store;
        QVERIFY(store.open(directory.filePath("ordered.sqlite")));
        QVERIFY(store.applyRemoteCards({{"now", QVariantList{QVariantMap{{"id", cardId}, {"title", "Other"}}}},
            {"later", QVariantList{}}, {"missed", QVariantList{}}}));
        QVERIFY(store.createCard("Retained journal", "", "journal"));
        const QString journal = store.cardIds().first();
        const QString dependent = store.saveNote(journal, "Later edit also retained");
        const QString unrelated = store.enqueueOp(cardId, "archive");
        ScriptedTransport transport({422, 422, 200, 200});
        QSignalSpy failed(&store, &CardStore::operationFailed);
        store.setTransport(&transport);
        store.setOnline(true);
        QTRY_COMPARE(failed.size(), 1); // Only the reversible archive reports failure.
        QTRY_COMPARE(store.pendingOps(), 2);
        QCOMPARE(transport.posted.size(), 2);
        QCOMPARE(transport.posted.last().value("op_id").toString(), unrelated);
        QVERIFY(store.cardIds().contains(journal));
        QVERIFY(store.cardIds().contains(cardId));
        QVERIFY(store.retainedWriteFailed(transport.posted.first().value("op_id").toString()));
        QVERIFY(!store.retainedWriteFailed(unrelated));
        store.refresh();
        QTRY_COMPARE(store.pendingOps(), 0);
        QCOMPARE(transport.posted.size(), 4);
        QCOMPARE(transport.posted[2], transport.posted[0]);
        QCOMPARE(transport.posted[3].value("op_id").toString(), dependent);
        QVERIFY(!store.saveFailed());
    }
    void ownerWriteSurvivesFailureRestartAndMissingSnapshot_data() {
        QTest::addColumn<int>("status");
        QTest::addColumn<QString>("kind");
        for (int status : {400, 401, 403, 409, 422, 429, 500, 503, 0})
            for (const QString &kind : {QStringLiteral("manual"), QStringLiteral("journal"), QStringLiteral("note"), QStringLiteral("reminder"), QStringLiteral("chat")})
                QTest::newRow(qPrintable(QString::number(status) + "-" + kind)) << status << kind;
    }
    void ownerWriteSurvivesFailureRestartAndMissingSnapshot() {
        QFETCH(int, status);
        QFETCH(QString, kind);
        QTemporaryDir directory;
        const QString path = directory.filePath("owner.sqlite");
        const QString text = QStringLiteral("Owner text\nNever discard this edit");
        const QVariantMap empty{{"now", QVariantList{}}, {"later", QVariantList{}}, {"missed", QVariantList{}}};
        OpsServer server;
        QVERIFY(server.listen(QHostAddress::LocalHost));
        server.reject = status;
        server.loseNextAck = status == 0;
        Api api;
        api.setBaseUrl(QStringLiteral("http://127.0.0.1:%1").arg(server.serverPort()));
        QVariantMap original;
        QString id;
        {
            CardStore store;
            QVERIFY(store.open(path));
            CardStoreTransport transport(store, api);
            if (kind == "note") {
                QVERIFY(store.applyRemoteCards({{"now", QVariantList{QVariantMap{{"id", cardId}, {"title", "Mail"}, {"note", "old"}}}},
                    {"later", QVariantList{}}, {"missed", QVariantList{}}}));
                QVERIFY(!store.saveNote(cardId, text).isEmpty());
            } else if (kind == "reminder") QVERIFY(!store.createReminder(text, "2030-01-01T12:00:00Z", "weekly").isEmpty());
            else if (kind == "chat") QVERIFY(!store.sendAgentMessage(cardId, text).isEmpty());
            else QVERIFY(store.createCard(text, kind == "manual" ? text : QString(), kind));
            id = kind == "chat" ? store.pendingAgentMessages(cardId).first().toMap().value("client_msg_id").toString() : store.cardIds().first();
            store.setOnline(true);
            QTRY_COMPARE(server.received.size(), 1);
            store.setOnline(false);
            original = server.received.first();
            // Undo is not a text-discard API, even before the operation is sent.
            store.undoQueuedOp(original.value("op_id", original.value("client_msg_id")).toString());
            QCOMPARE(store.pendingOps(), 1);
        }
        CardStore restored;
        QVERIFY(restored.open(path));
        QCOMPARE(restored.pendingOps(), 1);
        QVERIFY(restored.applyRemoteCards(empty));
        if (kind == "chat") {
            QCOMPARE(restored.pendingAgentMessages(cardId).first().toMap().value("text").toString(), text);
        } else {
            QCOMPARE(restored.cardIds(), QStringList{id});
            const auto card = restored.data(restored.index(0), CardStore::CardRole).toMap();
            if (kind == "note") QCOMPARE(card.value("note").toString(), text);
            else if (kind == "journal") QCOMPARE(card.value("title").toString() + "\n" + card.value("summary").toString(), text);
            else if (kind == "reminder") QCOMPARE(card.value("title").toString(), text);
            else QCOMPARE(card.value("summary").toString(), text);
        }
        server.reject = 0;
        CardStoreTransport transport(restored, api);
        restored.setOnline(true);
        QTRY_COMPARE(restored.pendingOps(), 0);
        QCOMPARE(server.received.size(), 2);
        QCOMPARE(server.received.last(), original);
        QCOMPARE(server.applied, 1);
        restored.flush();
        QTest::qWait(20);
        QCOMPARE(server.received.size(), 2);
    }
    void ownerWriteRestartMidFlight() {
        QTemporaryDir directory;
        const QString path = directory.filePath("inflight.sqlite");
        QVariantMap original;
        {
            CardStore store;
            QVERIFY(store.open(path));
            QSignalSpy requests(&store, &CardStore::requestPost);
            store.setOnline(true);
            QVERIFY(store.createCard("Unacknowledged text", "", "journal"));
            QTRY_COMPARE(requests.size(), 1);
            original = requests.first().at(2).toMap();
            // The transport never supplies a completion before process exit.
        }
        OpsServer server;
        QVERIFY(server.listen(QHostAddress::LocalHost));
        HttpTransport transport(server.serverPort());
        CardStore restored;
        QVERIFY(restored.open(path));
        QCOMPARE(restored.pendingOps(), 1);
        restored.setTransport(&transport);
        restored.setOnline(true);
        QTRY_COMPARE(restored.pendingOps(), 0);
        QCOMPARE(server.received, QList<QVariantMap>{original});
        QCOMPARE(server.applied, 1);
    }
    void ownerSaveIsLocalBeforeReplyAndQuietOnFailure_data() {
        QTest::addColumn<int>("status");
        for (int status : {422, 500, 0}) QTest::newRow(qPrintable(QString::number(status))) << status;
    }
    void ownerSaveIsLocalBeforeReplyAndQuietOnFailure() {
        QFETCH(int, status);
        QTemporaryDir directory;
        CardStore store;
        QVERIFY(store.open(directory.filePath("local-first.sqlite")));
        QSignalSpy requests(&store, &CardStore::requestPost);
        QSignalSpy toasts(&store, &CardStore::operationFailed);
        store.setOnline(true);
        QVERIFY(store.createCard("Saved locally", "", "journal"));
        QCOMPARE(store.rowCount(), 1); // Save has returned, no transport reply exists.
        QCOMPARE(store.pendingOps(), 1);
        QTRY_COMPARE(requests.size(), 1);
        const QString id = requests.first().at(0).toString();
        store.reportPostResult(id, status, {{"ok", false}});
        QCOMPARE(toasts.size(), 0);
        QVERIFY(store.saveFailed());
        QCOMPARE(store.rowCount(), 1);
        store.refresh();
        QTRY_COMPARE(requests.size(), 2);
        QCOMPARE(requests.last().at(2), requests.first().at(2));
        store.reportPostResult(id, 200, {{"ok", true}});
        QTRY_COMPARE(store.pendingOps(), 0);
    }
    void ownerDraftSurvivesKillBeforeDatabaseCommit() {
        const QString childPath = qEnvironmentVariable("LB_DRAFT_CRASH_CHILD");
        if (!childPath.isEmpty()) {
            CardStore store;
            QVERIFY(store.open(childPath));
            auto lock = QSqlDatabase::addDatabase("QSQLITE", "crash-lock");
            lock.setDatabaseName(childPath);
            QVERIFY(lock.open());
            QSqlQuery query(lock);
            QVERIFY(query.exec("BEGIN EXCLUSIVE"));
            QVERIFY(store.createCard("Survive SIGKILL", "raw owner text", "journal"));
            QFile ready(childPath + ".ready");
            QVERIFY(ready.open(QIODevice::WriteOnly));
            ready.write("saved");
            ready.close();
            QTest::qWait(60000); // Parent kills this exact PID while SQLite is locked.
            QFAIL("Parent did not kill the child");
        }
        QTemporaryDir directory;
        const QString path = directory.filePath("killed.sqlite");
        QProcess child;
        auto environment = QProcessEnvironment::systemEnvironment();
        environment.insert("LB_DRAFT_CRASH_CHILD", path);
        child.setProcessEnvironment(environment);
        child.start(QCoreApplication::applicationFilePath(), {"ownerDraftSurvivesKillBeforeDatabaseCommit"});
        QVERIFY(child.waitForStarted());
        const qint64 pid = child.processId();
        qInfo() << "Crash-test child PID:" << pid;
        QTRY_VERIFY_WITH_TIMEOUT(QFile::exists(path + ".ready"), 10000);
        QCOMPARE(::kill(pid, SIGKILL), 0);
        QVERIFY(child.waitForFinished());
        QCOMPARE(child.exitStatus(), QProcess::CrashExit);
        CardStore restored;
        QVERIFY(restored.open(path));
        QCOMPARE(restored.rowCount(), 1);
        QCOMPARE(restored.data(restored.index(0), CardStore::TitleRole).toString(), QStringLiteral("Survive SIGKILL"));
        QCOMPARE(restored.pendingOps(), 1);
        OpsServer server;
        QVERIFY(server.listen(QHostAddress::LocalHost));
        HttpTransport transport(server.serverPort());
        restored.setTransport(&transport);
        restored.setOnline(true);
        QTRY_COMPARE(restored.pendingOps(), 0);
        QCOMPARE(server.applied, 1);
        QCOMPARE(server.received.size(), 1);
    }
    void ownerRowsAndOutboxRejectDestructiveSql() {
        QTemporaryDir directory;
        const QString path = directory.filePath("guard.sqlite");
        {
            CardStore store;
            QVERIFY(store.open(path));
            QVERIFY(store.createCard("Do not delete", "", "journal"));
        }
        const QString connection = QUuid::createUuid().toString();
        auto db = QSqlDatabase::addDatabase("QSQLITE", connection);
        db.setDatabaseName(path);
        QVERIFY(db.open());
        {
            QSqlQuery query(db);
            QVERIFY(!query.exec("DELETE FROM cards"));
            QVERIFY(!query.exec("DELETE FROM outbox"));
            QVERIFY(!query.exec("UPDATE outbox SET payload='{}'"));
            QVERIFY(query.exec("SELECT count(*) FROM cards"));
            QVERIFY(query.next());
            QCOMPARE(query.value(0).toInt(), 1);
        }
        db.close();
        db = QSqlDatabase();
        QSqlDatabase::removeDatabase(connection);
    }
    void reminderAndChatConsumersPersistBeforeReply_data() {
        QTest::addColumn<QString>("kind");
        QTest::newRow("reminder") << QStringLiteral("reminder");
        QTest::newRow("chat") << QStringLiteral("chat");
    }
    void reminderAndChatConsumersPersistBeforeReply() {
        QFETCH(QString, kind);
        QTemporaryDir directory;
        const QString path = directory.filePath("consumer.sqlite");
        OpsServer server;
        QVERIFY(server.listen(QHostAddress::LocalHost));
        server.reject = 422;
        Api api;
        api.setBaseUrl(QStringLiteral("http://127.0.0.1:%1").arg(server.serverPort()));
        QVariantMap original;
        {
            CardStore store;
            QVERIFY(store.open(path));
            CardStoreTransport transport(store, api);
            QQmlEngine engine;
            QQmlComponent component(&engine, QUrl::fromLocalFile(QStringLiteral(LB_PAGES_DIR) +
                (kind == "chat" ? "/AgentChatPage.qml" : "/ReminderDialog.qml")));
            QVERIFY2(component.isReady(), qPrintable(component.errorString()));
            QVariantMap properties{{"store", QVariant::fromValue(static_cast<QObject *>(&store))}};
            if (kind == "chat") {
                properties.insert("api", QVariant::fromValue(static_cast<QObject *>(&api)));
                properties.insert("threadId", cardId);
                properties.insert("stream", QVariant::fromValue(static_cast<QObject *>(nullptr)));
            }
            std::unique_ptr<QObject> page(component.createWithInitialProperties(properties));
            QVERIFY2(page, qPrintable(component.errorString()));
            if (kind == "chat") {
                auto composer = page->findChild<QObject *>("messageComposer");
                QVERIFY(composer);
                composer->setProperty("text", "Durable chat text");
                QVERIFY(QMetaObject::invokeMethod(page.get(), "sendMessage"));
                QCOMPARE(composer->property("text").toString(), QString());
                QCOMPARE(store.pendingAgentMessages(cardId).size(), 1);
                QCOMPARE(store.pendingAgentMessages(cardId).first().toMap().value("text").toString(), QStringLiteral("Durable chat text"));
            } else {
                auto title = page->findChild<QObject *>("titleField");
                auto when = page->findChild<QObject *>("whenField");
                QVERIFY(title && when);
                title->setProperty("text", "Durable reminder text");
                QQmlExpression expression(QQmlEngine::contextForObject(page.get()), page.get(),
                    "new Date(2030,0,1,12,0).toLocaleString(Qt.locale(), localeDateTimeFormat)");
                when->setProperty("text", expression.evaluate());
                QVERIFY(QMetaObject::invokeMethod(page.get(), "submit"));
                QCOMPARE(store.rowCount(), 1);
            }
            QCOMPARE(store.pendingOps(), 1);
            QFile journal(path + ".drafts.jsonl");
            QVERIFY(journal.open(QIODevice::ReadOnly));
            QVERIFY(journal.readAll().contains(kind == "chat" ? "Durable chat text" : "Durable reminder text"));
            store.setOnline(true);
            QTRY_VERIFY(store.saveFailed());
            QCOMPARE(store.pendingOps(), 1);
            QCOMPARE(server.received.size(), 1);
            original = server.received.first();
            if (kind == "chat") {
                QCOMPARE(server.paths.first(), QByteArray("/v1/a2a/threads/") + cardId.toUtf8() + "/messages");
                QCOMPARE(original.size(), 2);
                QCOMPARE(original.value("text").toString(), QStringLiteral("Durable chat text"));
            } else {
                QCOMPARE(server.paths.first(), QByteArray("/v1/ops"));
                QCOMPARE(original.value("type").toString(), QStringLiteral("create_reminder"));
                QCOMPARE(original.value("args").toMap().value("id"), original.value("card_id"));
            }
            store.setOnline(false);
        }
        CardStore restored;
        QVERIFY(restored.open(path));
        QCOMPARE(restored.pendingOps(), 1);
        if (kind == "chat") QCOMPARE(restored.pendingAgentMessages(cardId).size(), 1);
        else QCOMPARE(restored.rowCount(), 1);
        CardStoreTransport transport(restored, api);
        server.reject = 0;
        restored.setOnline(true);
        QTRY_COMPARE(restored.pendingOps(), 0);
        QCOMPARE(server.received.size(), 2);
        QCOMPARE(server.received.last(), original);
        QCOMPARE(server.applied, 1);
    }
    void mainTransportRetainsRejectedNoteAndRetriesExactOperation() {
        QTemporaryDir directory;
        OpsServer server;
        QVERIFY(server.listen(QHostAddress::LocalHost));
        server.reject = 422;
        Api api;
        api.setBaseUrl(QStringLiteral("http://127.0.0.1:%1").arg(server.serverPort()));
        api.setToken("isolated-test-token");
        CardStore store;
        QVERIFY(store.open(directory.filePath("main-transport.sqlite")));
        QVERIFY(store.applyRemoteCards({{"now", QVariantList{QVariantMap{{"id", cardId}, {"title", "Mail"}, {"note", "old"}}}},
            {"later", QVariantList{}}, {"missed", QVariantList{}}}));
        // This is the exact production binding constructed by main.cpp, not OpTransport.
        CardStoreTransport transport(store, api);
        QSignalSpy toasts(&store, &CardStore::operationFailed);
        store.setOnline(true);
        const QString op = store.saveNote(cardId, "Owner's revised note");
        QVERIFY(!op.isEmpty());
        QCOMPARE(store.data(store.index(0), CardStore::CardRole).toMap().value("note").toString(), QStringLiteral("Owner's revised note"));
        QTRY_VERIFY(store.saveFailed());
        QCOMPARE(store.pendingOps(), 1);
        QCOMPARE(toasts.size(), 0);
        QCOMPARE(server.paths, QList<QByteArray>{"/v1/ops"});
        const QVariantMap original = server.received.first();
        QCOMPARE(original.value("op_id").toString(), op);
        QCOMPARE(original.value("type").toString(), QStringLiteral("note"));
        server.reject = 0;
        store.refresh();
        QTRY_COMPARE(store.pendingOps(), 0);
        QCOMPARE(server.paths, (QList<QByteArray>{"/v1/ops", "/v1/ops"}));
        QCOMPARE(server.received.last(), original);
        QCOMPARE(server.applied, 1);
        QCOMPARE(store.data(store.index(0), CardStore::CardRole).toMap().value("note").toString(), QStringLiteral("Owner's revised note"));
    }
    void latestNoteEditSurvivesStaleSnapshotAndRestart() {
        QTemporaryDir directory;
        const QString path = directory.filePath("edits.sqlite");
        const QVariantMap stale{{"now", QVariantList{QVariantMap{{"id", cardId}, {"title", "Mail"}, {"note", "server old"}}}},
            {"later", QVariantList{}}, {"missed", QVariantList{}}};
        {
            CardStore store;
            QVERIFY(store.open(path));
            QVERIFY(store.applyRemoteCards(stale));
            QVERIFY(!store.saveNote(cardId, "First edit").isEmpty());
            QVERIFY(!store.saveNote(cardId, "Latest edit").isEmpty());
            QVERIFY(store.applyRemoteCards(stale));
            QCOMPARE(store.data(store.index(0), CardStore::CardRole).toMap().value("note").toString(), QStringLiteral("Latest edit"));
        }
        CardStore restored;
        QVERIFY(restored.open(path));
        QVERIFY(restored.applyRemoteCards(stale));
        QCOMPARE(restored.data(restored.index(0), CardStore::CardRole).toMap().value("note").toString(), QStringLiteral("Latest edit"));
        OpsServer server;
        QVERIFY(server.listen(QHostAddress::LocalHost));
        HttpTransport transport(server.serverPort());
        restored.setTransport(&transport);
        restored.setOnline(true);
        QTRY_COMPARE(restored.pendingOps(), 0);
        QCOMPARE(server.applied, 2);
        QCOMPARE(server.received[0].value("args").toMap().value("note").toString(), QStringLiteral("First edit"));
        QCOMPARE(server.received[1].value("args").toMap().value("note").toString(), QStringLiteral("Latest edit"));
    }
    void rejectedReorderRebasedFromFreshSnapshot_data() {
        QTest::addColumn<QString>("reason");
        QTest::newRow("removed-id") << QStringLiteral("card not found");
        QTest::newRow("new-id") << QStringLiteral("card order must include every open card");
    }
    void rejectedReorderRebasedFromFreshSnapshot() {
        QFETCH(QString, reason);
        QTemporaryDir directory;
        const QString path = directory.filePath("rebase.sqlite");
        const QString second = QUuid::createUuid().toString(QUuid::WithoutBraces);
        const QString third = QUuid::createUuid().toString(QUuid::WithoutBraces);
        const QString added = QUuid::createUuid().toString(QUuid::WithoutBraces);
        CardStore store;
        QVERIFY(store.open(path));
        QVERIFY(store.applyRemoteCards({{"now", QVariantList{QVariantMap{{"id", cardId}}, QVariantMap{{"id", second}}, QVariantMap{{"id", third}}}},
            {"later", QVariantList{}}, {"missed", QVariantList{}}}));
        const QString original = store.reorderCards({second, third, cardId});
        QVERIFY(!original.isEmpty());
        QSignalSpy posts(&store, &CardStore::requestPost);
        QSignalSpy snapshots(&store, &CardStore::requestCards);
        store.setOnline(true);
        QTRY_COMPARE(posts.size(), 1);
        store.reportPostResult(original, 422, {{"error", reason}});
        QCOMPARE(store.pendingOps(), 1);
        QVERIFY(!snapshots.isEmpty());
        const quint64 fresh = snapshots.last().at(1).toULongLong();
        QVERIFY(store.applyRemoteCards({{"now", QVariantList{QVariantMap{{"id", cardId}}, QVariantMap{{"id", third}}, QVariantMap{{"id", added}}}},
            {"later", QVariantList{}}, {"missed", QVariantList{}}}, fresh));
        QTRY_COMPARE(posts.size(), 2);
        const QVariantMap replacement = posts.last().at(2).toMap();
        const QString replacementId = replacement.value("op_id").toString();
        QVERIFY(replacementId != original);
        QCOMPARE(replacement.value("args").toMap().value("cards").toList(), (QVariantList{third, cardId, added}));
        QCOMPARE(store.pendingOps(), 1);
        store.reportPostResult(replacementId, 200, {{"ok", true}});
        QTRY_COMPARE(store.pendingOps(), 0);
        store.retrySync();
        QCOMPARE(posts.size(), 2);
    }
    void deletedCardOperationRetires_data() {
        QTest::addColumn<int>("status");
        QTest::newRow("http404") << 404;
        QTest::newRow("semantic422") << 422;
    }
    void deletedCardOperationRetires() {
        QFETCH(int, status);
        QTemporaryDir directory;
        const QString path = directory.filePath("deleted.sqlite");
        QString original;
        {
            CardStore store;
            QVERIFY(store.open(path));
            QVERIFY(store.applyRemoteCards({{"now", QVariantList{QVariantMap{{"id", cardId}, {"note", "Original"}}}},
                {"later", QVariantList{}}, {"missed", QVariantList{}}}));
            original = store.enqueueOp(cardId, "note", {{"note", "Preserve original journal bytes"}});
            QVERIFY(!original.isEmpty());
            QSignalSpy posts(&store, &CardStore::requestPost);
            QSignalSpy acknowledgements(&store, &CardStore::operationAcknowledged);
            store.setOnline(true);
            QTRY_COMPARE(posts.size(), 1);
            store.reportPostResult(original, status, {{"error", "card not found"}});
            QTRY_COMPARE(store.pendingOps(), 0);
            QCOMPARE(acknowledgements.size(), 0);
            store.retrySync();
            QCOMPARE(posts.size(), 1);
        }
        CardStore reopened;
        QVERIFY(reopened.open(path));
        QCOMPARE(reopened.pendingOps(), 0);
        QFile journal(path + ".drafts.jsonl");
        QVERIFY(journal.open(QIODevice::ReadOnly));
        const QByteArray bytes = journal.readAll();
        QVERIFY(bytes.contains("Preserve original journal bytes"));
        QVERIFY(bytes.contains("\"retired\":true"));
    }
    void rejectedJournalRetainsTextAndRetries_data() {
        QTest::addColumn<QString>("retryVia");
        QTest::newRow("backoff") << QStringLiteral("backoff");
        QTest::newRow("retry") << QStringLiteral("retry");
        QTest::newRow("restart") << QStringLiteral("restart");
    }
    void rejectedJournalRetainsTextAndRetries() {
        QFETCH(QString, retryVia);
        QTemporaryDir directory;
        const QString path = directory.filePath("rejected-journal.sqlite");
        const QString body = QStringLiteral("Owner's private note\nText must survive rejection.");
        OpsServer server;
        QVERIFY(server.listen(QHostAddress::LocalHost));
        server.statusSequence = {422};
        HttpTransport transport(server.serverPort());
        QString journalId;
        QVariantMap originalOp;
        {
            CardStore store;
            QVERIFY(store.open(path));
            QSignalSpy failed(&store, &CardStore::operationFailed);
            store.setTransport(&transport);
            store.setOnline(true);
            QVERIFY(store.createCard(body, "", "journal"));
            journalId = store.cardIds().first();
            QTRY_VERIFY(store.saveFailed());
            QCOMPARE(failed.size(), 0); // Background sync never reports Save as undone.
            QCOMPARE(store.cardIds(), QStringList{journalId});
            QCOMPARE(store.pendingOps(), 1);
            originalOp = server.received.first();
            QCOMPARE(originalOp.value("args").toMap().value("body").toString(), body);
            QVERIFY(store.applyRemoteCards({{"now", QVariantList{}}, {"later", QVariantList{}}, {"missed", QVariantList{}}}));
            QCOMPARE(store.cardIds(), QStringList{journalId});
            const auto card = store.data(store.index(0), CardStore::CardRole).toMap();
            QCOMPARE(card.value("title").toString() + "\n" + card.value("summary").toString(), body);
            if (retryVia == "retry") store.refresh(); // Production Retry entrypoint.
            if (retryVia != "restart") QTRY_COMPARE(store.pendingOps(), 0);
            else store.setOnline(false);
        }
        CardStore reopened;
        QVERIFY(reopened.open(path));
        QCOMPARE(reopened.cardIds(), QStringList{journalId});
        const auto cached = reopened.data(reopened.index(0), CardStore::CardRole).toMap();
        QCOMPARE(cached.value("title").toString() + "\n" + cached.value("summary").toString(), body);
        if (retryVia == "restart") {
            QCOMPARE(reopened.pendingOps(), 1);
            reopened.setTransport(&transport);
            reopened.setOnline(true);
            QTRY_COMPARE(reopened.pendingOps(), 0);
        }
        QCOMPARE(reopened.pendingOps(), 0);
        QCOMPARE(server.received.size(), 2);
        QCOMPARE(server.received.last(), originalOp);
        QCOMPARE(server.applied, 1);
        reopened.flush();
        QTest::qWait(50);
        QCOMPARE(server.received.size(), 2);
    }
    void journalCreateOfflineRestartSyncAndArchive() {
        QTemporaryDir directory;
        const QString path = directory.filePath("cache.sqlite");
        const QString body = QStringLiteral("A private thought\nOnly the owner writes this.");
        const QVariantMap sections{{"now", QVariantList{QVariantMap{{"id", cardId}, {"title", "Existing task"}}}}, {"later", QVariantList{}}, {"missed", QVariantList{}}};
        QString journalId;
        {
            CardStore store;
            QVERIFY(store.open(path));
            QVERIFY(store.applyRemoteCards(sections));
            QSignalSpy resets(&store, &QAbstractItemModel::modelReset);
            QVERIFY(!store.createCard("   ", "", "journal"));
            QVERIFY(!store.createCard("Private", "", "external"));
            QCOMPARE(store.pendingOps(), 0);
            QVERIFY(store.createCard("  " + body + "  ", "", "journal"));
            journalId = store.cardIds().first();
            const auto card = store.data(store.index(0), CardStore::CardRole).toMap();
            QCOMPARE(card.value("source").toString(), QStringLiteral("journal"));
            QCOMPARE(store.sourceLabel(card), QStringLiteral("journal"));
            QCOMPARE(card.value("title").toString(), QStringLiteral("A private thought"));
            QCOMPARE(card.value("summary").toString(), QStringLiteral("Only the owner writes this."));
            QCOMPARE(resets.count(), 0);
        }
        OpsServer server;
        QVERIFY(server.listen(QHostAddress::LocalHost));
        HttpTransport transport(server.serverPort());
        CardStore restored;
        QVERIFY(restored.open(path));
        QCOMPARE(restored.cardIds(), QStringList({journalId, cardId}));
        // A snapshot racing reconnect must not hide the offline-created note.
        QVERIFY(restored.applyRemoteCards(sections));
        QCOMPARE(restored.cardIds(), QStringList({journalId, cardId}));
        restored.setTransport(&transport);
        restored.setOnline(true);
        QTRY_COMPARE(restored.pendingOps(), 0);
        QCOMPARE(server.received.first().value("args").toMap().value("body").toString(), body);
        QVERIFY(!restored.enqueueOp(journalId, "archive").isEmpty());
        QCOMPARE(restored.cardIds(), QStringList({cardId}));
        QTRY_COMPARE(restored.pendingOps(), 0);
        QCOMPARE(server.received.last().value("type").toString(), QStringLiteral("archive"));
        CardStore coldStart;
        QVERIFY(coldStart.open(path));
        QCOMPARE(coldStart.cardIds(), QStringList({cardId}));
    }
    void journalSaveDoesNotWaitForDatabaseLock() {
        QTemporaryDir directory;
        const QString path = directory.filePath("locked.sqlite");
        CardStore store;
        QVERIFY(store.open(path));
        const QString connection = QUuid::createUuid().toString();
        auto lock = QSqlDatabase::addDatabase("QSQLITE", connection);
        lock.setDatabaseName(path);
        QVERIFY(lock.open());
        bool heartbeat = false;
        QSignalSpy resets(&store, &QAbstractItemModel::modelReset);
        {
            QSqlQuery query(lock);
            QVERIFY(query.exec("BEGIN EXCLUSIVE"));
            QTimer::singleShot(0, &store, [&] { heartbeat = true; });
            QElapsedTimer timer;
            timer.start();
            QVERIFY(store.createCard("Private note", "", "journal"));
            QVERIFY2(timer.elapsed() < 100, "Journal creation waited for SQLite on the GUI thread");
            QTRY_VERIFY(heartbeat);
            QCOMPARE(store.sourceLabel(store.data(store.index(0), CardStore::CardRole).toMap()), QStringLiteral("journal"));
            QCOMPARE(resets.count(), 0);
            QVERIFY(query.exec("COMMIT"));
        }
        lock.close();
        lock = QSqlDatabase();
        QSqlDatabase::removeDatabase(connection);
    }
    void bodyPrefetchBoundsOutstandingAndAdvancesOnPersistence() {
        QTemporaryDir directory;
        CardStore store;
        QVERIFY(store.open(directory.filePath("prefetch.sqlite")));
        QVariantList cards;
        QStringList ids;
        for (int i = 0; i < 200; ++i) {
            const QString id = QUuid::createUuid().toString(QUuid::WithoutBraces);
            ids.append(id);
            cards.append(QVariantMap{{"id", id}, {"source", "mail"}, {"state", "open"}});
        }
        const QVariantMap sections{{"now", cards}, {"later", QVariantList{}}, {"missed", QVariantList{}}};
        store.setOnline(true);
        QSignalSpy requested(&store, &CardStore::requestMailBodyGet);
        QVERIFY(store.applyRemoteCards(sections));
        QCOMPARE(requested.size(), CardStore::kBodyWindow);
        QCOMPARE(store.mailSyncPending(), 200);
        QVERIFY(!store.mailSyncComplete());
        for (int i = 0; i < 3; ++i) QVERIFY(store.applyRemoteCards(sections));
        store.requestMailBody(ids.first());
        QCOMPARE(requested.size(), CardStore::kBodyWindow);
        store.applyRemoteMailBody(ids.first(), {{"html", "<p>First original</p>"}});
        // A slot stays occupied until SQLite commits; snapshots do not duplicate it.
        QCOMPARE(requested.size(), CardStore::kBodyWindow);
        QVERIFY(store.applyRemoteCards(sections));
        QTRY_COMPARE_WITH_TIMEOUT(requested.size(), CardStore::kBodyWindow + 1, 2000);
        QCOMPARE(requested.last().at(0).toString(), ids.at(CardStore::kBodyWindow));
        QCOMPARE(store.mailSyncPending(), 199);
        QCOMPARE(store.pendingOps(), 0);
        QCOMPARE(store.rowCount(), 200);
    }
    void bodyPrefetchDeduplicatesReconnectAndRetriesFailures() {
        QTemporaryDir directory;
        CardStore store;
        QVERIFY(store.open(directory.filePath("prefetch-dedup.sqlite")));
        const QString second = QUuid::createUuid().toString(QUuid::WithoutBraces);
        const QString third = QUuid::createUuid().toString(QUuid::WithoutBraces);
        const QVariantMap sections{{"now", QVariantList{
            QVariantMap{{"id", cardId}, {"source", "mail"}},
            QVariantMap{{"id", second}, {"source", "mail"}},
            QVariantMap{{"id", third}, {"source", "agent"}, {"has_body", true}}}},
            {"later", QVariantList{}}, {"missed", QVariantList{}}};
        store.setOnline(true);
        QSignalSpy requested(&store, &CardStore::requestMailBodyGet);
        QVERIFY(store.applyRemoteCards(sections));
        QCOMPARE(requested.size(), 3); // All three fit the wide window at once.
        for (int i = 0; i < 3; ++i) QVERIFY(store.applyRemoteCards(sections));
        store.requestMailBody(second);
        QCOMPARE(requested.size(), 3);
        store.setOnline(false);
        QVERIFY(store.applyRemoteCards(sections));
        store.setOnline(true);
        QCOMPARE(requested.size(), 3); // Live HTTP identities survive reconnect.
        store.reportMailBodyFailed(cardId, 503);
        store.applyRemoteMailBody(second, {{"html", "<p>Second original</p>"}});
        QTRY_COMPARE(store.mailSyncPending(), 2);
        QCOMPARE(requested.size(), 3); // Failed card has a real retry delay.
        QTRY_COMPARE_WITH_TIMEOUT(requested.size(), 4, 2500);
        QCOMPARE(requested.last().at(0).toString(), cardId);
        store.reportMailBodyFailed(cardId, 429);
        store.requestMailBody(cardId);
        QVERIFY(store.applyRemoteCards(sections));
        store.setOnline(false);
        store.setOnline(true);
        QTest::qWait(100);
        QCOMPARE(requested.size(), 4); // Shared rate-limit cooldown survives reconnect.
        QVERIFY(!store.mailSyncComplete());
        store.setOnline(false);
    }
    void bodyPrefetchPipelinesWideAndBatchesWrites() {
        // R9b: 400 bodies; the window is kBodyWindow wide, fetches complete
        // out of order, SQLite sees far fewer transactions than bodies, and
        // no client-side pacing.
        QTemporaryDir directory;
        const QString path = directory.filePath("wide.sqlite");
        constexpr int total = 400;
        QVariantList cards;
        QStringList ids;
        for (int i = 0; i < total; ++i) {
            ids << QUuid::createUuid().toString(QUuid::WithoutBraces);
            cards << QVariantMap{{"id", ids.last()}, {"source", "mail"}};
        }
        CardStore store;
        QVERIFY(store.open(path));
        QSet<QString> inFlight;
        int maxInFlight = 0, fetched = 0;
        connect(&store, &CardStore::requestMailBodyGet, &store, [&](const QString &id, const QString &) {
            ++fetched;
            inFlight.insert(id);
            maxInFlight = qMax(maxInFlight, int(inFlight.size()));
            QTimer::singleShot(5, &store, [&, id] {
                inFlight.remove(id);
                store.applyRemoteMailBody(id, {{"html", "<p>" + id + "</p>"}});
            });
        });
        QElapsedTimer clock;
        clock.start();
        store.setOnline(true);
        QVERIFY(store.applyRemoteCards({{"now", cards}, {"later", QVariantList{}}, {"missed", QVariantList{}}}));
        QTRY_COMPARE_WITH_TIMEOUT(store.mailSyncPending(), 0, 20000);
        qInfo() << "BENCH bodies" << fetched << "max_in_flight" << maxInFlight << "wall_ms" << clock.elapsed();
        QVERIFY(maxInFlight > 8);                       // wide, not two at a time
        QVERIFY(maxInFlight <= CardStore::kBodyWindow);
        QCOMPARE(fetched, total);
        QVERIFY(store.bodyWriteTransactions() < fetched / 4); // batched writes
    }
    void neverOpenedMailSynchronizesThroughHttpAndReopensOffline() {
        QTemporaryDir directory;
        const QString path = directory.filePath("never-opened.sqlite");
        const QString second = QUuid::createUuid().toString(QUuid::WithoutBraces);
        const QString third = QUuid::createUuid().toString(QUuid::WithoutBraces);
        const QVariantMap sections{{"now", QVariantList{
            QVariantMap{{"id", cardId}, {"source", "mail"}},
            QVariantMap{{"id", second}, {"source", "mail"}},
            QVariantMap{{"id", third}, {"source", "mail"}}}},
            {"later", QVariantList{}}, {"missed", QVariantList{}}};
        const QString image = QStringLiteral("data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=");
        const QVariantList messages{
            QVariantMap{{"id", "earlier"}, {"sender_name", "Earlier sender"}, {"html", "<p>Earlier original</p><img src='" + image + "'>"}},
            QVariantMap{{"id", "newest"}, {"html", "<p>Newest original</p>"}}};
        const QHash<QString, QVariantMap> bodies{
            {cardId, {{"html", "<p>Rich original</p><img src='" + image + "'>"}, {"messages", messages}, {"source_url", "https://mail.synthetic.test/thread"}}},
            {second, {{"html", "<pre>Plain text original\nSecond line</pre>"}, {"messages", QVariantList{}}}},
            {third, {{"html", "<p>Third never-opened original</p>"}, {"messages", QVariantList{}}}}};
        QTcpServer server;
        QVERIFY(server.listen(QHostAddress::LocalHost));
        QHash<QString, int> requests;
        connect(&server, &QTcpServer::newConnection, &server, [&] {
            while (server.hasPendingConnections()) {
                QTcpSocket *socket = server.nextPendingConnection();
                auto bytes = std::make_shared<QByteArray>();
                connect(socket, &QTcpSocket::readyRead, socket, [&, socket, bytes] {
                    bytes->append(socket->readAll());
                    if (!bytes->contains("\r\n\r\n")) return;
                    const QString route = QString::fromUtf8(bytes->left(bytes->indexOf("\r\n")).split(' ').value(1));
                    ++requests[route];
                    const QVariantMap payload = route == "/v1/cards" ? sections : bodies.value(route.section('/', 3, 3));
                    const QByteArray json = QJsonDocument(QJsonObject::fromVariantMap(payload)).toJson(QJsonDocument::Compact);
                    socket->write("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: " + QByteArray::number(json.size()) + "\r\nConnection: close\r\n\r\n" + json);
                    socket->disconnectFromHost();
                });
                connect(socket, &QTcpSocket::disconnected, socket, &QObject::deleteLater);
            }
        });
        {
            CardStore store;
            QVERIFY(store.open(path));
            Api api;
            api.setBaseUrl(QStringLiteral("http://127.0.0.1:%1").arg(server.serverPort()));
            QHash<int, quint64> snapshots;
            QHash<int, QString> downloads;
            connect(&store, &CardStore::requestCards, &api, [&](const QString &route, quint64 id) { snapshots.insert(api.get(route), id); });
            connect(&store, &CardStore::requestMailBodyGet, &api, [&](const QString &id, const QString &route) { downloads.insert(api.get(route), id); });
            connect(&api, &Api::requestFinished, &store, [&](int id, const QJsonValue &value, int) {
                if (downloads.contains(id)) store.applyRemoteMailBody(downloads.take(id), value.toObject().toVariantMap());
                else if (snapshots.contains(id)) store.applyRemoteCards(value.toObject().toVariantMap(), snapshots.take(id));
            });
            connect(&api, &Api::requestFailed, &store, [&](int id, int status, const QString &, const QJsonValue &) {
                if (downloads.contains(id)) store.reportMailBodyFailed(downloads.take(id), status);
            });
            store.setOnline(true); // Production sync entrypoint, no reader calls.
            // Loopback regression bound, not an application latency SLA. The old
            // three-second-per-body scheduler cannot complete this fixture here.
            QTRY_VERIFY_WITH_TIMEOUT(store.mailSyncComplete(), 2000);
            QCOMPARE(store.mailSyncPending(), 0);
            QCOMPARE(store.pendingOps(), 0);
            for (const QString &id : {cardId, second, third})
                QCOMPARE(requests.value("/v1/cards/" + id + "/body"), 1);
        }
        server.close(); // No transport is available during the first reader request.
        CardStore reopened;
        QVERIFY(reopened.open(path));
        QVERIFY(!reopened.online());
        QCOMPARE(reopened.rowCount(), 3);
        QSignalSpy requested(&reopened, &CardStore::requestMailBodyGet);
        QSignalSpy changed(&reopened, &CardStore::mailBodyChanged);
        for (const QString &id : {cardId, second, third}) {
            reopened.requestMailBody(id);
            const QVariantMap cached = reopened.cachedMailBody(id);
            QCOMPARE(cached.value("html"), bodies.value(id).value("html"));
            QCOMPARE(cached.value("messages"), bodies.value(id).value("messages"));
        }
        QCOMPARE(changed.size(), 3);
        QCOMPARE(requested.size(), 0);
        QCOMPARE(reopened.cachedMailBody(cardId).value("source_url"), bodies.value(cardId).value("source_url"));
        const QVariantMap original = reopened.cachedMailBody(cardId);
        reopened.applyRemoteMailBody(cardId, {{"html", " "}, {"messages", QVariantList{}}});
        reopened.reportMailBodyFailed(cardId, 503);
        QCOMPARE(reopened.cachedMailBody(cardId), original);
        QCOMPARE(reopened.pendingOps(), 0);
        CardStore retained;
        QVERIFY(retained.open(path));
        QCOMPARE(retained.cachedMailBody(cardId), original);
    }
    void interruptedBodySyncDoesNotClaimOfflineCompletion() {
        QTemporaryDir directory;
        const QString path = directory.filePath("interrupted.sqlite");
        const QString missing = QUuid::createUuid().toString(QUuid::WithoutBraces);
        {
            CardStore store;
            QVERIFY(store.open(path));
            store.setOnline(true);
            QVERIFY(store.applyRemoteCards({{"now", QVariantList{
                QVariantMap{{"id", cardId}, {"source", "mail"}},
                QVariantMap{{"id", missing}, {"source", "mail"}}}},
                {"later", QVariantList{}}, {"missed", QVariantList{}}}));
            QSignalSpy changed(&store, &CardStore::mailBodyChanged);
            store.applyRemoteMailBody(cardId, {{"html", "<p>Retained original</p>"},
                {"messages", QVariantList{QVariantMap{{"html", "<p>Earlier retained</p>"}}}}});
            QTRY_COMPARE(changed.size(), 1);
            store.setOnline(false);
            store.reportMailBodyFailed(missing, 0);
            QCOMPARE(store.mailSyncPending(), 1);
            QVERIFY(!store.mailSyncComplete());
        }
        CardStore reopened;
        QVERIFY(reopened.open(path));
        QCOMPARE(reopened.mailSyncPending(), 1);
        QVERIFY(!reopened.mailSyncComplete());
        QCOMPARE(reopened.cachedMailBody(cardId).value("html").toString(), QString("<p>Retained original</p>"));
        QCOMPARE(reopened.cachedMailBody(cardId).value("messages").toList().size(), 1);
        QVERIFY(reopened.cachedMailBody(missing).isEmpty());
    }
    void mailBodiesLoadFromSqliteOnDemandWithBoundedResidentBytes() {
        // R9 bodies on demand: opening a profile never loads body content;
        // reads are served from SQLite through an LRU bounded by bytes.
        QTemporaryDir directory;
        const QString path = directory.filePath("on-demand.sqlite");
        constexpr int count = 40;
        const QString filler(1024 * 1024, QLatin1Char('x')); // 2 MiB UTF-16 each, 80 MiB total
        QStringList ids;
        QVariantList cards;
        for (int i = 0; i < count; ++i) {
            ids << QUuid::createUuid().toString(QUuid::WithoutBraces);
            cards << QVariantMap{{"id", ids.last()}, {"source", "mail"}};
        }
        {
            CardStore store;
            QVERIFY(store.open(path));
            QVERIFY(store.applyRemoteCards({{"now", cards}, {"later", QVariantList{}}, {"missed", QVariantList{}}}));
            QSignalSpy changed(&store, &CardStore::mailBodyChanged);
            for (int i = 0; i < count; ++i)
                store.applyRemoteMailBody(ids.at(i), {{"html", QString::number(i) + filler},
                    {"messages", QVariantList{QVariantMap{{"html", "<p>earlier " + QString::number(i) + "</p>"}}}}});
            QTRY_COMPARE_WITH_TIMEOUT(changed.size(), count, 10000);
            QVERIFY(store.mailSyncPending() == 0);
        }
        CardStore reopened;
        QVERIFY(reopened.open(path));
        QCOMPARE(reopened.mailSyncPending(), 0); // sync metadata known without loading content
        QCOMPARE(reopened.residentBodyBytes(), qint64(0));
        for (int round = 0; round < 2; ++round)
            for (int i = 0; i < count; ++i) {
                const QVariantMap body = reopened.cachedMailBody(ids.at(i));
                QCOMPARE(body.value("html").toString(), QString::number(i) + filler);
                QCOMPARE(body.value("messages").toList().size(), 1);
                QVERIFY(reopened.residentBodyBytes() <= CardStore::kBodyLruBytes);
            }
        QVERIFY(reopened.residentBodyBytes() > 0);
        QVERIFY(reopened.cachedMailBody("missing").isEmpty());
    }
    void foregroundBodyUsesCacheAndPreservesItOnBadResponses() {
        QTemporaryDir directory;
        CardStore store;
        QVERIFY(store.open(directory.filePath("body-cache.sqlite")));
        const QString manual = QUuid::createUuid().toString(QUuid::WithoutBraces);
        QVERIFY(store.applyRemoteCards({{"now", QVariantList{
            QVariantMap{{"id", cardId}, {"source", "mail"}},
            QVariantMap{{"id", manual}, {"source", "manual"}}}},
            {"later", QVariantList{}}, {"missed", QVariantList{}}}));
        QSignalSpy requested(&store, &CardStore::requestMailBodyGet);
        QSignalSpy changed(&store, &CardStore::mailBodyChanged);
        QSignalSpy failed(&store, &CardStore::mailBodyFailed);
        store.setOnline(true);
        QCOMPARE(requested.size(), 1);
        // Unfetchable ids now report failure instead of hanging the reader.
        store.requestMailBody(manual);
        store.requestMailBody("missing");
        store.requestMailBody(cardId);
        QCOMPARE(requested.size(), 1);
        store.applyRemoteMailBody(cardId, {{"html", "   "}});
        QCOMPARE(failed.size(), 3);
        QVERIFY(store.cachedMailBody(cardId).isEmpty());
        store.requestMailBody(cardId);
        QCOMPARE(requested.size(), 2);
        store.applyRemoteMailBody(cardId, {{"html", "<p>Useful cached mail</p>"}});
        QTRY_COMPARE(changed.size(), 1);
        const QVariantMap useful = store.cachedMailBody(cardId);
        store.applyRemoteMailBody(cardId, {{"error", "rate limit exceeded"}});
        store.reportMailBodyFailed(cardId);
        QCOMPARE(failed.size(), 5);
        QCOMPARE(store.cachedMailBody(cardId), useful);
        store.requestMailBody(cardId);
        QCOMPARE(changed.size(), 2);
        QCOMPARE(requested.size(), 2);
        store.setOnline(false);
        store.requestMailBody(cardId);
        QCOMPARE(changed.size(), 3);
        QCOMPARE(requested.size(), 2);
        QCOMPARE(store.pendingOps(), 0);
        QCOMPARE(store.rowCount(), 2);
    }
    void unknownCardBodyRequestReportsFailure() {
        // A reader opened on an id the open-list snapshot no longer contains
        // (archived elsewhere, stale bundle row) must not hang with loading
        // stuck on: with no cached body and no fetch possible, report the
        // failure so the reader shows its error instead of a blank pane.
        QTemporaryDir directory;
        CardStore store;
        QVERIFY(store.open(directory.filePath("body-unknown.sqlite")));
        const QString manual = QUuid::createUuid().toString(QUuid::WithoutBraces);
        QVERIFY(store.applyRemoteCards({{"now", QVariantList{
            QVariantMap{{"id", cardId}, {"source", "mail"}},
            QVariantMap{{"id", manual}, {"source", "manual"}}}},
            {"later", QVariantList{}}, {"missed", QVariantList{}}}));
        store.setOnline(true);
        QSignalSpy requested(&store, &CardStore::requestMailBodyGet);
        QSignalSpy failed(&store, &CardStore::mailBodyFailed);
        // Unknown id: no request is possible, but the failure is reported so
        // MailDetailPage.reload stops loading and shows its error text.
        store.requestMailBody("no-such-card");
        QCOMPARE(requested.size(), 0);
        QCOMPARE(failed.size(), 1);
        QCOMPARE(failed.first().at(0).toString(), QString("no-such-card"));
        // Known card with no body route (non-mail, no has_body): same contract.
        store.requestMailBody(manual);
        QCOMPARE(requested.size(), 0);
        QCOMPARE(failed.size(), 2);
        QCOMPARE(failed.last().at(0).toString(), manual);
    }
    void archive429StaysDurableAndOptimisticThenAcksSameOp() {
        QTemporaryDir directory;
        const QString path = directory.filePath("rate-limited.sqlite");
        CardStore store;
        QVERIFY(store.open(path));
        const QString neighbor = QUuid::createUuid().toString(QUuid::WithoutBraces);
        const QVariantMap sections{{"now", QVariantList{
            QVariantMap{{"id", cardId}, {"source", "mail"}},
            QVariantMap{{"id", neighbor}, {"source", "mail"}}}},
            {"later", QVariantList{}}, {"missed", QVariantList{}}};
        QVERIFY(store.applyRemoteCards(sections));
        QSignalSpy posted(&store, &CardStore::requestPost);
        QSignalSpy failed(&store, &CardStore::operationFailed);
        store.setOnline(true);
        const QString op = store.enqueueOp(cardId, "archive");
        QVERIFY(!op.isEmpty());
        QTRY_COMPARE(posted.size(), 1);
        const QVariantMap payload = posted.first().at(2).toMap();
        QElapsedTimer retryElapsed;
        retryElapsed.start();
        store.reportPostResult(op, 429, {{"error", "rate limit exceeded"}});
        QCOMPARE(store.pendingOps(), 1);
        QCOMPARE(failed.size(), 0);
        QCOMPARE(store.cardIds(), QStringList{neighbor});
        QVERIFY(store.applyRemoteCards(sections));
        QCOMPARE(store.cardIds(), QStringList{neighbor});
        // Foreground navigation is independent of the operation's retry timer.
        QSignalSpy bodies(&store, &CardStore::requestMailBodyGet);
        // Eager sync already owns this request; finish that attempt before an
        // explicit reader retry, without bypassing the operation's429 timer.
        store.reportMailBodyFailed(neighbor, 503);
        store.requestMailBody(neighbor);
        QCOMPARE(bodies.size(), 1);
        QCOMPARE(bodies.first().at(0).toString(), neighbor);
        {
            CardStore coldStart;
            QVERIFY(coldStart.open(path));
            QCOMPARE(coldStart.pendingOps(), 1);
            QCOMPARE(coldStart.cardIds(), QStringList{neighbor});
            QVERIFY(coldStart.applyRemoteCards(sections));
            QCOMPARE(coldStart.cardIds(), QStringList{neighbor});
        }
        store.flush(); // an explicit flush must not bypass the cooldown
        QCOMPARE(posted.size(), 1);
        QCOMPARE(failed.size(), 0);
        // Exercise the real timer: 429 waits one bounded slice, not a minute.
        QTRY_COMPARE_WITH_TIMEOUT(posted.size(), 2, 5000);
        QVERIFY(retryElapsed.elapsed() >= 1000);
        QCOMPARE(posted.last().at(0).toString(), op);
        QCOMPARE(posted.last().at(2).toMap(), payload);
        QCOMPARE(store.pendingOps(), 1);
        QCOMPARE(store.cardIds(), QStringList{neighbor});
        store.reportPostResult(op, 200, {{"ok", true}});
        QTRY_COMPARE(store.pendingOps(), 0);
        QCOMPARE(failed.size(), 0);
        QVERIFY(store.applyRemoteCards(sections));
        QCOMPARE(store.cardIds(), QStringList{neighbor});
        CardStore acknowledged;
        QVERIFY(acknowledged.open(path));
        QCOMPARE(acknowledged.pendingOps(), 0);
        QCOMPARE(acknowledged.cardIds(), QStringList{neighbor});
    }
    void hasBodyAgentCardPrefetchesAndSurvivesRestart() {
        QTemporaryDir directory;
        QVERIFY(directory.isValid());
        const QString path = directory.filePath(QStringLiteral("cache.sqlite"));
        const QString agentId = QStringLiteral("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa");
        const QString manualId = QStringLiteral("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb");
        const QString mailId = QStringLiteral("cccccccc-cccc-4ccc-8ccc-cccccccccccc");
        const QVariantMap agent{{QStringLiteral("id"), agentId}, {QStringLiteral("title"), QStringLiteral("Research")},
            {QStringLiteral("source"), QStringLiteral("agent")}, {QStringLiteral("has_body"), true}};
        const QVariantMap manual{{QStringLiteral("id"), manualId}, {QStringLiteral("title"), QStringLiteral("Plain")},
            {QStringLiteral("source"), QStringLiteral("manual")}};
        const QVariantMap mail{{QStringLiteral("id"), mailId}, {QStringLiteral("title"), QStringLiteral("Mail")},
            {QStringLiteral("source"), QStringLiteral("mail")}};
        const QVariantMap sections{{QStringLiteral("now"), QVariantList{agent, manual, mail}},
            {QStringLiteral("later"), QVariantList{}}, {QStringLiteral("missed"), QVariantList{}}};
        const QString bodyHtml = QStringLiteral("<pre>full result</pre>");
        {
            CardStore store;
            QVERIFY(store.open(path));
            store.setOnline(true);
            QSignalSpy prefetch(&store, &CardStore::requestMailBodyGet);
            QVERIFY(store.applyRemoteCards(sections));
            QCOMPARE(prefetch.size(), 2);
            store.requestMailBody(mailId);
            QCOMPARE(prefetch.size(), 2);
            QSet<QString> ids;
            for (const QVariantList &args : prefetch) ids.insert(args.first().toString());
            QVERIFY(ids.contains(agentId));
            QVERIFY(ids.contains(mailId));
            QVERIFY(!ids.contains(manualId));
            // Plain cards without has_body have no fetchable body.
            store.requestMailBody(manualId);
            QCOMPARE(prefetch.size(), 2);
            store.requestMailBody(agentId);
            QCOMPARE(prefetch.size(), 2); // Already in flight: no duplicate.
            store.applyRemoteMailBody(agentId, {{QStringLiteral("html"), bodyHtml}});
            QCOMPARE(store.cachedMailBody(agentId).value(QStringLiteral("html")).toString(), bodyHtml);
            QCOMPARE(store.cachedMailBody(agentId).value(QStringLiteral("source_url")).toString(), QStringLiteral(""));
        }
        CardStore restored;
        QVERIFY(restored.open(path));
        QVERIFY(!restored.online());
        QCOMPARE(restored.cachedMailBody(agentId).value(QStringLiteral("html")).toString(), bodyHtml);
        QCOMPARE(restored.cachedMailBody(agentId).value(QStringLiteral("source_url")).toString(), QStringLiteral(""));
        QVERIFY(restored.cachedMailBody(manualId).isEmpty());
    }
    void originalThreadHistorySurvivesRestartAndExplicitEmptyClears() {
        QTemporaryDir directory;
        QVERIFY(directory.isValid());
        const QString path = directory.filePath(QStringLiteral("thread-history.sqlite"));
        const QVariantList originals{
            QVariantMap{{"id", "old-1"}, {"gmail_message_id", "gmail-old-1"}, {"sender_name", "Older writer"},
                {"sender_address", "older@synthetic.test"}, {"received_at", "2026-10-16T08:00:00Z"}, {"html", "<p>First original</p>"}},
            QVariantMap{{"id", "new-2"}, {"gmail_message_id", "gmail-new-2"}, {"sender_name", "Newest writer"},
                {"sender_address", "newest@synthetic.test"}, {"received_at", "2026-10-17T09:30:00Z"}, {"html", "<p>Newest original</p>"}}
        };
        const QString card = QStringLiteral("dddddddd-dddd-4ddd-8ddd-dddddddddddd");
        {
            CardStore store;
            QVERIFY(store.open(path));
            store.applyRemoteMailBody(card, {{"html", "<p>Newest original</p>"},
                {"source_url", "https://mail.synthetic.test/thread/fixture"}, {"messages", originals}});
            QSignalSpy changed(&store, &CardStore::mailBodyChanged);
            QTRY_COMPARE_WITH_TIMEOUT(changed.size(), 1, 5000);
            QCOMPARE(store.cachedMailBody(card).value("messages").toList(), originals);
            store.applyRemoteMailBody(card, {{"html", "<p>Newest original</p>"}, {"source_url", "https://mail.synthetic.test/thread/fixture"}});
            QTRY_COMPARE_WITH_TIMEOUT(changed.size(), 2, 5000);
            QCOMPARE(store.cachedMailBody(card).value("messages").toList(), originals);
        }
        {
            CardStore restored;
            QVERIFY(restored.open(path));
            const QVariantMap body = restored.cachedMailBody(card);
            QCOMPARE(body.value("messages").toList(), originals);
            QCOMPARE(body.value("html").toString(), QStringLiteral("<p>Newest original</p>"));
            QCOMPARE(body.value("source_url").toString(), QStringLiteral("https://mail.synthetic.test/thread/fixture"));
            restored.applyRemoteMailBody(card, {{"html", "<p>Newest original</p>"},
                {"source_url", "https://mail.synthetic.test/thread/fixture"}, {"messages", QVariantList{}}});
            QSignalSpy changed(&restored, &CardStore::mailBodyChanged);
            QTRY_COMPARE_WITH_TIMEOUT(changed.size(), 1, 5000);
            QCOMPARE(restored.cachedMailBody(card).value("messages").toList(), QVariantList{});
            QCOMPARE(restored.cachedMailBody(card).value("html").toString(), QStringLiteral("<p>Newest original</p>"));
        }
        {
            CardStore restoredAgain;
            QVERIFY(restoredAgain.open(path));
            QCOMPARE(restoredAgain.cachedMailBody(card).value("messages").toList(), QVariantList{});
            QVERIFY(restoredAgain.cachedMailBody(QStringLiteral("eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee")).isEmpty());
        }
    }
    void legacyMailBodyMigrationPreservesHtmlAndSourceUrl() {
        QTemporaryDir directory;
        QVERIFY(directory.isValid());
        const QString path = directory.filePath(QStringLiteral("legacy-mail.sqlite"));
        const QString card = QStringLiteral("ffffffff-ffff-4fff-8fff-ffffffffffff");
        {
            QSqlDatabase db = QSqlDatabase::addDatabase(QStringLiteral("QSQLITE"), QStringLiteral("legacy-fixture"));
            db.setDatabaseName(path);
            QVERIFY(db.open());
            {
                QSqlQuery query(db);
                QVERIFY(query.exec(QStringLiteral("CREATE TABLE mail_bodies(card_id TEXT PRIMARY KEY, html TEXT NOT NULL, source_url TEXT NOT NULL)")));
                query.prepare(QStringLiteral("INSERT INTO mail_bodies(card_id,html,source_url) VALUES(?,?,?)"));
                query.addBindValue(card);
                query.addBindValue(QStringLiteral("<p>Legacy cached original</p>"));
                query.addBindValue(QStringLiteral("https://mail.synthetic.test/legacy"));
                QVERIFY(query.exec());
            }
             db.close();
        }
        CardStore store;
        QVERIFY(store.open(path));
        const QVariantMap cached = store.cachedMailBody(card);
        QCOMPARE(cached.value("html").toString(), QStringLiteral("<p>Legacy cached original</p>"));
        QCOMPARE(cached.value("source_url").toString(), QStringLiteral("https://mail.synthetic.test/legacy"));
        QVERIFY(cached.contains("messages"));
        QCOMPARE(cached.value("messages").toList(), QVariantList{});
    }
    void openCachedFileWritesBytesAndRejectsUnsafe() {
        QStandardPaths::setTestModeEnabled(true);
        QTemporaryDir directory;
        QVERIFY(directory.isValid());
        const QString path = directory.filePath(QStringLiteral("cache.sqlite"));
        const QString agentId = QStringLiteral("dddddddd-dddd-4ddd-8ddd-dddddddddddd");
        const QString namelessId = QStringLiteral("eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee");
        const QString traversalId = QStringLiteral("ffffffff-ffff-4fff-8fff-ffffffffffff");
        const QString hiddenId = QStringLiteral("99999999-9999-4999-8999-999999999999");
        const QVariantMap sections{{QStringLiteral("now"), QVariantList{
                QVariantMap{{QStringLiteral("id"), agentId}, {QStringLiteral("title"), QStringLiteral("Research")},
                    {QStringLiteral("source"), QStringLiteral("agent")}, {QStringLiteral("has_body"), true}},
                QVariantMap{{QStringLiteral("id"), namelessId}, {QStringLiteral("title"), QStringLiteral("Nameless")},
                    {QStringLiteral("source"), QStringLiteral("agent")}, {QStringLiteral("has_body"), true}},
                QVariantMap{{QStringLiteral("id"), traversalId}, {QStringLiteral("title"), QStringLiteral("Traversal")},
                    {QStringLiteral("source"), QStringLiteral("agent")}, {QStringLiteral("has_body"), true}},
                QVariantMap{{QStringLiteral("id"), hiddenId}, {QStringLiteral("title"), QStringLiteral("Hidden")},
                    {QStringLiteral("source"), QStringLiteral("agent")}, {QStringLiteral("has_body"), true}}}},
            {QStringLiteral("later"), QVariantList{}}, {QStringLiteral("missed"), QVariantList{}}};
        CardStore store;
        QVERIFY(store.open(path));
        QVERIFY(store.applyRemoteCards(sections));
        const QByteArray original = QByteArray("useful-report-bytes offline");
        const QString b64 = QString::fromLatin1(original.toBase64());
        const QString goodUrl = QStringLiteral("data:application/pdf;name=report.pdf;base64,") + b64;
        const QString goodHtml = QStringLiteral("<pre>full summary</pre><a href='") + goodUrl + QStringLiteral("'>report.pdf</a>");
        store.applyRemoteMailBody(agentId, {{QStringLiteral("html"), goodHtml}});
        const QString localUrl = store.openCachedFile(agentId, goodUrl);
        QVERIFY(!localUrl.isEmpty());
        const QString localPath = QUrl(localUrl).toLocalFile();
        QVERIFY(!localPath.isEmpty());
        QVERIFY(localPath.contains(agentId));
        QVERIFY(localPath.endsWith(QStringLiteral("report.pdf")));
        QFile file(localPath);
        QVERIFY(file.open(QIODevice::ReadOnly));
        QCOMPARE(file.readAll(), original);
        file.close();
        // A data: URL that is not part of this card's cached HTML is rejected.
        const QString otherUrl = QStringLiteral("data:application/pdf;name=other.pdf;base64,") + b64;
        QCOMPARE(store.openCachedFile(agentId, otherUrl), QString());
        QCOMPARE(store.openCachedFile(agentId, QStringLiteral("https://example.test/x")), QString());
        QCOMPARE(store.openCachedFile(QStringLiteral("00000000-0000-4000-8000-000000000000"), goodUrl), QString());
        // A cached link without a name= parameter carries no safe filename.
        const QString namelessUrl = QStringLiteral("data:application/pdf;base64,") + b64;
        store.applyRemoteMailBody(namelessId, {{QStringLiteral("html"), namelessUrl}});
        QCOMPARE(store.cachedMailBody(namelessId).value(QStringLiteral("html")).toString(), namelessUrl);
        QCOMPARE(store.openCachedFile(namelessId, namelessUrl), QString());
        // Traversal and dotfile names are rejected even when cached verbatim.
        const QString traversalUrl = QStringLiteral("data:application/pdf;name=%2E%2E%2Fevil.pdf;base64,") + b64;
        store.applyRemoteMailBody(traversalId, {{QStringLiteral("html"), traversalUrl}});
        QCOMPARE(store.cachedMailBody(traversalId).value(QStringLiteral("html")).toString(), traversalUrl);
        QCOMPARE(store.openCachedFile(traversalId, traversalUrl), QString());
        const QString hiddenUrl = QStringLiteral("data:application/pdf;name=.hidden;base64,") + b64;
        store.applyRemoteMailBody(hiddenId, {{QStringLiteral("html"), hiddenUrl}});
        QCOMPARE(store.cachedMailBody(hiddenId).value(QStringLiteral("html")).toString(), hiddenUrl);
        QCOMPARE(store.openCachedFile(hiddenId, hiddenUrl), QString());
    }
    void reconnectSnapshotsAcrossArchiveAckDiscoverUnseenCard() {
        QTemporaryDir directory;
        CardStore store;
        QVERIFY(store.open(directory.filePath("reconnect-snapshot.sqlite")));
        const QVariantMap mail{{"id", cardId}, {"source", "mail"}, {"state", "open"}};
        const QVariantMap unseen{{"id", "B"}, {"source", "manual"}, {"state", "open"}};
        const QVariantMap full{{"now", QVariantList{mail}}, {"later", QVariantList{}}, {"missed", QVariantList{}}};
        const QVariantMap committed{{"now", QVariantList{unseen}}, {"later", QVariantList{}}, {"missed", QVariantList{}}};
        QVERIFY(store.applyRemoteCards(full));
        const QString archive = store.enqueueOp(cardId, "archive");
        SnapshotConsumer snapshots(store);
        QSignalSpy posted(&store, &CardStore::requestPost);
        store.setOnline(true);
        const quint64 reconnect = snapshots.latestRequest;
        QTRY_COMPARE(posted.size(), 1);
        store.refresh(); // Archive SSE event requests another snapshot before ACK.
        const quint64 sse = snapshots.latestRequest;
        store.reportPostResult(archive, 200, {{"ok", true}});
        QTRY_COMPARE(store.pendingOps(), 0);
        snapshots.apply(reconnect, committed);
        QCOMPARE(store.cardIds(), QStringList{"B"});
        snapshots.apply(sse, committed);
        QCOMPARE(store.cardIds(), QStringList{"B"});
        // Older responses cannot undo a newer response; a drop fetches a replacement.
        store.refresh();
        const quint64 newer = snapshots.latestRequest;
        const QVariantMap later{{"now", QVariantList{unseen, QVariantMap{{"id", "C"}, {"source", "manual"}}}},
            {"later", QVariantList{}}, {"missed", QVariantList{}}};
        QVERIFY(snapshots.apply(newer, later));
        QVERIFY(!snapshots.apply(sse, committed));
        QVERIFY(snapshots.latestRequest > newer);
        QCOMPARE(store.cardIds(), QStringList({"B", "C"}));
    }
    void postInverseAckSnapshotWithoutCardReleasesRestoreOverlay() {
        QTemporaryDir directory;
        CardStore store;
        QVERIFY(store.open(directory.filePath("fresh-absent-snapshot.sqlite")));
        const QVariantMap full{{"now", QVariantList{QVariantMap{{"id", cardId}, {"source", "mail"}, {"state", "open"}}}},
            {"later", QVariantList{}}, {"missed", QVariantList{}}};
        const QVariantMap absent{{"now", QVariantList{}}, {"later", QVariantList{}}, {"missed", QVariantList{}}};
        QVERIFY(store.applyRemoteCards(full));
        SnapshotConsumer snapshots(store);
        QSignalSpy posted(&store, &CardStore::requestPost);
        store.setOnline(true);
        const QString archive = store.enqueueOp(cardId, "archive");
        QTRY_COMPARE(posted.size(), 1);
        store.reportPostResult(archive, 200, {{"ok", true}});
        QTRY_COMPARE(store.pendingOps(), 0);
        QVERIFY(store.undoBundleArchive(archive));
        QTRY_COMPARE(posted.size(), 2);
        store.refresh();
        const quint64 beforeAck = snapshots.latestRequest;
        store.reportPostResult(posted.last().at(0).toString(), 200, {{"ok", true}});
        QTRY_COMPARE(store.pendingOps(), 0);
        const quint64 afterAck = snapshots.latestRequest;
        QVERIFY(afterAck > beforeAck);
        snapshots.apply(beforeAck, absent);
        QCOMPARE(store.cardIds(), QStringList{cardId});
        // Another device archived it before any confirming full snapshot arrived.
        snapshots.apply(afterAck, absent);
        QCOMPARE(store.rowCount(), 0);
        store.refresh();
        QVERIFY(snapshots.apply(snapshots.latestRequest, absent));
        QCOMPARE(store.rowCount(), 0); // No retained mutation reinserts a ghost.
    }
    void lostAckUndoAuthFailuresRemainArchivedAndQueued_data() {
        QTest::addColumn<int>("status");
        QTest::addColumn<bool>("inverseFailure");
        for (int status : {401, 403, 500, 0}) {
            QTest::newRow(qPrintable(QString::number(status) + "-archive-retry")) << status << false;
            QTest::newRow(qPrintable(QString::number(status) + "-inverse")) << status << true;
        }
    }
    void lostAckUndoAuthFailuresRemainArchivedAndQueued() {
        QFETCH(int, status);
        QFETCH(bool, inverseFailure);
        QTemporaryDir directory;
        const QString path = directory.filePath("lost-ack-auth.sqlite");
        CardStore store;
        QVERIFY(store.open(path));
        const QVariantMap full{{"now", QVariantList{QVariantMap{{"id", cardId}, {"source", "mail"}, {"state", "open"}}}},
            {"later", QVariantList{}}, {"missed", QVariantList{}}};
        const QVariantMap absent{{"now", QVariantList{}}, {"later", QVariantList{}}, {"missed", QVariantList{}}};
        QVERIFY(store.applyRemoteCards(full));
        QSignalSpy posted(&store, &CardStore::requestPost);
        QSignalSpy failed(&store, &CardStore::operationFailed);
        store.setOnline(true);
        const QString archive = store.enqueueOp(cardId, "archive");
        QTRY_COMPARE(posted.size(), 1);
        store.reportPostResult(archive, 0, {}); // Server committed; ACK was lost.
        QVERIFY(store.undoBundleArchive(archive));
        QTRY_COMPARE(posted.size(), 2);
        QCOMPARE(posted.last().at(0).toString(), archive);
        if (inverseFailure) {
            store.reportPostResult(archive, 200, {{"ok", true}}); // Receipt-deduplicated retry.
            QTRY_COMPARE(posted.size(), 3);
        }
        store.reportPostResult(posted.last().at(0).toString(), status, {{"error", "unavailable"}});
        QCOMPARE(store.cardIds(), QStringList{});
        QCOMPARE(store.pendingOps(), inverseFailure ? 1 : 2);
        QCOMPARE(failed.size(), 0);
        QVERIFY(store.applyRemoteCards(absent));
        QCOMPARE(store.rowCount(), 0);
        QVERIFY(store.applyRemoteCards(full)); // Even a stale open snapshot cannot resurrect it.
        QCOMPARE(store.rowCount(), 0);
        // Archive must remain ahead of its inverse, so an inverse 401 cannot
        // overtake a retry that is still unauthenticated.
        if (!inverseFailure) {
            CardStore durable;
            QVERIFY(durable.open(path));
            QSignalSpy replayed(&durable, &CardStore::requestPost);
            durable.setOnline(true);
            QTRY_COMPARE(replayed.size(), 1);
            QCOMPARE(replayed.first().at(0).toString(), archive);
            QCOMPARE(durable.pendingOps(), 2);
        }
        // Re-authentication resumes the same ordered operations, not replacements.
        const int beforeRetry = posted.size();
        QTRY_COMPARE_WITH_TIMEOUT(posted.size(), beforeRetry + 1, 5000);
        store.reportPostResult(posted.last().at(0).toString(), 200, {{"ok", true}});
        if (!inverseFailure) {
            QTRY_COMPARE(posted.size(), beforeRetry + 2);
            store.reportPostResult(posted.last().at(0).toString(), 200, {{"ok", true}});
        }
        QTRY_COMPARE(store.pendingOps(), 0);
        QVERIFY(store.applyRemoteCards(full));
        QCOMPARE(store.cardIds(), QStringList{cardId});
    }
    void delayedArchivedSnapshotAfterInverseAckKeepsRestoredCard() {
        QTemporaryDir directory;
        const QString path = directory.filePath("delayed-snapshot.sqlite");
        const QVariantMap full{{"now", QVariantList{QVariantMap{{"id", cardId}, {"source", "mail"}, {"state", "open"}}}},
            {"later", QVariantList{}}, {"missed", QVariantList{}}};
        const QVariantMap absent{{"now", QVariantList{}}, {"later", QVariantList{}}, {"missed", QVariantList{}}};
        {
            CardStore store;
            QVERIFY(store.open(path));
            QVERIFY(store.applyRemoteCards(full));
            SnapshotConsumer snapshots(store);
            QSignalSpy posted(&store, &CardStore::requestPost);
            store.setOnline(true);
            const QString archive = store.enqueueOp(cardId, "archive");
            QTRY_COMPARE(posted.size(), 1);
            store.reportPostResult(archive, 200, {{"ok", true}});
            QTRY_COMPARE(store.pendingOps(), 0);
            // This archived snapshot was requested before Undo and returns last.
            store.refresh();
            const quint64 delayed = snapshots.latestRequest;
            QVERIFY(store.undoBundleArchive(archive));
            QTRY_COMPARE(posted.size(), 2);
            store.reportPostResult(posted.last().at(0).toString(), 200, {{"ok", true}});
            QTRY_COMPARE(store.pendingOps(), 0);
            QVERIFY(snapshots.apply(delayed, absent));
            QCOMPARE(store.cardIds(), QStringList{cardId});
            QVERIFY(store.applyRemoteCards(full));
            QCOMPARE(store.cardIds(), QStringList{cardId});
            // A later real archive must still be allowed to remove the row.
            QVERIFY(store.applyRemoteCards(absent));
            QCOMPARE(store.rowCount(), 0);
            QVERIFY(store.applyRemoteCards(full));
        }
        CardStore coldStart;
        QVERIFY(coldStart.open(path));
        QCOMPARE(coldStart.cardIds(), QStringList{cardId});
    }
    void undoInflightArchiveThenBothRejectedKeepsOpenCard() {
        QTemporaryDir directory;
        CardStore store;
        QVERIFY(store.open(directory.filePath("rejected-archive-undo.sqlite")));
        const QVariantMap full{{"now", QVariantList{QVariantMap{{"id", cardId}, {"source", "mail"}, {"state", "open"}}}},
            {"later", QVariantList{}}, {"missed", QVariantList{}}};
        QVERIFY(store.applyRemoteCards(full));
        QSignalSpy posted(&store, &CardStore::requestPost);
        QSignalSpy refreshes(&store, &CardStore::requestCards);
        QSignalSpy failed(&store, &CardStore::operationFailed);
        store.setOnline(true);
        const QString archive = store.enqueueOp(cardId, "archive");
        QTRY_COMPARE(posted.size(), 1);
        QVERIFY(store.undoBundleArchive(archive));
        store.reportPostResult(archive, 422, {{"ok", false}});
        QTRY_COMPARE(posted.size(), 2);
        // The archive transaction rolled back; its refresh reports open.
        QVERIFY(store.applyRemoteCards(full));
        const int beforeRejection = refreshes.size();
        store.reportPostResult(posted.last().at(0).toString(), 422, {{"ok", false}});
        QTRY_COMPARE(store.pendingOps(), 0);
        QCOMPARE(store.cardIds(), QStringList{cardId});
        QCOMPARE(failed.size(), 2);
        QVERIFY(refreshes.size() > beforeRejection);
    }
    void partialBundleRejectionsThenUndoPreservesFullOrder() {
        QTemporaryDir directory;
        CardStore store;
        QVERIFY(store.open(directory.filePath("partial-bundle.sqlite")));
        const auto mail = [](const QString &id) {
            return QVariantMap{{"id", id}, {"source", "mail"}, {"state", "open"}, {"bundle_id", "bundle"}};
        };
        QVariantMap pin = mail("P");
        pin.insert("pinned_rank", 1);
        QVERIFY(store.applyRemoteCards({{"now", QVariantList{pin, mail("A"), mail("B"), mail("C")}},
            {"later", QVariantList{}}, {"missed", QVariantList{}}}));
        QSignalSpy posted(&store, &CardStore::requestPost);
        store.setOnline(true);
        const QString token = store.archiveBundleNow("bundle").value("token").toString();
        // The bounded pipeline sends the whole bundle at once, not one op
        // per round trip; rejections then restore cards independently.
        QTRY_COMPARE(posted.size(), 3);
        store.reportPostResult(posted[0].at(0).toString(), 422, {{"ok", false}});
        QTRY_COMPARE(store.pendingOps(), 2);
        // No further transmissions after failures: the third op is already in
        // flight, and going offline parks everything still unsent.
        store.setOnline(false);
        store.reportPostResult(posted[1].at(0).toString(), 422, {{"ok", false}});
        QTRY_COMPARE(store.pendingOps(), 1);
        QVERIFY(store.undoBundleArchive(token));
        QCOMPARE(store.cardIds(), QStringList({"P", "A", "B", "C"}));
        // C was already sent, so Undo queues its INBOX re-add on top of the
        // still unacknowledged original; nothing unsent remains.
        QCOMPARE(store.pendingOps(), 2);
        QCOMPARE(posted.size(), 3);
    }

    void singleArchiveUndoAcrossAcknowledgement_data() {
        QTest::addColumn<QString>("phase");
        for (const QString &phase : {"queued", "acknowledged", "inflight", "ack-callback", "lost-ack", "inverse-rejected", "expired"})
            QTest::newRow(qPrintable(phase)) << phase;
    }
    void singleArchiveUndoAcrossAcknowledgement() {
        QFETCH(QString, phase);
        QTemporaryDir directory;
        CardStore store;
        QVERIFY(store.open(directory.filePath("single-undo.sqlite")));
        const QString neighbor = QUuid::createUuid().toString(QUuid::WithoutBraces);
        const QString pinned = QUuid::createUuid().toString(QUuid::WithoutBraces);
        const QVariantMap pin{{"id", pinned}, {"source", "mail"}, {"state", "open"}, {"pinned_rank", 1}};
        const QVariantMap other{{"id", neighbor}, {"source", "manual"}, {"state", "open"}, {"note", "Untouched"}};
        const QVariantMap mail{{"id", cardId}, {"source", "mail"}, {"state", "open"}};
        const QVariantMap full{{"now", QVariantList{pin, other, mail}}, {"later", QVariantList{}}, {"missed", QVariantList{}}};
        const QVariantMap absent{{"now", QVariantList{pin, other}}, {"later", QVariantList{}}, {"missed", QVariantList{}}};
        QVERIFY(store.applyRemoteCards(full));
        const QStringList original{pinned, neighbor, cardId};
        QSignalSpy posted(&store, &CardStore::requestPost);
        QSignalSpy failed(&store, &CardStore::operationFailed);
        if (phase == "expired") store.setBundleArchiveUndoDurationForTest(1);
        if (phase != "queued" && phase != "expired") store.setOnline(true);
        const QString op = store.enqueueOp(cardId, "archive");
        QVERIFY(!op.isEmpty());
        QCOMPARE(store.cardIds(), QStringList({pinned, neighbor}));
        if (phase != "queued" && phase != "expired") {
            QTRY_COMPARE(posted.size(), 1);
            if (phase == "acknowledged" || phase == "inverse-rejected") {
                store.reportPostResult(op, 200, {{"ok", true}});
                QTRY_COMPARE(store.pendingOps(), 0);
            } else if (phase == "ack-callback") {
                store.reportPostResult(op, 200, {{"ok", true}});
            } else if (phase == "lost-ack") {
                store.reportPostResult(op, 0, {});
            }
        }
        if (phase == "expired") {
            QTest::qWait(5);
            QVERIFY(!store.undoBundleArchive(op));
            QCOMPARE(store.cardIds(), QStringList({pinned, neighbor}));
            return;
        }
        QVERIFY2(store.undoBundleArchive(op), "Single mail archive must retain its Undo token across the actual ACK transition");
        QCOMPARE(store.cardIds(), original);
        QVERIFY(!store.undoBundleArchive(op));
        QVERIFY(!store.undoBundleArchive("unknown-token"));
        if (phase == "queued") {
            QCOMPARE(store.pendingOps(), 0);
            store.setOnline(true);
            QTest::qWait(50);
            QCOMPARE(posted.size(), 0);
        } else {
            // The remote archived snapshot cannot mask or reorder the inverse.
            QVERIFY(store.applyRemoteCards(absent));
            QCOMPARE(store.cardIds(), original);
            if (phase == "inflight") store.reportPostResult(op, 200, {{"ok", true}});
            if (phase == "lost-ack") {
                QTRY_COMPARE(posted.size(), 2);
                QCOMPARE(posted.last().at(0).toString(), op);
                store.reportPostResult(op, 200, {{"ok", true}});
            }
            const int expected = phase == "lost-ack" ? 3 : 2;
            QTRY_COMPARE(posted.size(), expected);
            const QVariantMap inverse = posted.last().at(2).toMap();
            QCOMPARE(inverse.value("type").toString(), QStringLiteral("gmail.label_add"));
            QCOMPARE(inverse.value("card_id").toString(), cardId);
            QCOMPARE(inverse.value("args").toMap().value("label").toString(), QStringLiteral("INBOX"));
            QVERIFY(store.applyRemoteCards(absent));
            QCOMPARE(store.cardIds(), original);
            store.reportPostResult(inverse.value("op_id").toString(), phase == "inverse-rejected" ? 422 : 200, {{"ok", phase != "inverse-rejected"}});
            QTRY_COMPARE(store.pendingOps(), 0);
            if (phase == "inverse-rejected") {
                QCOMPARE(failed.size(), 1);
                QCOMPARE(store.cardIds(), QStringList({pinned, neighbor}));
                QVERIFY(store.applyRemoteCards(absent));
                QCOMPARE(store.cardIds(), QStringList({pinned, neighbor}));
                return;
            }
        }
        QVERIFY(store.applyRemoteCards(full));
        QCOMPARE(store.cardIds(), original);
        QCOMPARE(store.data(store.index(1), CardStore::CardRole).toMap().value("note").toString(), QStringLiteral("Untouched"));
        QCOMPARE(failed.size(), 0);
    }

    // Archive bundle acts at once; Undo puts the cards back in order and
    // cancels their queued archive ops, or queues the INBOX re-add for ops
    // that were already sent. Invoked by name so this test fails (instead of
    // not compiling) on a store without the API.
    void bundleArchiveUndoRestoresOrderAndReversesOps() {
        QTemporaryDir directory;
        const QString path = directory.filePath("bundle-undo.sqlite");
        const QString bundle = QStringLiteral("99999999-9999-4999-8999-999999999999");
        const QString pinned = QStringLiteral("aaaaaaaa-0000-4000-8000-000000000001");
        const QString loose = QStringLiteral("aaaaaaaa-0000-4000-8000-000000000002");
        const QString first = QStringLiteral("aaaaaaaa-0000-4000-8000-000000000003");
        const QString middle = QStringLiteral("aaaaaaaa-0000-4000-8000-000000000004");
        const QString last = QStringLiteral("aaaaaaaa-0000-4000-8000-000000000005");
        const auto mail = [&](const QString &id, const QVariant &bundleId) {
            return QVariantMap{{"id", id}, {"title", id}, {"source", "mail"}, {"state", "open"}, {"bundle_id", bundleId}};
        };
        QVariantMap pinnedCard = mail(pinned, bundle);
        pinnedCard.insert("pinned_rank", 1);
        CardStore store;
        QVERIFY(store.open(path));
        QVERIFY(store.applyRemoteCards({{"now", QVariantList{mail(loose, QVariant()), mail(first, bundle), pinnedCard,
            mail(middle, QVariant()), mail(last, bundle)}}, {"later", QVariantList{}}, {"missed", QVariantList{}}}));
        const QStringList original{pinned, loose, first, middle, last};
        QCOMPARE(store.cardIds(), original);
        const auto durableOps = [&] {
            QStringList ops;
            const QString connection = QUuid::createUuid().toString();
            {
                auto db = QSqlDatabase::addDatabase("QSQLITE", connection);
                db.setDatabaseName(path);
                QSqlQuery q(db);
                if (db.open() && q.exec("SELECT payload FROM outbox ORDER BY seq"))
                    while (q.next()) {
                        const QVariantMap op = QJsonDocument::fromJson(q.value(0).toByteArray()).object().toVariantMap();
                        ops.append(op.value("type").toString() + ":" + op.value("card_id").toString());
                    }
            }
            QSqlDatabase::removeDatabase(connection);
            return ops;
        };
        const auto archiveNow = [&](QVariantMap &result) {
            return QMetaObject::invokeMethod(&store, "archiveBundleNow", Q_RETURN_ARG(QVariantMap, result), Q_ARG(QString, bundle));
        };
        const auto undo = [&](const QString &token) {
            bool undone = false;
            const bool called = QMetaObject::invokeMethod(&store, "undoBundleArchive", Q_RETURN_ARG(bool, undone), Q_ARG(QString, token));
            return called && undone;
        };

        // Offline: cards leave at once and one archive op per unpinned card is queued.
        QVariantMap archived;
        QVERIFY2(archiveNow(archived), "CardStore::archiveBundleNow(QString) is missing");
        QCOMPARE(archived.value("count").toInt(), 2);
        QCOMPARE(store.cardIds(), QStringList({pinned, loose, middle}));
        QCOMPARE(store.pendingOps(), 2);
        QTRY_COMPARE(durableOps(), QStringList({"archive:" + first, "archive:" + last}));
        // Undo: same order as before, ops cancelled in memory and on disk.
        QVERIFY(undo(archived.value("token").toString()));
        QCOMPARE(store.cardIds(), original);
        QCOMPARE(store.pendingOps(), 0);
        QTRY_COMPARE(durableOps(), QStringList());
        // Negative controls: a used or unknown token changes nothing.
        QVERIFY(!undo(archived.value("token").toString()));
        QVERIFY(!undo(QStringLiteral("not-a-token")));
        QCOMPARE(store.cardIds(), original);

        // Online: once the archives were sent, Undo queues the INBOX re-add.
        OpsServer server;
        QVERIFY(server.listen(QHostAddress::LocalHost));
        HttpTransport transport(server.serverPort());
        store.setTransport(&transport);
        store.setOnline(true);
        QVERIFY(archiveNow(archived));
        QTRY_COMPARE(server.received.size(), 2);
        QTRY_COMPARE(store.pendingOps(), 0);
        QVERIFY(undo(archived.value("token").toString()));
        QCOMPARE(store.cardIds(), original);
        QTRY_COMPARE(server.received.size(), 4);
        for (int i = 2; i < 4; ++i) {
            QCOMPARE(server.received[i].value("type").toString(), QStringLiteral("gmail.label_add"));
            QCOMPARE(server.received[i].value("args").toMap().value("label").toString(), QStringLiteral("INBOX"));
        }
        // Pipelined inverses may arrive in either order; both must land.
        QCOMPARE(QSet<QString>({server.received[2].value("card_id").toString(),
            server.received[3].value("card_id").toString()}), QSet<QString>({first, last}));
        QTRY_COMPARE(store.pendingOps(), 0);
        QCOMPARE(store.cardIds(), original);
        QVERIFY(store.applyRemoteCards({{"now", QVariantList{pinnedCard, mail(loose, QVariant()), mail(first, bundle), mail(middle, QVariant()), mail(last, bundle)}}, {"later", QVariantList{}}, {"missed", QVariantList{}}}));
        QCOMPARE(store.cardIds(), original);
    }
    void outboxDrainsInBoundedRounds() {
        QTemporaryDir directory;
        QVERIFY(directory.isValid());
        CardStore store;
        QVERIFY(store.open(directory.filePath("drain.sqlite")));
        QVariantList cards;
        QStringList ids;
        for (int i = 0; i < 100; ++i) {
            const QString id = QUuid::createUuid().toString(QUuid::WithoutBraces);
            ids.append(id);
            cards.append(QVariantMap{{"id", id}, {"title", QStringLiteral("Note %1").arg(i)},
                {"source", "manual"}, {"state", "open"}});
        }
        QVERIFY(store.applyRemoteCards({{"now", cards}, {"later", QVariantList{}}, {"missed", QVariantList{}}}));
        // 50 ms wire delay per POST: a serial pump pays ~5 s of RTT alone.
        LatencyTransport transport(50);
        store.setTransport(&transport);
        QStringList ops;
        for (const QString &id : ids) {
            const QString op = store.saveNote(id, QStringLiteral("edit %1").arg(id));
            QVERIFY(!op.isEmpty());
            ops.append(op);
        }
        QCOMPARE(store.pendingOps(), 100);
        store.setOnline(true);
        QElapsedTimer wall;
        wall.start();
        QTRY_VERIFY2_WITH_TIMEOUT(store.pendingOps() == 0, "outbox did not drain", 30000);
        const qint64 ms = wall.elapsed();
        qInfo("drain 100 ops x 50ms RTT: max %d in flight, %lld ms", transport.maxOutstanding, ms);
        QCOMPARE(transport.posted.size(), 100);
        for (int i = 0; i < 100; ++i)
            QCOMPARE(transport.posted[i].value("op_id").toString(), ops[i]);
        QVERIFY2(transport.maxOutstanding >= 3, qPrintable(QStringLiteral("no pipelining: max %1").arg(transport.maxOutstanding)));
        // 25 flights x 50 ms plus one durable ack commit per op; a serial pump
        // pays the RTT per op (~8.5 s) instead of per flight.
        QVERIFY2(ms < 5000, qPrintable(QStringLiteral("drain took %1 ms, no batching").arg(ms)));
    }
    void drainCardCountScaling() {
        QList<qint64> times;
        for (int count : {200, 5000}) {
            QTemporaryDir directory;
            const QString path = directory.filePath("scale.sqlite");
            QVariantList cards;
            for (int i = 0; i < count; ++i) cards.append(QVariantMap{{"id", QUuid::createUuid().toString(QUuid::WithoutBraces)}, {"source", "manual"}, {"state", "open"}});
            {
                CardStore writer;
                QVERIFY(writer.open(path));
                QVERIFY(writer.applyRemoteCards({{"now", cards}, {"later", QVariantList{}}, {"missed", QVariantList{}}}));
                for (int i = 0; i < 100; ++i) QVERIFY(!writer.enqueueOp(cards[i].toMap().value("id").toString(), "unpin").isEmpty());
            }
            CardStore store;
            QVERIFY(store.open(path));
            QCOMPARE(store.pendingOps(), 100);
            LatencyTransport transport(1);
            store.setTransport(&transport);
            QElapsedTimer timer;
            timer.start();
            store.setOnline(true);
            QTRY_COMPARE_WITH_TIMEOUT(store.pendingOps(), 0, 30000);
            times.append(timer.elapsed());
            QCOMPARE(transport.posted.size(), 100);
        }
        qInfo("DRAIN_SCALING 200=%lld ms 5000=%lld ms", times[0], times[1]);
        QVERIFY2(times[1] <= 2 * times[0] + 150, "5000-card drain exceeds 2x plus scheduling allowance");
    }
    void continuousEventsHaveBoundedRefresh() {
        QTemporaryDir dir;
        CardStore store;
        QVERIFY(store.open(dir.filePath("deadline.sqlite")));
        store.setOnline(true);
        QHash<int, quint64> requests;
        CardEventRefresh coordinator(store, requests);
        QSignalSpy snapshots(&store, &CardStore::requestCards);
        QElapsedTimer elapsed; elapsed.start();
        coordinator.event();
        QTimer events;
        connect(&events, &QTimer::timeout, &store, [&] { coordinator.event(); });
        events.start(100);
        QTRY_VERIFY_WITH_TIMEOUT(!snapshots.isEmpty(), 750);
        QVERIFY2(elapsed.elapsed() < 750, "continuous events starved the refresh");
    }
    void pendingDeadlineSurvivesFailedSnapshot() {
        QTemporaryDir dir;
        CardStore store;
        QVERIFY(store.open(dir.filePath("pending-deadline.sqlite")));
        QVERIFY(store.applyRemoteCards({{"now", QVariantList{QVariantMap{{"id", cardId}, {"source", "manual"}, {"state", "open"}}}}, {"later", QVariantList{}}, {"missed", QVariantList{}}}));
        QVERIFY(!store.enqueueOp(cardId, "unpin").isEmpty());
        store.setOnline(true);
        QHash<int, quint64> requests;
        CardEventRefresh coordinator(store, requests);
        QSignalSpy snapshots(&store, &CardStore::requestCards);
        coordinator.event();
        QTest::qWait(9000);
        requests.insert(1, 1);
        QTest::qWait(2000);
        requests.clear();
        coordinator.snapshotFinished();
        QTRY_COMPARE_WITH_TIMEOUT(snapshots.size(), 1, 750);
    }
    void stalledSnapshotReleasesEventGate() {
        QTemporaryDir dir;
        CardStore store;
        QVERIFY(store.open(dir.filePath("stalled.sqlite")));
        QTcpServer server;
        QVERIFY(server.listen(QHostAddress::LocalHost));
        QList<QTcpSocket *> sockets;
        connect(&server, &QTcpServer::newConnection, &server, [&] {
            while (server.hasPendingConnections()) {
                auto *socket = server.nextPendingConnection();
                sockets.append(socket);
                if (sockets.size() > 1) connect(socket, &QTcpSocket::readyRead, socket, [socket] {
                    socket->readAll();
                    const QByteArray body = "{\"now\":[],\"later\":[],\"missed\":[]}";
                    socket->write("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: " + QByteArray::number(body.size()) + "\r\nConnection: close\r\n\r\n" + body);
                    socket->disconnectFromHost();
                });
            }
        });
        Api api;
        api.setBaseUrl(QStringLiteral("http://127.0.0.1:%1").arg(server.serverPort()));
        QHash<int, quint64> requests;
        CardEventRefresh coordinator(store, requests);
        connect(&store, &CardStore::requestCards, &store, [&](const QString &path, quint64 seq) { requests.insert(api.get(path), seq); });
        connect(&api, &Api::requestFinished, &store, [&](int id, const QJsonValue &body, int) {
            if (!requests.contains(id)) return;
            if (store.applyRemoteCards(body.toObject().toVariantMap(), requests.take(id))) store.setOnline(true);
            coordinator.snapshotFinished();
        });
        connect(&api, &Api::requestFailed, &store, [&](int id, int status, const QString &error, const QJsonValue &) {
            if (requests.remove(id)) coordinator.snapshotFinished();
            if (status == 0) store.reportOffline(error);
        });
        QSignalSpy failed(&api, &Api::requestFailed);
        QSignalSpy snapshots(&store, &CardStore::requestCards);
        store.setOnline(true);
        QTRY_COMPARE(sockets.size(), 1);
        store.refresh(); // A newer snapshot succeeds while the first is stalled.
        QTRY_COMPARE_WITH_TIMEOUT(snapshots.size(), 2, 1000);
        QTRY_COMPARE_WITH_TIMEOUT(requests.size(), 1, 1000);
        coordinator.event();
        QTRY_VERIFY_WITH_TIMEOUT(!failed.isEmpty(), 35000);
        QVERIFY(requests.isEmpty());
        store.setOnline(true);
        QTRY_VERIFY_WITH_TIMEOUT(snapshots.size() >= 3, 1000);
        QTRY_VERIFY_WITH_TIMEOUT(requests.isEmpty(), 1000);
        const int beforeEvent = snapshots.size();
        coordinator.event();
        QTRY_COMPARE_WITH_TIMEOUT(snapshots.size(), beforeEvent + 1, 1000);
    }
    void streamEventsCoalesceDuringDrain() {
        QTemporaryDir directory;
        CardStore store;
        QVERIFY(store.open(directory.filePath("stream.sqlite")));
        QVariantList cards;
        for (int i = 0; i < 130; ++i) cards.append(QVariantMap{{"id", QUuid::createUuid().toString(QUuid::WithoutBraces)}, {"source", "manual"}, {"state", "open"}});
        QVERIFY(store.applyRemoteCards({{"now", cards}, {"later", QVariantList{}}, {"missed", QVariantList{}}}));
        for (const auto &card : cards) QVERIFY(!store.enqueueOp(card.toMap().value("id").toString(), "unpin").isEmpty());
        LatencyTransport transport(10);
        store.setTransport(&transport);
        QHash<int, quint64> requests;
        int fetches = 0;
        CardEventRefresh coordinator(store, requests);
        connect(&store, &CardStore::requestCards, &store, [&](const QString &, quint64 seq) { requests.insert(++fetches, seq); });
        store.setOnline(true);
        QCOMPARE(fetches, 1);
        for (int i = 0; i < 130; ++i) coordinator.event();
        QTRY_COMPARE_WITH_TIMEOUT(store.pendingOps(), 0, 30000);
        QCOMPARE(fetches, 1);
        requests.clear();
        coordinator.snapshotFinished();
        QTRY_COMPARE_WITH_TIMEOUT(fetches, 2, 5000);
        for (int i = 0; i < 130; ++i) coordinator.event();
        QCOMPARE(fetches, 2);
        requests.clear();
        coordinator.snapshotFinished();
        QTRY_COMPARE_WITH_TIMEOUT(fetches, 3, 5000);
    }
    void drainBench_data() {
        QTest::addColumn<int>("ops");
        QTest::addColumn<int>("rttMs");
        QTest::addColumn<int>("inFlight");
        for (int rtt : {1, 150})
            for (int inFlight : {32})
                for (int ops : {100, 200}) {
                    if (rtt == 150 && ops == 200) continue;
                    QTest::addRow("%dops_%dms_x%d", ops, rtt, inFlight) << ops << rtt << inFlight;
                }
    }
    void drainBench() {
        QFETCH(int, ops);
        QFETCH(int, rttMs);
        QFETCH(int, inFlight);
        QTemporaryDir directory;
        QVERIFY(directory.isValid());
        const QString path = directory.filePath("bench.sqlite");
        QVariantList cards;
        QStringList ids;
        for (int i = 0; i < ops; ++i) {
            const QString id = QUuid::createUuid().toString(QUuid::WithoutBraces);
            ids.append(id);
            cards.append(QVariantMap{{"id", id}, {"title", QStringLiteral("Note %1").arg(i)},
                {"source", "manual"}, {"state", "open"}});
        }
        // Queue the ops, close, and reopen: a drain starts from a fully durable
        // outbox, as after an offline period, not behind the enqueue commits.
        {
            CardStore writer;
            QVERIFY(writer.open(path));
            QVERIFY(writer.applyRemoteCards({{"now", cards}, {"later", QVariantList{}}, {"missed", QVariantList{}}}));
            for (const QString &id : ids) QVERIFY(!writer.saveNote(id, QStringLiteral("edit %1").arg(id)).isEmpty());
        }
        CardStore store;
        QVERIFY(store.open(path));
        store.setMaxInFlight(inFlight);
        LatencyTransport transport(rttMs);
        store.setTransport(&transport);
        QCOMPARE(store.pendingOps(), ops);
        store.setOnline(true);
        QElapsedTimer wall;
        wall.start();
        QTRY_VERIFY2_WITH_TIMEOUT(store.pendingOps() == 0, "outbox did not drain", 60000);
        const qint64 ms = wall.elapsed();
        qInfo("BENCH ops=%d rtt=%dms inflight_cap=%d -> max_in_flight=%d wall=%lld ms",
              ops, rttMs, inFlight, transport.maxOutstanding, ms);
        QCOMPARE(transport.posted.size(), ops);
        // Owner target: a 100-op queue drains in well under half a second at LAN RTT.
        if (ops == 100 && rttMs == 1 && inFlight == 32)
            QVERIFY2(ms < 500, qPrintable(QStringLiteral("100 ops took %1 ms").arg(ms)));
    }
    void serverDeletedCardLeavesNoGhostRow() {
        QTemporaryDir directory;
        QVERIFY(directory.isValid());
        const QString path = directory.filePath("ghost.sqlite");
        const QString ghost = QStringLiteral("77777777-7777-4777-8777-777777777777");
        const QString survivor = QStringLiteral("88888888-8888-4888-8888-888888888888");
        const auto sections = [&](bool withGhost) {
            QVariantList now{QVariantMap{{"id", survivor}, {"source", "journal"},
                {"state", "open"}, {"title", "DrawerNote02"}}};
            if (withGhost) now.prepend(QVariantMap{{"id", ghost}, {"source", "journal"},
                {"state", "open"}, {"title", "DrawerNote01"}});
            return QVariantMap{{"now", now}, {"later", QVariantList{}}, {"missed", QVariantList{}}};
        };
        CardStore store;
        QVERIFY(store.open(path));
        QVERIFY(store.applyRemoteCards(sections(true)));
        QCOMPARE(store.rowCount(), 2);
        const QString ghostEdit = store.saveNote(ghost, "stale edit");
        QVERIFY(!ghostEdit.isEmpty());
        const QString survivorEdit = store.saveNote(survivor, "owner edit");
        QVERIFY(!survivorEdit.isEmpty());
        // The server confirms the first card is gone (404) while the second
        // edit is only generically rejected (422 retained for the owner,
        // including across the refresh-triggered retry of the survivor).
        ScriptedTransport transport({404, 422, 422});
        QSignalSpy failed(&store, &CardStore::operationFailed);
        store.setTransport(&transport);
        store.setOnline(true);
        QTRY_COMPARE(store.pendingOps(), 1);
        QCOMPARE(store.cardIds(), QStringList{survivor});
        QVERIFY(store.saveFailed());
        QCOMPARE(failed.size(), 0); // Background sync never reports Save as undone.
        QTest::qWait(500); // Let the async removal commit before reopening.
        CardStore reopened;
        QVERIFY(reopened.open(path));
        QCOMPARE(reopened.pendingOps(), 1);
        QCOMPARE(reopened.cardIds(), QStringList{survivor});
    }
};
QTEST_MAIN(CardStoreTest)
#include "tst_cardstore.moc"
