// SPDX-License-Identifier: MIT
#include "CardStore.h"
#include <QJsonDocument>
#include <QJsonObject>
#include <QPointer>
#include <QSet>
#include <QSqlError>
#include <QSqlQuery>
#include <QTimer>
#include <QUuid>
#include <QtQml/qqml.h>

CardStore::CardStore(QObject *parent) : QAbstractListModel(parent),
    m_connection(QUuid::createUuid().toString(QUuid::WithoutBraces)) {}
CardStore::~CardStore() {
    m_db.close();
    m_db = QSqlDatabase();
    QSqlDatabase::removeDatabase(m_connection);
}
void CardStore::registerQml(const char *uri, int major, int minor) {
    qmlRegisterType<CardStore>(uri, major, minor, "CardStore");
}
bool CardStore::execute(const QString &sql) {
    QSqlQuery q(m_db);
    if (q.exec(sql)) return true;
    emit storageError(q.lastError().text());
    return false;
}
bool CardStore::open(const QString &path) {
    if (m_db.isOpen() || !m_inFlight.isEmpty()) return false;
    m_db = QSqlDatabase::addDatabase(QStringLiteral("QSQLITE"), m_connection);
    m_db.setDatabaseName(path);
    if (!m_db.open()) { emit storageError(m_db.lastError().text()); return false; }
    if (!execute(QStringLiteral("PRAGMA synchronous=FULL")) ||
        !execute(QStringLiteral("CREATE TABLE IF NOT EXISTS cards (id TEXT PRIMARY KEY, position INTEGER NOT NULL, payload TEXT NOT NULL)")) ||
        !execute(QStringLiteral("CREATE TABLE IF NOT EXISTS outbox (seq INTEGER PRIMARY KEY AUTOINCREMENT, op_id TEXT UNIQUE NOT NULL, payload TEXT NOT NULL)"))) {
        m_db.close(); return false;
    }
    reloadCache();
    emit pendingOpsChanged();
    flush();
    return true;
}
void CardStore::reloadCache() {
    QList<QVariantMap> cards;
    QSqlQuery q(m_db);
    if (!q.exec(QStringLiteral("SELECT payload FROM cards ORDER BY position"))) {
        emit storageError(q.lastError().text()); return;
    }
    while (q.next()) cards.append(QJsonDocument::fromJson(q.value(0).toByteArray()).object().toVariantMap());
    beginResetModel(); m_cards = std::move(cards); endResetModel();
}
int CardStore::pendingOps() const {
    if (!m_db.isOpen()) return 0;
    QSqlQuery q(m_db);
    if (!q.exec(QStringLiteral("SELECT COUNT(*) FROM outbox")) || !q.next()) return 0;
    return q.value(0).toInt();
}
void CardStore::setOnline(bool online) {
    if (m_online == online) return;
    m_online = online; emit onlineChanged();
    if (online) { flush(); refresh(); }
}
void CardStore::setTransport(cardstore::OpTransport *transport) {
    m_transport = transport;
    flush();
}
void CardStore::refresh() {
    if (m_online && m_db.isOpen()) emit requestCards(QStringLiteral("/v1/cards"));
}
bool CardStore::applyRemoteCards(const QVariantMap &sections) {
    if (!m_db.isOpen()) return false;
    // Validate the complete snapshot before replacing a usable offline cache.
    QList<QVariantMap> cards;
    QSet<QString> ids;
    for (const QString &section : {QStringLiteral("now"), QStringLiteral("later"), QStringLiteral("missed")}) {
        if (!sections.contains(section) || sections.value(section).metaType().id() != QMetaType::QVariantList) return false;
        for (const QVariant &value : sections.value(section).toList()) {
            QVariantMap card = value.toMap();
            const QString id = card.value(QStringLiteral("id")).toString();
            if (id.isEmpty() || ids.contains(id)) return false;
            ids.insert(id); card.insert(QStringLiteral("section"), section); cards.append(card);
        }
    }
    if (!m_db.transaction()) return false;
    QSqlQuery q(m_db);
    bool ok = q.exec(QStringLiteral("DELETE FROM cards"));
    if (ok) ok = q.prepare(QStringLiteral("INSERT INTO cards(id,position,payload) VALUES(?,?,?)"));
    for (int i = 0; ok && i < cards.size(); ++i) {
        q.bindValue(0, cards[i].value(QStringLiteral("id")));
        q.bindValue(1, i);
        q.bindValue(2, QString::fromUtf8(QJsonDocument(QJsonObject::fromVariantMap(cards[i])).toJson(QJsonDocument::Compact)));
        ok = q.exec();
    }
    if (!ok || !m_db.commit()) {
        emit storageError(q.lastError().text()); m_db.rollback(); return false;
    }
    reloadCache(); return true;
}
QString CardStore::enqueueOp(const QString &cardId, const QString &type, const QVariantMap &args) {
    if (!m_db.isOpen() || QUuid(cardId).isNull()) return {};
    static const QSet<QString> types = {QStringLiteral("done"), QStringLiteral("note"), QStringLiteral("pin"), QStringLiteral("unpin"),
        QStringLiteral("reorder_pins"), QStringLiteral("snooze"), QStringLiteral("bundle_archive"), QStringLiteral("bundle_done"), QStringLiteral("take_out")};
    if (!types.contains(type)) return {};
    const QString id = QUuid::createUuid().toString(QUuid::WithoutBraces);
    const QJsonObject payload{{QStringLiteral("op_id"), id}, {QStringLiteral("card_id"), cardId},
        {QStringLiteral("type"), type}, {QStringLiteral("args"), QJsonObject::fromVariantMap(args)}};
    QSqlQuery q(m_db);
    q.prepare(QStringLiteral("INSERT INTO outbox(op_id,payload) VALUES(?,?)"));
    q.addBindValue(id); q.addBindValue(QString::fromUtf8(QJsonDocument(payload).toJson(QJsonDocument::Compact)));
    if (!q.exec()) { emit storageError(q.lastError().text()); return {}; }
    emit pendingOpsChanged(); flush(); return id;
}
void CardStore::flush() {
    if (m_online && m_db.isOpen() && m_inFlight.isEmpty()) flushNext();
}
void CardStore::flushNext() {
    if (!m_online || !m_inFlight.isEmpty()) return;
    QSqlQuery q(m_db);
    if (!q.exec(QStringLiteral("SELECT op_id,payload FROM outbox ORDER BY seq LIMIT 1"))) {
        emit storageError(q.lastError().text()); return;
    }
    if (!q.next()) { emit flushFinished(); return; }
    m_inFlight = q.value(0).toString();
    const QString id = m_inFlight;
    const QVariantMap payload = QJsonDocument::fromJson(q.value(1).toByteArray()).object().toVariantMap();
    if (m_transport) {
        QPointer<CardStore> self(this);
        m_transport->postOp(QStringLiteral("/v1/ops"), payload, [self,id](int status, const QVariantMap &response) {
            if (self) self->reportPostResult(id, status, response);
        });
    } else {
        emit requestPost(id, QStringLiteral("/v1/ops"), payload);
    }
}
void CardStore::reportPostResult(const QString &opId, int status, const QVariantMap &response) {
    if (opId != m_inFlight || m_inFlight.isEmpty()) return;
    m_inFlight.clear();
    // A transport success alone is not a server acknowledgement.
    if (status < 200 || status >= 300 || response.value(QStringLiteral("ok")) != QVariant(true)) {
        emit operationFailed(opId, status); return;
    }
    QSqlQuery q(m_db);
    q.prepare(QStringLiteral("DELETE FROM outbox WHERE op_id=?")); q.addBindValue(opId);
    if (!q.exec()) { emit storageError(q.lastError().text()); return; }
    emit pendingOpsChanged();
    // Avoid recursion if an injected transport acknowledges synchronously.
    QTimer::singleShot(0, this, [this] { flush(); });
    refresh();
}
int CardStore::rowCount(const QModelIndex &parent) const { return parent.isValid() ? 0 : m_cards.size(); }
QVariant CardStore::data(const QModelIndex &index, int role) const {
    if (!index.isValid() || index.row() < 0 || index.row() >= m_cards.size()) return {};
    const QVariantMap &card = m_cards[index.row()];
    switch (role) {
    case CardRole: return card;
    case CardIdRole: return card.value(QStringLiteral("id"));
    case TitleRole: return card.value(QStringLiteral("title"));
    case SectionRole: return card.value(QStringLiteral("section"));
    default: return {};
    }
}
QHash<int,QByteArray> CardStore::roleNames() const {
    return {{CardRole,"card"},{CardIdRole,"cardId"},{TitleRole,"title"},{SectionRole,"section"}};
}
