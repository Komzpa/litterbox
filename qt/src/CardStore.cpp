// SPDX-License-Identifier: MIT
#include "CardStore.h"
#include <algorithm>
#include <QByteArray>
#include <QDir>
#include <QFile>
#include <QHash>
#include <QJsonDocument>
#include <QJsonObject>
#include <QPointer>
#include <QSet>
#include <QSqlError>
#include <QSqlQuery>
#include <QStandardPaths>
#include <QTimer>
#include <QUrl>
#include <QUuid>
#include <QtQml/qqml.h>

CardStore::CardStore(QObject *parent) : QAbstractListModel(parent),
    m_connection(QUuid::createUuid().toString(QUuid::WithoutBraces)),
    m_storage(new QObject) {
    m_storage->moveToThread(&m_storageThread);
    connect(&m_storageThread, &QThread::finished, m_storage, &QObject::deleteLater);
    m_storageThread.start();
}
CardStore::~CardStore() {
    // Drain queued commits before closing: an offline action survives exit.
    QMetaObject::invokeMethod(m_storage, [this] {
        m_db.close();
        m_db = QSqlDatabase();
        QSqlDatabase::removeDatabase(m_connection);
    }, Qt::BlockingQueuedConnection);
    m_storageThread.quit();
    m_storageThread.wait();
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
    if (m_open || !m_inFlight.isEmpty()) return false;
    QList<QVariantMap> cards, outbox;
    QHash<QString, QVariantMap> bodies;
    bool opened = false;
    // Startup only, before the window exists. Every subsequent write is queued.
    QMetaObject::invokeMethod(m_storage, [&, this] {
        m_db = QSqlDatabase::addDatabase(QStringLiteral("QSQLITE"), m_connection);
        m_db.setDatabaseName(path);
        if (!m_db.open()) { emit storageError(m_db.lastError().text()); return; }
        if (!execute(QStringLiteral("PRAGMA synchronous=FULL")) ||
            !execute(QStringLiteral("CREATE TABLE IF NOT EXISTS cards (id TEXT PRIMARY KEY, position INTEGER NOT NULL, payload TEXT NOT NULL)")) ||
            !execute(QStringLiteral("CREATE TABLE IF NOT EXISTS outbox (seq INTEGER PRIMARY KEY AUTOINCREMENT, op_id TEXT UNIQUE NOT NULL, payload TEXT NOT NULL)")) ||
            !execute(QStringLiteral("CREATE TABLE IF NOT EXISTS mail_bodies (card_id TEXT PRIMARY KEY, html TEXT NOT NULL, source_url TEXT NOT NULL)"))) return;
        QSqlQuery columns(m_db);
        if (!columns.exec(QStringLiteral("PRAGMA table_info(cards)"))) return;
        bool hasSenderName = false;
        while (columns.next())
            if (columns.value(1).toString() == QStringLiteral("sender_name")) hasSenderName = true;
        if (!hasSenderName && !execute(QStringLiteral("ALTER TABLE cards ADD COLUMN sender_name TEXT NOT NULL DEFAULT ''"))) return;
        QSqlQuery q(m_db);
        if (!q.exec(QStringLiteral("SELECT payload,sender_name FROM cards ORDER BY position"))) return;
        while (q.next()) {
            QVariantMap card = QJsonDocument::fromJson(q.value(0).toByteArray()).object().toVariantMap();
            card.insert(QStringLiteral("sender_name"), q.value(1).toString());
            const QVariant rank = card.value(QStringLiteral("pinned_rank"));
            if (rank.isValid() && !rank.isNull() && card.value(QStringLiteral("section")).toString() != QStringLiteral("pinned")) {
                card.insert(QStringLiteral("section_before_pin"), card.value(QStringLiteral("section")));
                card.insert(QStringLiteral("section"), QStringLiteral("pinned"));
            }
            cards.append(std::move(card));
        }
        if (!q.exec(QStringLiteral("SELECT payload FROM outbox ORDER BY seq"))) return;
        while (q.next()) outbox.append(QJsonDocument::fromJson(q.value(0).toByteArray()).object().toVariantMap());
        if (!q.exec(QStringLiteral("SELECT card_id,html,source_url FROM mail_bodies"))) return;
        while (q.next()) bodies.insert(q.value(0).toString(), {{QStringLiteral("html"), q.value(1)}, {QStringLiteral("source_url"), q.value(2)}});
        opened = true;
    }, Qt::BlockingQueuedConnection);
    if (!opened) return false;
    m_open = true;
    m_outbox = std::move(outbox);
    for (const auto &op : m_outbox) m_durableOps.insert(op.value(QStringLiteral("op_id")).toString());
    m_mailBodies = std::move(bodies);
    replaceCards(std::move(cards));
    emit pendingOpsChanged();
    flush();
    return true;
}

void CardStore::replaceCards(QList<QVariantMap> cards) {
    deriveBundles(cards);
    QSet<QString> ids;
    for (const auto &card : cards) ids.insert(card.value(QStringLiteral("id")).toString());
    for (int row = m_cards.size() - 1; row >= 0; --row) {
        if (ids.contains(m_cards[row].value(QStringLiteral("id")).toString())) continue;
        beginRemoveRows({}, row, row);
        m_cards.removeAt(row);
        endRemoveRows();
    }
    QStringList order = cardIds();
    for (int row = 0; row < cards.size(); ++row) {
        const QString id = cards[row].value(QStringLiteral("id")).toString();
        if (row >= order.size() || order[row] != id) {
            const int from = order.indexOf(id, row);
            if (from < 0) {
                beginInsertRows({}, row, row);
                m_cards.insert(row, cards[row]);
                order.insert(row, id);
                endInsertRows();
            } else {
                beginMoveRows({}, from, from, {}, row);
                m_cards.move(from, row);
                order.move(from, row);
                endMoveRows();
            }
        }
        if (m_cards[row] != cards[row]) {
            m_cards[row] = cards[row];
            emit dataChanged(index(row), index(row));
        }
    }
}

bool CardStore::writeCards(const QList<QVariantMap> &before, const QList<QVariantMap> &after) {
    // Derived bundle flags are presentation state, not durable card changes.
    const auto stored = [](QVariantMap card) {
        card.remove(QStringLiteral("bundle_leader"));
        card.remove(QStringLiteral("bundle_member_count"));
        if (!card.contains(QStringLiteral("sender_name")))
            card.insert(QStringLiteral("sender_name"), QStringLiteral(""));
        return card;
    };
    QHash<QString, QVariantMap> old;
    QSet<QString> ids;
    QStringList retained, order;
    for (const auto &card : before) old.insert(card.value(QStringLiteral("id")).toString(), stored(card));
    for (const auto &card : after) {
        const QString id = card.value(QStringLiteral("id")).toString();
        ids.insert(id);
        order.append(id);
    }
    QSqlQuery q(m_db);
    if (!q.prepare(QStringLiteral("DELETE FROM cards WHERE id=?"))) return false;
    for (const auto &card : before) {
        const QString id = card.value(QStringLiteral("id")).toString();
        if (ids.contains(id)) { retained.append(id); continue; }
        q.bindValue(0, id);
        if (!q.exec()) return false;
    }
    const bool reordered = retained != order;
    for (int row = 0; row < after.size(); ++row) {
        const QVariantMap card = stored(after[row]);
        const QString id = card.value(QStringLiteral("id")).toString();
        if (!reordered && old.value(id) == card) continue;
        if (!q.prepare(reordered ?
                QStringLiteral("INSERT INTO cards(id,position,payload,sender_name) VALUES(?,?,?,?) ON CONFLICT(id) DO UPDATE SET position=excluded.position,payload=excluded.payload,sender_name=excluded.sender_name") :
                QStringLiteral("UPDATE cards SET position=position,payload=?,sender_name=? WHERE id=?"))) return false;
        const QString payload = QString::fromUtf8(QJsonDocument(QJsonObject::fromVariantMap(card)).toJson(QJsonDocument::Compact));
        if (reordered) { q.addBindValue(id); q.addBindValue(row); q.addBindValue(payload); q.addBindValue(card.value(QStringLiteral("sender_name")).toString()); }
        else { q.addBindValue(payload); q.addBindValue(card.value(QStringLiteral("sender_name")).toString()); q.addBindValue(id); }
        if (!q.exec()) return false;
    }
    return true;
}

void CardStore::persistCards(const QList<QVariantMap> &before, const QList<QVariantMap> &after,
                             const QList<QVariantMap> &ops, const QStringList &cancelledOps) {
    QMetaObject::invokeMethod(m_storage, [this, before, after, ops, cancelledOps] {
        bool ok = m_db.transaction();
        for (const QVariantMap &op : ops) {
            if (!ok) break;
            QSqlQuery q(m_db);
            ok = q.prepare(QStringLiteral("INSERT INTO outbox(op_id,payload) VALUES(?,?)"));
            q.addBindValue(op.value(QStringLiteral("op_id")));
            q.addBindValue(QString::fromUtf8(QJsonDocument(QJsonObject::fromVariantMap(op)).toJson(QJsonDocument::Compact)));
            if (ok) ok = q.exec();
        }
        for (const QString &opId : cancelledOps) {
            if (!ok) break;
            QSqlQuery q(m_db);
            ok = q.prepare(QStringLiteral("DELETE FROM outbox WHERE op_id=?"));
            q.addBindValue(opId);
            if (ok) ok = q.exec();
        }
        if (ok) ok = writeCards(before, after);
        if (ok) ok = m_db.commit();
        if (!ok) m_db.rollback();
        const QString error = m_db.lastError().text();
        QMetaObject::invokeMethod(this, [this, ops, ok, error] {
            if (!ok) {
                // Cancelled ops stay durable after a failed commit; the next
                // open() reloads that outbox as the source of truth.
                for (const QVariantMap &op : ops) {
                    const QString id = op.value(QStringLiteral("op_id")).toString();
                    undoOp(id);
                    m_outbox.erase(std::remove_if(m_outbox.begin(), m_outbox.end(), [&](const auto &entry) { return entry.value(QStringLiteral("op_id")).toString() == id; }), m_outbox.end());
                }
                if (!ops.isEmpty()) emit pendingOpsChanged();
                emit storageError(error);
                return;
            }
            for (const QVariantMap &op : ops) m_durableOps.insert(op.value(QStringLiteral("op_id")).toString());
            flush();
        }, Qt::QueuedConnection);
    }, Qt::QueuedConnection);
}

void CardStore::undoOp(const QString &opId) {
    if (!m_mutations.contains(opId)) return;
    const Mutation mutation = m_mutations.take(opId);
    QList<QVariantMap> cards = m_cards;
    QHash<QString, QVariantMap> before, after;
    for (const auto &card : mutation.before) before.insert(card.value(QStringLiteral("id")).toString(), card);
    for (const auto &card : mutation.after) after.insert(card.value(QStringLiteral("id")).toString(), card);
    for (int row = cards.size() - 1; row >= 0; --row) {
        const QString id = cards[row].value(QStringLiteral("id")).toString();
        if (!before.contains(id) && after.contains(id)) { cards.removeAt(row); continue; }
        if (!before.contains(id) || !after.contains(id)) continue;
        const auto old = before.value(id), optimistic = after.value(id);
        QSet<QString> keys;
        for (auto it = old.begin(); it != old.end(); ++it) keys.insert(it.key());
        for (auto it = optimistic.begin(); it != optimistic.end(); ++it) keys.insert(it.key());
        for (const auto &key : keys) {
            if (old.value(key) == optimistic.value(key) || cards[row].value(key) != optimistic.value(key)) continue;
            if (old.contains(key)) cards[row].insert(key, old.value(key));
            else cards[row].remove(key);
        }
    }
    for (int row = 0; row < mutation.before.size(); ++row) {
        const auto &card = mutation.before[row];
        const QString id = card.value(QStringLiteral("id")).toString();
        if (!after.contains(id)) cards.insert(qMin(row, int(cards.size())), card);
    }
    const auto current = m_cards;
    replaceCards(cards);
    persistCards(current, cards);
}
// Derive bundle_leader and bundle_member_count from the complete current open
// snapshot. Pinned and important cards are shown standalone: they are neither
// the leader nor counted members, so they are never concealed by a collapsed
// bundle. The first non-exempt member (in list order) is the leader; every
// other non-exempt member is concealed until expanded.
void CardStore::deriveBundles(QList<QVariantMap> &cards) {
    const auto isPinned = [](const QVariantMap &card) {
        const QVariant rank = card.value(QStringLiteral("pinned_rank"));
        return card.contains(QStringLiteral("pinned_rank")) && rank.isValid() && !rank.isNull();
    };
    const auto isExempt = [&](const QVariantMap &card) {
        return isPinned(card) || card.value(QStringLiteral("important")).toBool();
    };
    QHash<QString, int> memberCounts;
    QHash<QString, bool> leaderAssigned;
    for (const QVariantMap &card : cards) {
        const QString bundle = card.value(QStringLiteral("bundle_id")).toString();
        if (bundle.isEmpty() || isExempt(card)) continue;
        memberCounts[bundle] = memberCounts.value(bundle) + 1;
    }
    for (QVariantMap &card : cards) {
        const QString bundle = card.value(QStringLiteral("bundle_id")).toString();
        if (bundle.isEmpty() || isExempt(card)) {
            card.remove(QStringLiteral("bundle_leader"));
            card.remove(QStringLiteral("bundle_member_count"));
            continue;
        }
        card.insert(QStringLiteral("bundle_leader"), !leaderAssigned.value(bundle));
        if (!leaderAssigned.value(bundle)) leaderAssigned.insert(bundle, true);
        card.insert(QStringLiteral("bundle_member_count"), memberCounts.value(bundle));
    }
}
int CardStore::pendingOps() const { return m_outbox.size(); }
void CardStore::setOnline(bool online) {
    if (m_online == online) return;
    m_online = online; emit onlineChanged();
    if (online) { m_failedOp.clear(); flush(); refresh(); }
}
void CardStore::setTransport(cardstore::OpTransport *transport) {
    m_transport = transport;
    flush();
}
void CardStore::refresh() {
    if (m_online && m_open) emit requestCards(QStringLiteral("/v1/cards"));
}
bool CardStore::applyRemoteCards(const QVariantMap &sections) {
    if (!m_open) return false;
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
    // A snapshot received while an operation is pending must not resurrect its
    // removed card or overwrite its local note/pin state.
    for (const auto &mutation : m_mutations) {
        QHash<QString, QVariantMap> before, after;
        for (const auto &card : mutation.before) before.insert(card.value(QStringLiteral("id")).toString(), card);
        for (const auto &card : mutation.after) after.insert(card.value(QStringLiteral("id")).toString(), card);
        cards.erase(std::remove_if(cards.begin(), cards.end(), [&](const auto &card) {
            const QString id = card.value(QStringLiteral("id")).toString();
            return before.contains(id) && !after.contains(id);
        }), cards.end());
        for (auto &card : cards) {
            const QString id = card.value(QStringLiteral("id")).toString();
            if (!before.contains(id) || !after.contains(id)) continue;
            const auto old = before.value(id), optimistic = after.value(id);
            for (auto it = optimistic.begin(); it != optimistic.end(); ++it)
                if (old.value(it.key()) != it.value()) card.insert(it.key(), it.value());
        }
        for (const auto &card : mutation.after) {
            const QString id = card.value(QStringLiteral("id")).toString();
            if (!before.contains(id) && std::none_of(cards.begin(), cards.end(), [&](const auto &entry) { return entry.value(QStringLiteral("id")).toString() == id; })) cards.prepend(card);
        }
    }
    // Cold starts have a durable outbox but no in-memory mutation snapshots.
    // Keep unsent creations visible until the server acknowledges them.
    for (const auto &op : m_outbox) {
        const QString id = op.value(QStringLiteral("card_id")).toString();
        const QString type = op.value(QStringLiteral("type")).toString();
        if (type == QStringLiteral("create_card")) {
            if (std::any_of(cards.begin(), cards.end(), [&](const auto &card) { return card.value(QStringLiteral("id")).toString() == id; })) continue;
            const auto local = std::find_if(m_cards.begin(), m_cards.end(), [&](const auto &card) { return card.value(QStringLiteral("id")).toString() == id; });
            if (local != m_cards.end()) cards.insert(qMin(qsizetype(std::distance(m_cards.begin(), local)), cards.size()), *local);
        } else if (type == QStringLiteral("archive") || type == QStringLiteral("done") || type == QStringLiteral("snooze")) {
            cards.erase(std::remove_if(cards.begin(), cards.end(), [&](const auto &card) { return card.value(QStringLiteral("id")).toString() == id; }), cards.end());
        }
    }
    const auto before = m_cards;
    replaceCards(cards);
    persistCards(before, cards);
    // Proactively cache mail and has_body agent bodies after a remote snapshot
    // so first-slice offline reading is not contingent on opening each card.
    // Mail behavior is unchanged; agent cards opt in via has_body from /v1/cards.
    if (m_online) {
        for (const QVariantMap &card : m_cards) {
            const bool isMail = card.value(QStringLiteral("source")).toString() == QStringLiteral("mail");
            if (!isMail && !card.value(QStringLiteral("has_body")).toBool()) continue;
            const QString id = card.value(QStringLiteral("id")).toString();
            if (cachedMailBody(id).isEmpty())
                emit requestMailBodyGet(id, QStringLiteral("/v1/cards/") + id + QStringLiteral("/body"));
        }
    }
    return true;
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
bool CardStore::createCard(const QString &title, const QString &summary, const QString &kind) {
    const QString text = title.trimmed();
    if (text.isEmpty() || (kind != QStringLiteral("manual") && kind != QStringLiteral("journal"))) return false;
    QVariantMap args{{QStringLiteral("title"), text}, {QStringLiteral("summary"), summary.trimmed()}};
    if (kind == QStringLiteral("journal")) {
        args.insert(QStringLiteral("kind"), kind);
        args.insert(QStringLiteral("body"), text);
        args.insert(QStringLiteral("title"), text.section(QLatin1Char('\n'), 0, 0));
        args.insert(QStringLiteral("summary"), text.section(QLatin1Char('\n'), 1));
    }
    return !enqueueOp(QUuid::createUuid().toString(QUuid::WithoutBraces), QStringLiteral("create_card"), args).isEmpty();
}
QString CardStore::enqueueOp(const QString &cardId, const QString &type, const QVariantMap &args) {
    if (!m_open || QUuid(cardId).isNull()) return {};
    static const QSet<QString> types = {QStringLiteral("done"), QStringLiteral("archive"), QStringLiteral("note"), QStringLiteral("pin"), QStringLiteral("unpin"),
        QStringLiteral("reorder_pins"), QStringLiteral("reorder_cards"), QStringLiteral("create_card"), QStringLiteral("snooze"), QStringLiteral("bundle_archive"), QStringLiteral("bundle_done"), QStringLiteral("take_out")};
    if (!types.contains(type)) return {};
    const QString id = QUuid::createUuid().toString(QUuid::WithoutBraces);
    const QVariantMap payload{{QStringLiteral("op_id"), id}, {QStringLiteral("card_id"), cardId},
        {QStringLiteral("type"), type}, {QStringLiteral("args"), args}};

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
                    !isPinned(card);
            }), cards.end());
            changed = cards.size() != oldSize;
        }
    } else if (type == QStringLiteral("note")) {
        for (QVariantMap &card : cards) {
            if (card.value(QStringLiteral("id")).toString() != cardId) continue;
            card.insert(QStringLiteral("note"), args.value(QStringLiteral("note")));
            changed = true;
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
            changed = true;
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
        const QString kind = args.value(QStringLiteral("kind"), QStringLiteral("manual")).toString();
        if (kind != QStringLiteral("manual") && kind != QStringLiteral("journal")) return {};
        if (kind == QStringLiteral("journal") && args.value(QStringLiteral("body")).toString().trimmed().isEmpty()) return {};
        QVariantMap card{{QStringLiteral("id"), cardId}, {QStringLiteral("source"), kind},
            {QStringLiteral("title"), args.value(QStringLiteral("title"))}, {QStringLiteral("summary"), args.value(QStringLiteral("summary"))},
            {QStringLiteral("state"), QStringLiteral("open")}, {QStringLiteral("section"), QStringLiteral("now")}};
        if (card.value(QStringLiteral("title")).toString().trimmed().isEmpty()) return {};
        cards.prepend(card);
        changed = true;
    }
    if (changed && (type == QStringLiteral("pin") || type == QStringLiteral("unpin") ||
                    type == QStringLiteral("reorder_pins") || type == QStringLiteral("create_card"))) {
        std::stable_sort(cards.begin(), cards.end(), [&](const QVariantMap &left, const QVariantMap &right) {
            const bool leftPinned = isPinned(left), rightPinned = isPinned(right);
            if (leftPinned != rightPinned) return leftPinned;
            if (leftPinned) return left.value(QStringLiteral("pinned_rank")).toLongLong() < right.value(QStringLiteral("pinned_rank")).toLongLong();
            return false;
        });
    }

    const auto before = m_cards;
    if (changed) {
        m_mutations.insert(id, {before, cards});
        replaceCards(cards);
    }
    m_outbox.append(payload);
    persistCards(before, cards, {payload});
    emit pendingOpsChanged();
    return id;
}
QVariantMap CardStore::archiveBundleNow(const QString &bundleId) {
    if (!m_open || bundleId.isEmpty()) return {};
    for (auto it = m_bundleArchives.begin(); it != m_bundleArchives.end();)
        it = it->deadline.hasExpired() ? m_bundleArchives.erase(it) : std::next(it);
    const auto isPinned = [](const QVariantMap &card) {
        const QVariant rank = card.value(QStringLiteral("pinned_rank"));
        return card.contains(QStringLiteral("pinned_rank")) && rank.isValid() && !rank.isNull();
    };
    // R22: every open unpinned card in the bundle; pinned cards stay open.
    const QList<QVariantMap> before = m_cards;
    QList<QVariantMap> after;
    QSet<QString> archived;
    for (const QVariantMap &card : before) {
        if (card.value(QStringLiteral("bundle_id")).toString() == bundleId &&
            card.value(QStringLiteral("state")).toString() == QStringLiteral("open") && !isPinned(card))
            archived.insert(card.value(QStringLiteral("id")).toString());
        else
            after.append(card);
    }
    if (archived.isEmpty()) return {};
    // One existing per-card `archive` op per message: the server archives each
    // mail thread in its originating Gmail account (R7). Each op keeps its own
    // mutation so a rejected op restores only its card, and a remote snapshot
    // cannot resurrect a card while its op is pending.
    BundleArchive entry{before, {}, QDeadlineTimer(8000)};
    QList<QVariantMap> ops;
    for (const QVariantMap &card : before) {
        const QString cardId = card.value(QStringLiteral("id")).toString();
        if (!archived.contains(cardId)) continue;
        const QString opId = QUuid::createUuid().toString(QUuid::WithoutBraces);
        ops.append({{QStringLiteral("op_id"), opId}, {QStringLiteral("card_id"), cardId},
            {QStringLiteral("type"), QStringLiteral("archive")}, {QStringLiteral("args"), QVariantMap{}}});
        QList<QVariantMap> withCard;
        for (const QVariantMap &other : before) {
            const QString otherId = other.value(QStringLiteral("id")).toString();
            if (otherId == cardId || !archived.contains(otherId)) withCard.append(other);
        }
        m_mutations.insert(opId, {withCard, after});
        entry.ops.append({cardId, opId});
    }
    replaceCards(after);
    m_outbox.append(ops);
    persistCards(before, after, ops);
    emit pendingOpsChanged();
    const QString token = QUuid::createUuid().toString(QUuid::WithoutBraces);
    m_bundleArchives.insert(token, std::move(entry));
    return {{QStringLiteral("token"), token}, {QStringLiteral("count"), archived.size()}};
}
bool CardStore::undoBundleArchive(const QString &token) {
    if (!m_open || !m_bundleArchives.contains(token)) return false;
    const BundleArchive entry = m_bundleArchives.take(token);
    if (entry.deadline.hasExpired()) return false;
    QHash<QString, QVariantMap> previous;
    for (const QVariantMap &card : entry.before) previous.insert(card.value(QStringLiteral("id")).toString(), card);
    QSet<QString> present;
    for (const QVariantMap &card : m_cards) present.insert(card.value(QStringLiteral("id")).toString());
    QSet<QString> restore;
    QStringList cancelled;
    QList<QVariantMap> reverseOps;
    for (const auto &[cardId, opId] : entry.ops) {
        // A card already back (for example after a rejected op) needs nothing.
        if (present.contains(cardId)) continue;
        const bool queued = opId != m_inFlight && std::any_of(m_outbox.begin(), m_outbox.end(),
            [&](const QVariantMap &op) { return op.value(QStringLiteral("op_id")).toString() == opId; });
        // Undo owns the card from here: a late failure of an in-flight op
        // must not restore it a second time.
        m_mutations.remove(opId);
        if (queued) {
            m_outbox.erase(std::remove_if(m_outbox.begin(), m_outbox.end(),
                [&](const QVariantMap &op) { return op.value(QStringLiteral("op_id")).toString() == opId; }), m_outbox.end());
            m_durableOps.remove(opId);
            cancelled.append(opId);
            restore.insert(cardId);
        } else if (previous.value(cardId).value(QStringLiteral("source")).toString() == QStringLiteral("mail")) {
            // Already sent: Gmail archive removed INBOX, so the existing source
            // action adds it back in the same account. Non-mail archives have
            // no reverse op and stay archived rather than reappear falsely.
            reverseOps.append({{QStringLiteral("op_id"), QUuid::createUuid().toString(QUuid::WithoutBraces)},
                {QStringLiteral("card_id"), cardId}, {QStringLiteral("type"), QStringLiteral("gmail.label_add")},
                {QStringLiteral("args"), QVariantMap{{QStringLiteral("label"), QStringLiteral("INBOX")}}}});
            restore.insert(cardId);
        }
    }
    if (restore.isEmpty()) return false;
    // Put each card back right after its nearest earlier neighbour that is
    // still listed, so its previous order survives unrelated changes.
    QList<QVariantMap> cards = m_cards;
    for (int row = 0; row < entry.before.size(); ++row) {
        const QString cardId = entry.before[row].value(QStringLiteral("id")).toString();
        if (!restore.contains(cardId)) continue;
        int at = 0;
        for (int prior = row - 1; prior >= 0 && at == 0; --prior) {
            const QString anchor = entry.before[prior].value(QStringLiteral("id")).toString();
            for (int i = 0; i < cards.size(); ++i)
                if (cards[i].value(QStringLiteral("id")).toString() == anchor) { at = i + 1; break; }
        }
        cards.insert(at, entry.before[row]);
    }
    // Keep restored cards visible across remote snapshots until the reverse
    // op is acknowledged, as for any other pending op.
    for (const QVariantMap &op : reverseOps) {
        const QString cardId = op.value(QStringLiteral("card_id")).toString();
        QList<QVariantMap> without = cards;
        without.erase(std::remove_if(without.begin(), without.end(),
            [&](const QVariantMap &card) { return card.value(QStringLiteral("id")).toString() == cardId; }), without.end());
        m_mutations.insert(op.value(QStringLiteral("op_id")).toString(), {without, cards});
    }
    const QList<QVariantMap> current = m_cards;
    replaceCards(cards);
    m_outbox.append(reverseOps);
    persistCards(current, cards, reverseOps, cancelled);
    emit pendingOpsChanged();
    return true;
}
void CardStore::flush() {
    if (m_online && m_open && m_inFlight.isEmpty()) flushNext();
}
void CardStore::flushNext() {
    if (!m_online || !m_inFlight.isEmpty()) return;
    if (m_outbox.isEmpty()) { emit flushFinished(); return; }
    const QVariantMap payload = m_outbox.first();
    const QString id = payload.value(QStringLiteral("op_id")).toString();
    if (id == m_failedOp) return;
    if (!m_durableOps.contains(id)) return;
    m_inFlight = id;
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
    // Keep the head in flight until its durable acknowledgement is committed.
    // A transport success alone is not a server acknowledgement.
    if (status < 200 || status >= 300 || response.value(QStringLiteral("ok")) != QVariant(true)) {
        m_inFlight.clear();
        m_failedOp = opId;
        undoOp(opId);
        emit operationFailed(opId, status); return;
    }
    QMetaObject::invokeMethod(m_storage, [this, opId] {
        QSqlQuery q(m_db);
        q.prepare(QStringLiteral("DELETE FROM outbox WHERE op_id=?")); q.addBindValue(opId);
        const bool ok = q.exec();
        const QString error = q.lastError().text();
        QMetaObject::invokeMethod(this, [this, opId, ok, error] {
            m_inFlight.clear();
            if (!ok) { m_failedOp = opId; undoOp(opId); emit storageError(error); return; }
            if (!m_outbox.isEmpty() && m_outbox.first().value(QStringLiteral("op_id")).toString() == opId) m_outbox.removeFirst();
            m_durableOps.remove(opId);
            m_mutations.remove(opId);
            emit pendingOpsChanged();
            flush();
        }, Qt::QueuedConnection);
    }, Qt::QueuedConnection);
}
QVariantMap CardStore::cachedMailBody(const QString &cardId) const {
    return m_mailBodies.value(cardId);
}
void CardStore::requestMailBody(const QString &cardId) {
    if (!m_online || !m_open || cardId.isEmpty()) return;
    // Mail cards keep their existing fetchable body; non-mail cards opt in
    // via has_body from /v1/cards. Cards without either have no body route.
    const QVariantMap *card = nullptr;
    for (const QVariantMap &c : m_cards) {
        if (c.value(QStringLiteral("id")).toString() == cardId) { card = &c; break; }
    }
    if (!card) return;
    const bool isMail = card->value(QStringLiteral("source")).toString() == QStringLiteral("mail");
    if (!isMail && !card->value(QStringLiteral("has_body")).toBool()) return;
    emit requestMailBodyGet(cardId, QStringLiteral("/v1/cards/") + cardId + QStringLiteral("/body"));
}
void CardStore::applyRemoteMailBody(const QString &cardId, const QVariantMap &body) {
    if (!m_open || cardId.isEmpty()) return;
    const QString html = body.value(QStringLiteral("html")).toString();
    QString sourceUrl = body.value(QStringLiteral("source_url")).toString();
    // Body responses omit empty source_url; SQLite requires a non-null string.
    if (sourceUrl.isNull()) sourceUrl = QStringLiteral("");
    // A malformed/error response must not replace a usable cached body.
    if (html.trimmed().isEmpty()) { emit mailBodyFailed(cardId); return; }
    const auto previous = m_mailBodies.value(cardId);
    m_mailBodies.insert(cardId, {{QStringLiteral("html"), html}, {QStringLiteral("source_url"), sourceUrl}});
    QMetaObject::invokeMethod(m_storage, [this, cardId, html, sourceUrl, previous] {
        QSqlQuery q(m_db);
        q.prepare(QStringLiteral("INSERT INTO mail_bodies(card_id,html,source_url) VALUES(?,?,?) "
                                 "ON CONFLICT(card_id) DO UPDATE SET html=excluded.html, source_url=excluded.source_url"));
        q.addBindValue(cardId); q.addBindValue(html); q.addBindValue(sourceUrl);
        const bool ok = q.exec();
        const QString error = q.lastError().text();
        QMetaObject::invokeMethod(this, [this, cardId, previous, ok, error] {
            if (!ok) { m_mailBodies.insert(cardId, previous); emit storageError(error); emit mailBodyFailed(cardId); return; }
            emit mailBodyChanged(cardId);
        }, Qt::QueuedConnection);
    }, Qt::QueuedConnection);
}
void CardStore::reportMailBodyFailed(const QString &cardId) {
    emit mailBodyFailed(cardId);
}
QString CardStore::openCachedFile(const QString &cardId, const QString &dataUrl) const {
    if (!m_open || cardId.isEmpty() || dataUrl.isEmpty()) return {};
    if (!dataUrl.startsWith(QStringLiteral("data:"))) return {};
    if (cardId.contains(QLatin1Char('/')) || cardId.contains(QLatin1Char('\\')) ||
        cardId.contains(QStringLiteral("..")) || cardId.contains(QChar(u'\0')) ||
        cardId.startsWith(QLatin1Char('.'))) return {};
    // File bytes survive restart because they live inside the mail_bodies HTML
    // cached in SQLite. Decode only a data: URL present byte-for-byte in the
    // exact cached body for this card; never trust a producer-supplied path.
    const QVariantMap cached = cachedMailBody(cardId);
    const QString html = cached.value(QStringLiteral("html")).toString();
    if (html.isEmpty() || !html.contains(dataUrl)) return {};
    const int comma = dataUrl.indexOf(QLatin1Char(','));
    if (comma < 0) return {};
    QString meta = dataUrl.left(comma);
    const QString encodedData = dataUrl.mid(comma + 1);
    if (encodedData.isEmpty()) return {};
    const QString base64Marker = QStringLiteral(";base64");
    if (!meta.endsWith(base64Marker)) return {};
    meta.chop(base64Marker.size());
    const QString nameMarker = QStringLiteral(";name=");
    const int namePos = meta.indexOf(nameMarker);
    if (namePos < 0) return {};
    QString encodedName = meta.mid(namePos + nameMarker.size());
    const int nextParam = encodedName.indexOf(QLatin1Char(';'));
    if (nextParam >= 0) encodedName = encodedName.left(nextParam);
    if (encodedName.isEmpty() || encodedName.contains(QLatin1Char('/')) ||
        encodedName.contains(QLatin1Char('\\'))) return {};
    // Server percent-encodes the name= parameter; decode then apply the same
    // basename/traversal rules as ingest (reject /, \, NUL, leading dot).
    const QString name = QUrl::fromPercentEncoding(encodedName.toUtf8());
    if (name.isEmpty() || name.contains(QLatin1Char('/')) ||
        name.contains(QLatin1Char('\\')) || name.contains(QChar(u'\0')) ||
        name.startsWith(QLatin1Char('.'))) return {};
    const QByteArray nameBytes = name.toUtf8();
    if (nameBytes.size() < 1 || nameBytes.size() > 255) return {};
    const QByteArray raw = encodedData.toLatin1();
    const QByteArray bytes = QByteArray::fromBase64(raw, QByteArray::AbortOnBase64DecodingErrors);
    if (bytes.isEmpty()) return {};
    const QString base = QStandardPaths::writableLocation(QStandardPaths::CacheLocation);
    if (base.isEmpty()) return {};
    QDir dir(base + QStringLiteral("/card-files/") + cardId);
    if (!dir.mkpath(QStringLiteral("."))) return {};
    const QString filePath = dir.filePath(name);
    // Defense in depth: the cleaned single-segment name must stay inside its
    // per-card directory even after cleaning.
    if (QDir::cleanPath(filePath) != QDir::cleanPath(dir.absolutePath() + QLatin1Char('/') + name)) return {};
    QFile file(filePath);
    if (!file.open(QIODevice::WriteOnly | QIODevice::Truncate)) return {};
    if (file.write(bytes) != bytes.size()) return {};
    file.close();
    return QUrl::fromLocalFile(filePath).toString();
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
    case SenderNameRole: return card.value(QStringLiteral("sender_name"));
    default: return {};
    }
}
QHash<int,QByteArray> CardStore::roleNames() const {
    return {{CardRole,"card"},{CardIdRole,"cardId"},{TitleRole,"title"},{SectionRole,"section"},{SenderNameRole,"sender_name"}};
}
