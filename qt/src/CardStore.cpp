// SPDX-License-Identifier: MIT
#include "CardStore.h"
#include <algorithm>
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
    while (q.next()) {
        QVariantMap card = QJsonDocument::fromJson(q.value(0).toByteArray()).object().toVariantMap();
        const QVariant rank = card.value(QStringLiteral("pinned_rank"));
        const bool pinned = card.contains(QStringLiteral("pinned_rank")) && rank.isValid() && !rank.isNull();
        if (pinned && card.value(QStringLiteral("section")).toString() != QStringLiteral("pinned")) {
            card.insert(QStringLiteral("section_before_pin"), card.value(QStringLiteral("section")));
            card.insert(QStringLiteral("section"), QStringLiteral("pinned"));
        }
        cards.append(std::move(card));
    }
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
            ids.insert(id);
            card.insert(QStringLiteral("section"), section);
            const QVariant rank = card.value(QStringLiteral("pinned_rank"));
            if (card.contains(QStringLiteral("pinned_rank")) && rank.isValid() && !rank.isNull()) {
                card.insert(QStringLiteral("section_before_pin"), section);
                card.insert(QStringLiteral("section"), QStringLiteral("pinned"));
            }
            cards.append(card);
        }
    }
    std::stable_sort(cards.begin(), cards.end(), [](const QVariantMap &left, const QVariantMap &right) {
        const QVariant leftRank = left.value(QStringLiteral("pinned_rank"));
        const QVariant rightRank = right.value(QStringLiteral("pinned_rank"));
        const bool leftPinned = left.contains(QStringLiteral("pinned_rank")) && leftRank.isValid() && !leftRank.isNull();
        const bool rightPinned = right.contains(QStringLiteral("pinned_rank")) && rightRank.isValid() && !rightRank.isNull();
        if (leftPinned != rightPinned) return leftPinned;
        return leftPinned && leftRank.toLongLong() < rightRank.toLongLong();
    });
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
QStringList CardStore::pinnedCardIds() const {
    QList<QPair<qint64, QString>> ordered;
    for (const QVariantMap &card : m_cards) {
        const QVariant rank = card.value(QStringLiteral("pinned_rank"));
        if (card.contains(QStringLiteral("pinned_rank")) && rank.isValid() && !rank.isNull())
            ordered.append({rank.toLongLong(), card.value(QStringLiteral("id")).toString()});
    }
    std::stable_sort(ordered.begin(), ordered.end(), [](const auto &left, const auto &right) {
        return left.first < right.first;
    });
    QStringList ids;
    ids.reserve(ordered.size());
    for (const auto &entry : ordered) ids.append(entry.second);
    return ids;
}
QStringList CardStore::cardIds() const {
    QStringList ids;
    ids.reserve(m_cards.size());
    for (const QVariantMap &card : m_cards) ids.append(card.value(QStringLiteral("id")).toString());
    return ids;
}
bool CardStore::moveCard(const QString &cardId, int delta) {
    QStringList ids = cardIds();
    const int source = ids.indexOf(cardId), target = source + delta;
    if (source < 0 || target < 0 || target >= ids.size()) return false;
    ids.move(source, target);
    return !reorderCards(ids).isEmpty();
}
// Absolute-position variant used by the drag handle. A pinned card moves only
// inside the pinned block (R21, reorder_pins); a card whose position the
// server anchors to its time (R30) is refused instead of being silently
// snapped back after the next sync.
bool CardStore::moveCardTo(const QString &cardId, int targetIndex) {
    const QStringList ids = cardIds();
    const int from = ids.indexOf(cardId);
    if (from < 0 || targetIndex < 0 || targetIndex >= ids.size()) return false;
    const QVariantMap card = m_cards.at(from);
    const QVariant rank = card.value(QStringLiteral("pinned_rank"));
    const bool pinned = card.contains(QStringLiteral("pinned_rank")) && rank.isValid() && !rank.isNull();
    if (pinned) {
        QStringList pins = pinnedCardIds();
        const int pinFrom = pins.indexOf(cardId);
        if (pinFrom < 0) return false;
        const int pinTarget = qBound(0, targetIndex, pins.size() - 1);
        if (pinTarget == pinFrom) return true;
        pins.move(pinFrom, pinTarget);
        QVariantList ordered;
        for (const QString &id : pins) ordered.append(id);
        return !enqueueOp(pins.first(), QStringLiteral("reorder_pins"), {{QStringLiteral("cards"), ordered}}).isEmpty();
    }
    const QVariant at = card.value(QStringLiteral("at"));
    const bool timed = card.value(QStringLiteral("timed")).toBool() ||
        (card.contains(QStringLiteral("at")) && at.isValid() && !at.isNull() && !at.toString().isEmpty());
    if (timed) return false;
    const QString section = card.value(QStringLiteral("section")).toString();
    int first = from, last = from;
    for (int i = 0; i < m_cards.size(); ++i) {
        if (m_cards[i].value(QStringLiteral("section")).toString() != section) continue;
        first = qMin(first, i);
        last = qMax(last, i);
    }
    const int target = qBound(first, targetIndex, last);
    if (target == from) return true;
    QStringList reordered = ids;
    reordered.move(from, target);
    return !reorderCards(reordered).isEmpty();
}
QString CardStore::reorderCards(const QStringList &ids) {
    const QStringList current = cardIds();
    if (ids.size() != current.size() || QSet<QString>(ids.begin(), ids.end()) != QSet<QString>(current.begin(), current.end()) || ids.isEmpty()) return {};
    QVariantList ordered;
    for (const QString &cardId : ids) ordered.append(cardId);
    return enqueueOp(ids.first(), QStringLiteral("reorder_cards"), {{QStringLiteral("cards"), ordered}});
}
bool CardStore::createCard(const QString &title, const QString &summary) {
    const QString cleanTitle = title.trimmed();
    if (cleanTitle.isEmpty()) return false;
    return !enqueueOp(QUuid::createUuid().toString(QUuid::WithoutBraces), QStringLiteral("create_card"),
        {{QStringLiteral("title"), cleanTitle}, {QStringLiteral("summary"), summary.trimmed()}}).isEmpty();
}
QString CardStore::enqueueOp(const QString &cardId, const QString &type, const QVariantMap &args) {
    if (!m_db.isOpen() || QUuid(cardId).isNull()) return {};
    static const QSet<QString> types = {QStringLiteral("done"), QStringLiteral("archive"), QStringLiteral("note"), QStringLiteral("pin"), QStringLiteral("unpin"),
        QStringLiteral("reorder_pins"), QStringLiteral("reorder_cards"), QStringLiteral("create_card"), QStringLiteral("snooze"), QStringLiteral("bundle_archive"), QStringLiteral("bundle_done"), QStringLiteral("take_out")};
    if (!types.contains(type)) return {};
    const QString id = QUuid::createUuid().toString(QUuid::WithoutBraces);
    const QJsonObject payload{{QStringLiteral("op_id"), id}, {QStringLiteral("card_id"), cardId},
        {QStringLiteral("type"), type}, {QStringLiteral("args"), QJsonObject::fromVariantMap(args)}};

    QList<QVariantMap> cards = m_cards;
    auto isPinned = [](const QVariantMap &card) {
        const QVariant rank = card.value(QStringLiteral("pinned_rank"));
        return card.contains(QStringLiteral("pinned_rank")) && rank.isValid() && !rank.isNull();
    };
    bool changed = false;
    if (type == QStringLiteral("done") || type == QStringLiteral("archive") || type == QStringLiteral("snooze")) {
        const auto oldSize = cards.size();
        cards.erase(std::remove_if(cards.begin(), cards.end(), [&](const QVariantMap &card) {
            return card.value(QStringLiteral("id")).toString() == cardId;
        }), cards.end());
        changed = cards.size() != oldSize;
    } else if (type == QStringLiteral("bundle_archive") || type == QStringLiteral("bundle_done")) {
        const QString bundle = args.value(QStringLiteral("bundle_id")).toString();
        if (!bundle.isEmpty()) {
            const auto oldSize = cards.size();
            cards.erase(std::remove_if(cards.begin(), cards.end(), [&](const QVariantMap &card) {
                return card.value(QStringLiteral("bundle_id")).toString() == bundle &&
                    card.value(QStringLiteral("state")).toString() == QStringLiteral("open") &&
                    card.contains(QStringLiteral("pinned_rank")) && !isPinned(card);
            }), cards.end());
            changed = cards.size() != oldSize;
        }
    } else if (type == QStringLiteral("take_out")) {
        const QString target = args.value(QStringLiteral("card")).toString();
        for (QVariantMap &card : cards) {
            if (card.value(QStringLiteral("id")).toString() == target) {
                card.insert(QStringLiteral("bundle_id"), QVariant());
                changed = true;
            }
        }
    } else if (type == QStringLiteral("pin") || type == QStringLiteral("unpin")) {
        qint64 maxRank = 0;
        for (const QVariantMap &card : cards)
            if (isPinned(card)) maxRank = qMax(maxRank, card.value(QStringLiteral("pinned_rank")).toLongLong());
        for (QVariantMap &card : cards) {
            if (card.value(QStringLiteral("id")).toString() != cardId) continue;
            if (type == QStringLiteral("pin")) {
                if (!isPinned(card) && card.value(QStringLiteral("section")).toString() != QStringLiteral("pinned"))
                    card.insert(QStringLiteral("section_before_pin"), card.value(QStringLiteral("section")));
                card.insert(QStringLiteral("section"), QStringLiteral("pinned"));
                card.insert(QStringLiteral("pinned_rank"), QVariant::fromValue(maxRank + 1));
            } else {
                card.insert(QStringLiteral("pinned_rank"), QVariant());
                if (card.value(QStringLiteral("section")).toString() == QStringLiteral("pinned")) {
                    if (card.contains(QStringLiteral("section_before_pin")))
                        card.insert(QStringLiteral("section"), card.value(QStringLiteral("section_before_pin")));
                    card.remove(QStringLiteral("section_before_pin"));
                }
            }
        }
    } else if (type == QStringLiteral("reorder_pins")) {
        const QVariantList ordered = args.value(QStringLiteral("cards")).toList();
        for (int i = 0; i < ordered.size(); ++i) {
            for (QVariantMap &card : cards) {
                if (card.value(QStringLiteral("id")).toString() == ordered[i].toString() && isPinned(card)) {
                    card.insert(QStringLiteral("pinned_rank"), i + 1);
                    changed = true;
                }
            }
        }
    } else if (type == QStringLiteral("reorder_cards")) {
        QStringList ordered;
        for (const QVariant &value : args.value(QStringLiteral("cards")).toList()) ordered.append(value.toString());
        const QStringList current = cardIds();
        if (ordered.size() != cards.size() || QSet<QString>(ordered.begin(), ordered.end()) != QSet<QString>(current.begin(), current.end())) return {};
        QList<QVariantMap> reordered;
        for (int i = 0; i < ordered.size(); ++i) {
            for (QVariantMap &card : cards) if (card.value(QStringLiteral("id")).toString() == ordered[i]) {
                card.insert(QStringLiteral("note_order"), i);
                reordered.append(card);
            }
        }
        cards = std::move(reordered);
        changed = true;
    } else if (type == QStringLiteral("create_card")) {
        QVariantMap card{{QStringLiteral("id"), cardId}, {QStringLiteral("source"), QStringLiteral("manual")},
            {QStringLiteral("title"), args.value(QStringLiteral("title"))}, {QStringLiteral("summary"), args.value(QStringLiteral("summary"))},
            {QStringLiteral("state"), QStringLiteral("open")}, {QStringLiteral("section"), QStringLiteral("now")}};
        if (card.value(QStringLiteral("title")).toString().trimmed().isEmpty()) return {};
        cards.prepend(card);
        changed = true;
    }
    if (changed) {
        std::stable_sort(cards.begin(), cards.end(), [&](const QVariantMap &left, const QVariantMap &right) {
            const bool leftPinned = isPinned(left), rightPinned = isPinned(right);
            if (leftPinned != rightPinned) return leftPinned;
            if (leftPinned) return left.value(QStringLiteral("pinned_rank")).toLongLong() < right.value(QStringLiteral("pinned_rank")).toLongLong();
            return false;
        });
    }

    if (!m_db.transaction()) return {};
    QSqlQuery q(m_db);
    q.prepare(QStringLiteral("INSERT INTO outbox(op_id,payload) VALUES(?,?)"));
    q.addBindValue(id); q.addBindValue(QString::fromUtf8(QJsonDocument(payload).toJson(QJsonDocument::Compact)));
    if (!q.exec()) { m_db.rollback(); emit storageError(q.lastError().text()); return {}; }
    if (changed) {
        if (!q.exec(QStringLiteral("DELETE FROM cards")) ||
            !q.prepare(QStringLiteral("INSERT INTO cards(id,position,payload) VALUES(?,?,?)"))) {
            m_db.rollback(); emit storageError(q.lastError().text()); return {};
        }
        for (int i = 0; i < cards.size(); ++i) {
            q.bindValue(0, cards[i].value(QStringLiteral("id")));
            q.bindValue(1, i);
            q.bindValue(2, QString::fromUtf8(QJsonDocument(QJsonObject::fromVariantMap(cards[i])).toJson(QJsonDocument::Compact)));
            if (!q.exec()) { m_db.rollback(); emit storageError(q.lastError().text()); return {}; }
        }
    }
    if (!m_db.commit()) { emit storageError(m_db.lastError().text()); m_db.rollback(); return {}; }
    if (changed) reloadCache();
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
