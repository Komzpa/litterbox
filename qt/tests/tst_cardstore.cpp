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
                    int status = reject;
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
class CardStoreTest : public QObject {
    Q_OBJECT
private slots:
    void cachedCardsSurviveRestart() {
        QTemporaryDir directory;
        QVERIFY(directory.isValid());
        const QString path = directory.filePath("cache.sqlite");
        const QVariantMap card{{"id",cardId},{"title","Offline card"},{"note","Durable note"}};
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
        QCOMPARE(restored.data(restored.index(0),CardStore::TitleRole).toString(), QStringLiteral("Offline card"));
        // A malformed response cannot destroy the offline snapshot.
        QVERIFY(!restored.applyRemoteCards({{"now",QVariantList{}}}));
        QCOMPARE(restored.rowCount(),1);
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
        QString id;
        {
            CardStore store;
            QVERIFY(store.open(path));
            store.setTransport(&transport);
            id = store.dismiss(cardId);
            QSignalSpy failed(&store,&CardStore::operationFailed);
            store.setOnline(true);
            QTRY_COMPARE(failed.size(),1);
            QCOMPARE(store.pendingOps(),1);
            QCOMPARE(server.applied,1);
        }
        CardStore restored;
        QVERIFY(restored.open(path));
        restored.setTransport(&transport);
        restored.setOnline(true);
        QTRY_COMPARE(restored.pendingOps(),0);
        QCOMPARE(server.received.size(),2);
        QCOMPARE(server.received[0],server.received[1]);
        QCOMPARE(server.received[1].value("op_id").toString(),id);
        QCOMPARE(server.applied,1);
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
    void rejectionRetainsHeadAndBlocksLaterOps() {
        QTemporaryDir directory;
        OpsServer server;
        QVERIFY(server.listen(QHostAddress::LocalHost));
        server.reject = 422;
        HttpTransport transport(server.serverPort());
        CardStore store;
        QVERIFY(store.open(directory.filePath("cache.sqlite")));
        const QString first = store.enqueueOp(cardId,"pin");
        store.enqueueOp(cardId,"unpin");
        store.setTransport(&transport);
        QSignalSpy failed(&store,&CardStore::operationFailed);
        store.setOnline(true);
        QTRY_COMPARE(failed.size(),1);
        QCOMPARE(store.pendingOps(),2);
        QCOMPARE(server.received.size(),1);
        QCOMPARE(server.received[0].value("op_id").toString(),first);
        server.reject = 0;
        store.setOnline(false); store.setOnline(true);
        QTRY_COMPARE(store.pendingOps(),0);
        QCOMPARE(server.received[1].value("op_id").toString(),first);
        QCOMPARE(server.received[2].value("type").toString(),QStringLiteral("unpin"));
    }
};
QTEST_MAIN(CardStoreTest)
#include "tst_cardstore.moc"
