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
            QSet<QString> ids;
            for (const QVariantList &args : prefetch) ids.insert(args.first().toString());
            QVERIFY(ids.contains(agentId));
            QVERIFY(ids.contains(mailId));
            QVERIFY(!ids.contains(manualId));
            // Plain cards without has_body have no fetchable body.
            store.requestMailBody(manualId);
            QCOMPARE(prefetch.size(), 2);
            store.requestMailBody(agentId);
            QCOMPARE(prefetch.size(), 3);
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
    }
};
QTEST_MAIN(CardStoreTest)
#include "tst_cardstore.moc"
