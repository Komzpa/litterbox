// SPDX-License-Identifier: MIT
#include "CardStore.h"
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

static const QString cardId = QStringLiteral("11111111-1111-4111-8111-111111111111");

// HTTP oracle for the /v1/ops contract. Records op_id/payload pairs and
// deliberately loses one ACK after applying an op. This proves the client's
// replay protocol, not the production server's PostgreSQL implementation.
class OpsServer : public QTcpServer {
public:
    QList<QVariantMap> received;
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
                    const QString id = op.value("op_id").toString();
                    int status = statusSequence.isEmpty() ? reject : statusSequence.takeFirst();
                    if (!status) {
                        if (ledger.contains(id) && ledger[id] != body) status = 409;
                        else if (!ledger.contains(id)) { ledger[id] = body; ++applied; }
                    }
                    if (loseNextAck) { loseNextAck = false; socket->disconnectFromHost(); return; }
                    if (!status) status = 200;
                    const QByteArray response = status == 200 ? QByteArray("{\"ok\":true}") : QByteArray("{\"ok\":false}");
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
class CardStoreTest : public QObject {
    Q_OBJECT
private slots:
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
            QVERIFY(QMetaObject::invokeMethod(snoozeActions, "chooseSnoozeDateTime"));
            QObject *dialog = snoozeActions->findChild<QObject *>(QStringLiteral("snoozeDialog"));
            QObject *field = snoozeActions->findChild<QObject *>(QStringLiteral("snoozeDateTime"));
            QVERIFY(dialog);
            QVERIFY(field);
            field->setProperty("text", QStringLiteral("2030-03-04 05:06"));
            QVERIFY(QMetaObject::invokeMethod(dialog, "accept"));
            QCOMPARE(offline.pendingOps(), 1);
            QCOMPARE(offline.rowCount(), 2);

            QVERIFY(QMetaObject::invokeMethod(snoozeActions, "chooseSnoozeDateTime"));
            field->setProperty("text", QStringLiteral("2030-02-30 12:00"));
            QVERIFY(QMetaObject::invokeMethod(dialog, "accept"));
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
    void bodyPrefetchPacesLargeSnapshotsAndForegroundBypassesQueue() {
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
        QElapsedTimer elapsed;
        elapsed.start();
        QVERIFY(store.applyRemoteCards(sections));
        QCOMPARE(requested.size(), 1);
        QCOMPARE(requested.first().at(0).toString(), ids.first());
        // Repeated snapshots and a foreground call for an in-flight body dedup.
        for (int i = 0; i < 3; ++i) QVERIFY(store.applyRemoteCards(sections));
        store.requestMailBody(ids.first());
        QCOMPARE(requested.size(), 1);
        store.requestMailBody(ids.last());
        QCOMPARE(requested.size(), 2);
        QCOMPARE(requested.last().at(0).toString(), ids.last());
        store.requestMailBody(ids.last());
        QCOMPARE(requested.size(), 2);
        QVERIFY(store.applyRemoteCards(sections));
        QTest::qWait(100);
        QCOMPARE(requested.size(), 2);
        // No response to the first request: later bodies still progress.
        QTRY_COMPARE_WITH_TIMEOUT(requested.size(), 3, 5000);
        QVERIFY(elapsed.elapsed() >= 3000);
        QCOMPARE(requested.last().at(0).toString(), ids.at(1));
        QVERIFY(store.applyRemoteCards(sections));
        QCOMPARE(requested.size(), 3);
        QCOMPARE(store.pendingOps(), 0); // Reading bodies cannot archive anything.
        QCOMPARE(store.rowCount(), 200);
    }
    void bodyPrefetchDeduplicatesQueuedCardsAndStopsOffline() {
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
        QCOMPARE(requested.size(), 1);
        for (int i = 0; i < 3; ++i) QVERIFY(store.applyRemoteCards(sections));
        // Foreground removes second from the queue, rather than fetching it twice.
        store.requestMailBody(second);
        QCOMPARE(requested.size(), 2);
        QTRY_COMPARE_WITH_TIMEOUT(requested.size(), 3, 5000);
        QCOMPARE(requested.last().at(0).toString(), third);
        QVERIFY(store.applyRemoteCards(sections));
        QTest::qWait(3100);
        QCOMPARE(requested.size(), 3);
        store.setOnline(false);
        store.reportMailBodyFailed(cardId);
        QVERIFY(store.applyRemoteCards(sections));
        QTest::qWait(3100);
        QCOMPARE(requested.size(), 3);
        // Reconnection rebuilds the queue; failed/abandoned requests can retry.
        store.setOnline(true);
        QCOMPARE(requested.size(), 4);
        QCOMPARE(requested.last().at(0).toString(), cardId);
        store.setOnline(false);
        store.setOnline(true);
        QCOMPARE(requested.size(), 4); // A reconnect cannot bypass the 3000ms pace.
        store.requestMailBody(third);
        QCOMPARE(requested.size(), 5); // Offline cleared the abandoned in-flight set.
        QCOMPARE(requested.last().at(0).toString(), third);
        store.setOnline(false);
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
        store.requestMailBody(manual);
        store.requestMailBody("missing");
        store.requestMailBody(cardId);
        QCOMPARE(requested.size(), 1);
        store.applyRemoteMailBody(cardId, {{"html", "   "}});
        QCOMPARE(failed.size(), 1);
        QVERIFY(store.cachedMailBody(cardId).isEmpty());
        store.requestMailBody(cardId);
        QCOMPARE(requested.size(), 2);
        store.applyRemoteMailBody(cardId, {{"html", "<p>Useful cached mail</p>"}});
        QTRY_COMPARE(changed.size(), 1);
        const QVariantMap useful = store.cachedMailBody(cardId);
        store.applyRemoteMailBody(cardId, {{"error", "rate limit exceeded"}});
        store.reportMailBodyFailed(cardId);
        QCOMPARE(failed.size(), 3);
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
        store.flush();
        QTest::qWait(1100);
        QCOMPARE(posted.size(), 1);
        QCOMPARE(failed.size(), 0);
        // Exercise the real existing timer: 429 must wait a complete minute.
        QTRY_COMPARE_WITH_TIMEOUT(posted.size(), 2, 65000);
        QVERIFY(retryElapsed.elapsed() >= 60000);
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
            QCOMPARE(prefetch.size(), 1);
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
            QSignalSpy posted(&store, &CardStore::requestPost);
            store.setOnline(true);
            const QString archive = store.enqueueOp(cardId, "archive");
            QTRY_COMPARE(posted.size(), 1);
            store.reportPostResult(archive, 200, {{"ok", true}});
            QTRY_COMPARE(store.pendingOps(), 0);
            // This archived snapshot was requested before Undo and returns last.
            store.refresh();
            QVERIFY(store.undoBundleArchive(archive));
            QTRY_COMPARE(posted.size(), 2);
            store.reportPostResult(posted.last().at(0).toString(), 200, {{"ok", true}});
            QTRY_COMPARE(store.pendingOps(), 0);
            QVERIFY(store.applyRemoteCards(absent));
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
        QTRY_COMPARE(posted.size(), 1);
        store.reportPostResult(posted.last().at(0).toString(), 422, {{"ok", false}});
        QTRY_COMPARE(posted.size(), 2);
        // Keep C queued when B fails, rather than transmitting another archive.
        store.setOnline(false);
        store.reportPostResult(posted.last().at(0).toString(), 422, {{"ok", false}});
        QTRY_COMPARE(store.pendingOps(), 1);
        QVERIFY(store.undoBundleArchive(token));
        QCOMPARE(store.cardIds(), QStringList({"P", "A", "B", "C"}));
        QCOMPARE(store.pendingOps(), 0);
        QCOMPARE(posted.size(), 2);
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
        QCOMPARE(server.received[2].value("card_id").toString(), first);
        QCOMPARE(server.received[3].value("card_id").toString(), last);
        QTRY_COMPARE(store.pendingOps(), 0);
        QCOMPARE(store.cardIds(), original);
        QVERIFY(store.applyRemoteCards({{"now", QVariantList{pinnedCard, mail(loose, QVariant()), mail(first, bundle), mail(middle, QVariant()), mail(last, bundle)}}, {"later", QVariantList{}}, {"missed", QVariantList{}}}));
        QCOMPARE(store.cardIds(), original);
    }
};
QTEST_MAIN(CardStoreTest)
#include "tst_cardstore.moc"
