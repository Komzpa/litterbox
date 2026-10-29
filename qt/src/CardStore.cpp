// SPDX-License-Identifier: MIT
#include "CardStore.h"
#include <algorithm>
#include <vector>
#include <QByteArray>
#include <QDir>
#include <QFile>
#include <QHash>
#include <QJsonDocument>
#include <QJsonObject>
#include <QJsonArray>
#include <QPointer>
#include <QSet>
#include <QSqlError>
#include <QSqlQuery>
#include <QStandardPaths>
#include <QTimer>
#include <QUrl>
#include <QUuid>
#include <QtQml/qqml.h>
#include <unistd.h>
#include <fcntl.h>
#include <QFileInfo>
#include <QLoggingCategory>

// One 429 pauses only the op pump, for this long; the server's per-device token
// bucket refills five requests per second.
static constexpr int kRateLimitRetryMs = 1000;

// Default to retaining unknown operations. Only reversible triage may roll back.
bool CardStore::ownerWrite(const QVariantMap &op) {
    const auto args = op.value(QStringLiteral("args")).toMap();
    if (args.contains(QStringLiteral("note")) || args.contains(QStringLiteral("body")) ||
        args.contains(QStringLiteral("title")) || args.contains(QStringLiteral("summary"))) return true;
    static const QSet<QString> reversible{QStringLiteral("archive"), QStringLiteral("bundle_archive"),
        QStringLiteral("spam"), QStringLiteral("bundle_spam"), QStringLiteral("snooze"),
        QStringLiteral("pin"), QStringLiteral("unpin"), QStringLiteral("gmail.label_add")};
    return !reversible.contains(op.value(QStringLiteral("type")).toString());
}

bool CardStore::appendDraftRecord(const QVariantMap &record) {
    return appendDraftRecords({record});
}
// One write and one file+directory fsync cover every record.
bool CardStore::appendDraftRecords(const QList<QVariantMap> &records) {
    QFile journal(m_draftPath);
    QByteArray bytes;
    for (const QVariantMap &record : records)
        bytes += '\n' + QJsonDocument(QJsonObject::fromVariantMap(record)).toJson(QJsonDocument::Compact) + '\n';
    // Leading newline isolates a torn final record from the next append.
    if (!journal.open(QIODevice::WriteOnly | QIODevice::Append, QFileDevice::ReadOwner | QFileDevice::WriteOwner) ||
        journal.write(bytes) != bytes.size() || !journal.flush() || ::fsync(journal.handle()) != 0) {
        emit storageError(QStringLiteral("Cannot durably save owner draft: ") + journal.errorString());
        return false;
    }
    // The directory entry only changes when the journal file is created; once one
    // directory fsync has succeeded, appends need the file fsync alone.
    if (m_draftDirSynced) return true;
    const QByteArray directory = QFile::encodeName(QFileInfo(m_draftPath).absolutePath());
    const int fd = ::open(directory.constData(), O_RDONLY | O_DIRECTORY);
    if (fd < 0) { emit storageError(QStringLiteral("Cannot open draft directory for sync")); return false; }
    const bool synced = ::fsync(fd) == 0;
    ::close(fd);
    if (!synced) { emit storageError(QStringLiteral("Cannot sync draft directory")); return false; }
    m_draftDirSynced = true;
    return true;
}

// Insert missing optimistic cards after their nearest surviving local neighbour.
static void restoreCardOrder(QList<QVariantMap> &cards, const QList<QVariantMap> &before,
                             const QSet<QString> &restore) {
    for (int row = 0; row < before.size(); ++row) {
        if (!restore.contains(before[row].value(QStringLiteral("id")).toString())) continue;
        int at = 0;
        for (int prior = row - 1; prior >= 0 && at == 0; --prior) {
            const QString anchor = before[prior].value(QStringLiteral("id")).toString();
            for (int i = 0; i < cards.size(); ++i)
                if (cards[i].value(QStringLiteral("id")).toString() == anchor) { at = i + 1; break; }
        }
        cards.insert(at, before[row]);
    }
}

CardStore::CardStore(QObject *parent) : QAbstractListModel(parent),
    m_connection(QUuid::createUuid().toString(QUuid::WithoutBraces)),
    m_storage(new QObject), m_bundleUndoTimer(new QTimer(this)) {
    m_bundleUndoTimer->setSingleShot(true);
    connect(m_bundleUndoTimer, &QTimer::timeout, this, &CardStore::expireBundleArchives);
    m_retryTimer.setSingleShot(true);
    connect(&m_retryTimer, &QTimer::timeout, this, [this] { m_deferredWrites.clear(); flush(); });
    m_retryTimer.setTimerType(Qt::PreciseTimer);
    m_bodyPrefetchTimer.setSingleShot(true);
    m_bodyPrefetchTimer.setTimerType(Qt::PreciseTimer);
    connect(&m_bodyPrefetchTimer, &QTimer::timeout, this, &CardStore::queueMailBodyPrefetch);
    m_photoPrefetchTimer.setSingleShot(true);
    m_photoPrefetchTimer.setTimerType(Qt::PreciseTimer);
    connect(&m_photoPrefetchTimer, &QTimer::timeout, this, &CardStore::queueSenderPhotoPrefetch);
    m_storage->moveToThread(&m_storageThread);
    connect(&m_storageThread, &QThread::finished, m_storage, &QObject::deleteLater);
    m_storageThread.start();
}
CardStore::~CardStore() {
    m_bodyPrefetchTimer.stop();
    m_bodyPrefetchQueue.clear();
    m_bodyRequests.clear();
    m_photoPrefetchTimer.stop();
    m_photoPrefetchQueue.clear();
    m_photoRequests.clear();
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
    m_bodyPrefetchTimer.stop();
    m_bodyPrefetchQueue.clear();
    m_bodyRequests.clear();
    m_photoPrefetchTimer.stop();
    m_photoPrefetchQueue.clear();
    m_photoRequests.clear();
    m_photoUnavailable.clear();
    m_drafts.clear();
    m_draftOrder.clear();
    m_draftPath = path + QStringLiteral(".drafts.jsonl");
    QSet<QString> retired;
    QFile journal(m_draftPath);
    if (journal.exists()) {
        if (!journal.open(QIODevice::ReadOnly)) { emit storageError(journal.errorString()); return false; }
        while (!journal.atEnd()) {
            const auto record = QJsonDocument::fromJson(journal.readLine()).object().toVariantMap();
            const QString id = record.value(QStringLiteral("op_id")).toString();
            if (id.isEmpty()) continue; // Empty lines or a torn, unaccepted append.
            if (record.value(QStringLiteral("retired")).toBool()) {
                retired.insert(id);
                m_drafts.remove(id);
                const auto replacement = record.value(QStringLiteral("replacement")).toMap();
                const QString replacementId = replacement.value(QStringLiteral("op_id")).toString();
                if (!replacementId.isEmpty()) {
                    m_drafts.insert(replacementId, replacement);
                    m_draftOrder.append(replacementId);
                }
            }
            else if (record.value(QStringLiteral("acked")).toBool()) m_drafts.remove(id);
            else { m_drafts.insert(id, record); m_draftOrder.append(id); }
        }
    }
    QList<QVariantMap> cards, outbox;
    QSet<QString> cachedIds;
    QHash<QString, QString> photos;
    bool opened = false;
    // Startup only, before the window exists. Every subsequent write is queued.
    QMetaObject::invokeMethod(m_storage, [&, this] {
        m_db = QSqlDatabase::addDatabase(QStringLiteral("QSQLITE"), m_connection);
        m_db.setDatabaseName(path);
        if (!m_db.open()) { emit storageError(m_db.lastError().text()); return; }
        if (!execute(QStringLiteral("PRAGMA synchronous=FULL")) ||
            !execute(QStringLiteral("CREATE TABLE IF NOT EXISTS cards (id TEXT PRIMARY KEY, position INTEGER NOT NULL, payload TEXT NOT NULL)")) ||
            !execute(QStringLiteral("CREATE TABLE IF NOT EXISTS outbox (seq INTEGER PRIMARY KEY AUTOINCREMENT, op_id TEXT UNIQUE NOT NULL, payload TEXT NOT NULL)")) ||
            !execute(QStringLiteral("CREATE TABLE IF NOT EXISTS mail_bodies (card_id TEXT PRIMARY KEY, html TEXT NOT NULL, source_url TEXT NOT NULL, messages TEXT NOT NULL DEFAULT '[]')")) ||
            !execute(QStringLiteral("CREATE TABLE IF NOT EXISTS sender_photo_files (card_id TEXT PRIMARY KEY, file_url TEXT NOT NULL)"))) return;
        QSqlQuery bodyColumns(m_db);
        if (!bodyColumns.exec(QStringLiteral("PRAGMA table_info(mail_bodies)"))) return;
        bool hasMessages = false;
        while (bodyColumns.next())
            if (bodyColumns.value(1).toString() == QStringLiteral("messages")) hasMessages = true;
        if (!hasMessages && !execute(QStringLiteral("ALTER TABLE mail_bodies ADD COLUMN messages TEXT NOT NULL DEFAULT '[]'"))) return;
        QSqlQuery columns(m_db);
        if (!columns.exec(QStringLiteral("PRAGMA table_info(cards)"))) return;
        bool hasSenderName = false;
        while (columns.next())
            if (columns.value(1).toString() == QStringLiteral("sender_name")) hasSenderName = true;
        if (!hasSenderName && !execute(QStringLiteral("ALTER TABLE cards ADD COLUMN sender_name TEXT NOT NULL DEFAULT ''"))) return;
        QSqlQuery outboxColumns(m_db);
        if (!outboxColumns.exec(QStringLiteral("PRAGMA table_info(outbox)"))) return;
        bool hasOwnerWrite = false;
        while (outboxColumns.next()) if (outboxColumns.value(1).toString() == QStringLiteral("owner_write")) hasOwnerWrite = true;
        if (!hasOwnerWrite && !execute(QStringLiteral("ALTER TABLE outbox ADD COLUMN owner_write INTEGER NOT NULL DEFAULT 1"))) return;
        // Migrate old pending ops through the same conservative policy.
        QSqlQuery pending(m_db);
        if (!pending.exec(QStringLiteral("SELECT op_id,payload FROM outbox"))) return;
        while (pending.next()) {
            QSqlQuery classify(m_db);
            classify.prepare(QStringLiteral("UPDATE outbox SET owner_write=? WHERE op_id=?"));
            classify.addBindValue(ownerWrite(QJsonDocument::fromJson(pending.value(1).toByteArray()).object().toVariantMap()));
            classify.addBindValue(pending.value(0));
            if (!classify.exec()) return;
        }
        if (!execute(QStringLiteral("CREATE TABLE IF NOT EXISTS retired_ops (op_id TEXT PRIMARY KEY)")) ||
            !execute(QStringLiteral("CREATE TABLE IF NOT EXISTS acknowledged_ops (op_id TEXT PRIMARY KEY)")) ||
            !execute(QStringLiteral("DROP TRIGGER IF EXISTS retain_owner_outbox")) ||
            !execute(QStringLiteral("CREATE TRIGGER retain_owner_outbox BEFORE DELETE ON outbox WHEN OLD.owner_write=1 AND NOT EXISTS(SELECT 1 FROM acknowledged_ops WHERE op_id=OLD.op_id) AND NOT EXISTS(SELECT 1 FROM retired_ops WHERE op_id=OLD.op_id) BEGIN SELECT RAISE(ABORT,'Refusing deletion of unacknowledged owner op'); END")) ||
            !execute(QStringLiteral("CREATE TRIGGER IF NOT EXISTS retain_owner_card BEFORE DELETE ON cards WHEN EXISTS(SELECT 1 FROM outbox WHERE owner_write=1 AND json_extract(payload,'$.card_id')=OLD.id) BEGIN SELECT RAISE(ABORT,'Refusing deletion of unacknowledged owner row'); END")) ||
            !execute(QStringLiteral("CREATE TRIGGER IF NOT EXISTS immutable_owner_outbox BEFORE UPDATE OF payload,op_id ON outbox WHEN OLD.owner_write=1 BEGIN SELECT RAISE(ABORT,'Refusing overwrite of owner op'); END"))) return;
        for (const auto &id : retired) {
            QSqlQuery retire(m_db);
            retire.prepare(QStringLiteral("INSERT OR IGNORE INTO retired_ops VALUES(?)"));
            retire.addBindValue(id);
            if (!retire.exec()) return;
            retire.prepare(QStringLiteral("DELETE FROM outbox WHERE op_id=?"));
            retire.addBindValue(id);
            if (!retire.exec()) return;
        }
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
        // Replay fsynced saves that a crash prevented the async DB writer from committing.
        for (const QString &draftId : std::as_const(m_draftOrder)) {
            if (!m_drafts.contains(draftId)) continue;
            const QVariantMap draft = m_drafts.value(draftId);
            const QVariantMap op = draft.value(QStringLiteral("op")).toMap();
            const QString id = op.value(QStringLiteral("op_id")).toString();
            if (std::any_of(outbox.cbegin(), outbox.cend(), [&](const auto &entry) { return entry.value(QStringLiteral("op_id")).toString() == id; })) continue;
            QSqlQuery insert(m_db);
            insert.prepare(QStringLiteral("INSERT INTO outbox(op_id,payload,owner_write) VALUES(?,?,1)"));
            insert.addBindValue(id);
            insert.addBindValue(QString::fromUtf8(QJsonDocument(QJsonObject::fromVariantMap(op)).toJson(QJsonDocument::Compact)));
            if (!insert.exec()) return;
            outbox.append(op);
        }
        // Apply pending draft rows in journal order, including newer edits to the same card.
        QList<QVariantMap> recovered = cards;
        for (const auto &op : std::as_const(outbox)) {
            const auto draft = m_drafts.value(op.value(QStringLiteral("op_id")).toString());
            for (const QVariant &value : draft.value(QStringLiteral("cards")).toList()) {
                const auto card = value.toMap();
                const auto found = std::find_if(recovered.begin(), recovered.end(), [&](const auto &entry) { return entry.value(QStringLiteral("id")) == card.value(QStringLiteral("id")); });
                if (found == recovered.end()) recovered.prepend(card);
                else *found = card;
            }
        }
        if (!writeCards(cards, recovered)) return;
        cards = std::move(recovered);
        if (!q.exec(QStringLiteral("SELECT card_id FROM mail_bodies"))) return;
        while (q.next()) cachedIds.insert(q.value(0).toString());
        // R67: device photo cache survives restart; file URLs are local only.
        if (!q.exec(QStringLiteral("SELECT card_id,file_url FROM sender_photo_files"))) return;
        while (q.next()) {
            const QString id = q.value(0).toString();
            const QString fileUrl = q.value(1).toString();
            if (!id.isEmpty() && fileUrl.startsWith(QStringLiteral("file://")))
                photos.insert(id, fileUrl);
        }
        opened = true;
    }, Qt::BlockingQueuedConnection);
    if (!opened) return false;
    m_open = true;
    m_outbox = std::move(outbox);
    for (const auto &op : m_outbox) m_durableOps.insert(op.value(QStringLiteral("op_id")).toString());
    m_bodyCached = std::move(cachedIds);
    m_senderPhotos = std::move(photos);
    injectSenderPhotos(cards);
    replaceCards(std::move(cards));
    emit pendingOpsChanged();
    flush();
    queueMailBodyPrefetch();
    queueSenderPhotoPrefetch();
    return true;
}

void CardStore::retainOwnerRows(QList<QVariantMap> &cards) const {
    for (const auto &op : m_outbox) {
        if (!ownerWrite(op)) continue;
        const auto draft = m_drafts.value(op.value(QStringLiteral("op_id")).toString());
        QVariantList authored = draft.value(QStringLiteral("cards")).toList();
        const auto patches = draft.value(QStringLiteral("patches")).toMap();
        if (authored.isEmpty()) {
            const auto local = std::find_if(m_cards.cbegin(), m_cards.cend(), [&](const auto &entry) { return entry.value(QStringLiteral("id")) == op.value(QStringLiteral("card_id")); });
            if (local != m_cards.cend()) authored.append(*local);
        }
        for (const auto &value : std::as_const(authored)) {
            const auto row = value.toMap();
            const QString id = row.value(QStringLiteral("id")).toString();
            auto target = std::find_if(cards.begin(), cards.end(), [&](const auto &entry) { return entry.value(QStringLiteral("id")).toString() == id; });
            if (target == cards.end()) {
                qWarning() << "Refusing deletion of unacknowledged owner row" << id;
                cards.prepend(row);
            } else {
                const auto patch = patches.contains(id) ? patches.value(id).toMap() : op.value(QStringLiteral("args")).toMap();
                for (auto it = patch.begin(); it != patch.end(); ++it)
                    if (row.contains(it.key())) target->insert(it.key(), row.value(it.key()));
            }
        }
    }
}
// Pending operations are the tombstone source of truth, including after restart.
// Keep durable owner rows intact; removal is a projection, not lost owner data.
void CardStore::projectPendingRemovals(QList<QVariantMap> &cards) const {
    for (const auto &op : m_outbox) {
        const QString type = op.value(QStringLiteral("type")).toString();
        const QString id = op.value(QStringLiteral("card_id")).toString();
        const bool single = type == QStringLiteral("done") || type == QStringLiteral("archive") ||
            type == QStringLiteral("spam") || type == QStringLiteral("snooze") || type == QStringLiteral("delete_journal");
        const bool bundle = type == QStringLiteral("bundle_done") || type == QStringLiteral("bundle_archive") ||
            type == QStringLiteral("bundle_spam");
        if (!single && !bundle) continue;
        const bool undone = type == QStringLiteral("archive") && std::any_of(m_outbox.cbegin(), m_outbox.cend(), [&](const auto &pending) {
            return pending.value(QStringLiteral("card_id")).toString() == id &&
                pending.value(QStringLiteral("type")).toString() == QStringLiteral("gmail.label_add") &&
                pending.value(QStringLiteral("args")).toMap().value(QStringLiteral("label")).toString() == QStringLiteral("INBOX");
        });
        if (undone) continue;
        const QString bundleId = op.value(QStringLiteral("args")).toMap().value(QStringLiteral("bundle_id")).toString();
        const auto removed = m_drafts.value(op.value(QStringLiteral("op_id")).toString()).value(QStringLiteral("removed_cards")).toList();
        cards.erase(std::remove_if(cards.begin(), cards.end(), [&](const auto &card) {
            if (single) return card.value(QStringLiteral("id")).toString() == id;
            if (!removed.isEmpty()) return std::any_of(removed.cbegin(), removed.cend(), [&](const auto &value) {
                return value.toMap().value(QStringLiteral("id")) == card.value(QStringLiteral("id"));
            });
            const auto rank = card.value(QStringLiteral("pinned_rank"));
            return !bundleId.isEmpty() && card.value(QStringLiteral("bundle_id")).toString() == bundleId &&
                card.value(QStringLiteral("state")).toString() == QStringLiteral("open") && (!rank.isValid() || rank.isNull());
        }), cards.end());
    }
    cards.erase(std::remove_if(cards.begin(), cards.end(), [&](const auto &card) {
        return m_archivedAwaitingSnapshot.contains(card.value(QStringLiteral("id")).toString());
    }), cards.end());
}
void CardStore::replaceCards(QList<QVariantMap> cards) {
    const int pendingBefore = mailSyncPending();
    const bool completeBefore = mailSyncComplete();
    retainOwnerRows(cards);
    projectPendingRemovals(cards);
    deriveBundles(cards);
    // A live refresh can move or touch hundreds of rows at once; one model
    // notification per row stalls the GUI thread (each wakes the QML view).
    // Past a structural-churn threshold a single reset is cheaper and the
    // view shows the same final rows. Small edits keep fine-grained signals
    // so animations and persistent indexes survive.
    static constexpr int kResetThreshold = 32;
    QSet<QString> ids;
    for (const auto &card : cards) ids.insert(card.value(QStringLiteral("id")).toString());
    QStringList order = cardIds();
    // Count the real row operations the fine-grained path would emit: removals,
    // then one insert/move per operation on the post-removal order. A shift
    // caused by a single archive or insert is one change, not one change per
    // displaced row; role-only re-clusters count zero.
    QStringList projected = order;
    int structures = 0;
    for (int row = projected.size() - 1; row >= 0; --row) {
        if (ids.contains(projected[row])) continue;
        projected.removeAt(row);
        ++structures;
    }
    for (int row = 0; row < cards.size() && structures <= kResetThreshold; ++row) {
        const QString id = cards[row].value(QStringLiteral("id")).toString();
        if (row < projected.size() && projected[row] == id) continue;
        ++structures;  // an insert or a move will be needed here
        const int from = projected.indexOf(id, row);
        if (from < 0) projected.insert(row, id);
        else projected.move(from, row);
    }
    if (structures > kResetThreshold) {
        beginResetModel();
        m_cards = std::move(cards);
        endResetModel();
        if (pendingBefore != mailSyncPending() || completeBefore != mailSyncComplete()) emit mailSyncStateChanged();
        return;
    }
    for (int row = m_cards.size() - 1; row >= 0; --row) {
        if (ids.contains(m_cards[row].value(QStringLiteral("id")).toString())) continue;
        beginRemoveRows({}, row, row);
        m_cards.removeAt(row);
        // Keep order in lockstep with m_cards so the insert/move indexes below
        // address the same rows in both; a stale order yields out-of-range
        // beginMoveRows (and a crash on QList::move).
        order.removeAt(row);
        endRemoveRows();
    }
    std::vector<char> dirty(cards.size(), 0);
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
            dirty[row] = 1;
        }
    }
    // One emission per contiguous run of changed rows, not one per row.
    for (int row = 0; row < (int)cards.size();) {
        if (!dirty[row]) { ++row; continue; }
        int end = row;
        while (end + 1 < (int)cards.size() && dirty[end + 1]) ++end;
        emit dataChanged(index(row), index(end));
        row = end + 1;
    }
    if (pendingBefore != mailSyncPending() || completeBefore != mailSyncComplete()) emit mailSyncStateChanged();
}
bool CardStore::writeCards(const QList<QVariantMap> &before, const QList<QVariantMap> &after) {
    // Derived bundle flags are presentation state, not durable card changes.
    const auto stored = [](QVariantMap card) {
        card.remove(QStringLiteral("bundle_leader"));
        card.remove(QStringLiteral("bundle_member_count"));
        // R67: sender_photo is a device-local file:// URL, never durable in
        // the card payload; the mapping lives in sender_photo_files.
        card.remove(QStringLiteral("sender_photo"));
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
        QSqlQuery guard(m_db);
        guard.prepare(QStringLiteral("SELECT 1 FROM outbox WHERE owner_write=1 AND json_extract(payload,'$.card_id')=?"));
        guard.addBindValue(id);
        if (!guard.exec()) return false;
        if (guard.next()) { qWarning() << "Refusing deletion of unacknowledged owner row" << id; continue; }
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
                             const QList<QVariantMap> &ops, const QStringList &cancelledOps, quint64 bodySnapshot) {
    auto retained = after;
    retainOwnerRows(retained);
    QMetaObject::invokeMethod(m_storage, [this, before, after = std::move(retained), ops, cancelledOps, bodySnapshot] {
        bool ok = m_db.transaction();
        for (const QVariantMap &op : ops) {
            if (!ok) break;
            QSqlQuery q(m_db);
            ok = q.prepare(QStringLiteral("INSERT INTO outbox(op_id,payload,owner_write) VALUES(?,?,?)"));
            q.addBindValue(op.value(QStringLiteral("op_id")));
            q.addBindValue(QString::fromUtf8(QJsonDocument(QJsonObject::fromVariantMap(op)).toJson(QJsonDocument::Compact)));
            q.addBindValue(ownerWrite(op));
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
        QMetaObject::invokeMethod(this, [this, ops, ok, error, bodySnapshot] {
            if (!ok) {
                // Cancelled ops stay durable after a failed commit; the next
                // open() reloads that outbox as the source of truth.
                for (const QVariantMap &op : ops) {
                    if (ownerWrite(op)) continue; // Fsynced journal remains the recovery source.
                    const QString id = op.value(QStringLiteral("op_id")).toString();
                    m_outbox.erase(std::remove_if(m_outbox.begin(), m_outbox.end(), [&](const auto &entry) { return entry.value(QStringLiteral("op_id")).toString() == id; }), m_outbox.end());
                    undoOp(id);
                }
                if (!ops.isEmpty()) emit pendingOpsChanged();
                emit storageError(error);
                return;
            }
            for (const QVariantMap &op : ops) m_durableOps.insert(op.value(QStringLiteral("op_id")).toString());
            if (bodySnapshot) {
                m_durableBodySnapshot = bodySnapshot;
                emit mailSyncStateChanged();
            }
            flush();
        }, Qt::QueuedConnection);
    }, Qt::QueuedConnection);
}

void CardStore::undoOp(const QString &opId) {
    const auto pending = std::find_if(m_outbox.cbegin(), m_outbox.cend(), [&](const auto &op) { return op.value(QStringLiteral("op_id")).toString() == opId; });
    if (pending != m_outbox.cend() && ownerWrite(*pending)) {
        qWarning() << "Refusing rollback of unacknowledged owner op" << opId;
        return;
    }
    if (!m_mutations.contains(opId)) return;
    const Mutation mutation = m_mutations.take(opId);
    // Undo of a rejected archive never made the canonical card archived.
    if (mutation.archiveRejected) return;
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
    QSet<QString> restore;
    for (const auto &card : mutation.before) {
        const QString id = card.value(QStringLiteral("id")).toString();
        if (!after.contains(id) && std::none_of(cards.cbegin(), cards.cend(), [&](const QVariantMap &current) {
            return current.value(QStringLiteral("id")).toString() == id;
        })) restore.insert(id);
    }
    restoreCardOrder(cards, mutation.before, restore);
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
    if (online && !m_offlineReason.isEmpty()) {
        m_offlineReason.clear();
        emit offlineReasonChanged();
    }
    if (m_online == online) return;
    m_online = online; emit onlineChanged();
    if (online) { flush(); refresh(); queueMailBodyPrefetch(); queueSenderPhotoPrefetch(); }
    else {
        m_bodyPrefetchTimer.stop();
        m_bodyPrefetchQueue.clear();
        // HTTP requests are still alive; retain their identities until completion
        // so reconnecting cannot issue a duplicate for the same card.
        m_photoPrefetchTimer.stop();
        m_photoPrefetchQueue.clear();
        m_photoRequests.clear();
    }
}
void CardStore::reportOffline(const QString &reason) {
    const QString message = reason.isEmpty() ? QStringLiteral("Connection closed") : reason;
    if (m_offlineReason != message) {
        m_offlineReason = message;
        emit offlineReasonChanged();
    }
    setOnline(false);
}
void CardStore::setTransport(cardstore::OpTransport *transport) {
    m_transport = transport;
    flush();
}
Q_LOGGING_CATEGORY(lcRetry, "litterbox.retry", QtInfoMsg)

void CardStore::refresh() {
    // Retry and the next sync explicitly retry retained writes, without waiting
    // for the automatic backoff. Never change the operation's id or payload.
    if (m_online && !m_failedWrites.isEmpty()) {
        m_deferredWrites.clear();
        m_retryTimer.stop();
        flush();
    }
    if (m_online && m_open) emit requestCards(QStringLiteral("/v1/cards"), ++m_cardsRequestSequence);
}
void CardStore::retrySync() {
    qCDebug(lcRetry) << "retrySync entry online=" << m_online << "open=" << m_open
                     << "gateBypassed=" << (!m_online && m_open);
    if (m_online) refresh();
    else if (m_open) {
        qCDebug(lcRetry) << "retrySync emitting requestCards online=false open=true gateBypassed=true seq=" << (m_cardsRequestSequence + 1);
        emit requestCards(QStringLiteral("/v1/cards"), ++m_cardsRequestSequence);
    }
}
bool CardStore::applyRemoteCards(const QVariantMap &sections, quint64 requestId) {
    if (!m_open) return false;
    if (requestId == 0) requestId = ++m_cardsRequestSequence;
    if (requestId <= m_latestCardsResponse) {
        // A newer response already won. Never strand refresh liveness on a drop.
        refresh();
        return false;
    }
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
    m_latestCardsResponse = requestId;
    if (!m_rejectedReorder.isEmpty() && requestId >= m_rejectedReorderSnapshot) {
        const QString rejectedId = m_rejectedReorder;
        m_rejectedReorder.clear();
        QVariantList order;
        QSet<QString> included;
        for (const auto &op : std::as_const(m_outbox)) {
            if (op.value(QStringLiteral("op_id")).toString() != rejectedId) continue;
            for (const auto &value : op.value(QStringLiteral("args")).toMap().value(QStringLiteral("cards")).toList())
                if (ids.contains(value.toString())) { order.append(value); included.insert(value.toString()); }
        }
        for (const auto &card : cards) {
            const QVariant id = card.value(QStringLiteral("id"));
            if (!included.contains(id.toString())) order.append(id);
        }
        QVariantMap replacement;
        if (!order.isEmpty()) replacement = {{QStringLiteral("op_id"), QUuid::createUuid().toString(QUuid::WithoutBraces)},
            {QStringLiteral("card_id"), order.first()}, {QStringLiteral("type"), QStringLiteral("reorder_cards")},
            {QStringLiteral("args"), QVariantMap{{QStringLiteral("cards"), order}}}};
        retireRejectedOperation(rejectedId, replacement);
    }
    for (auto it = m_restoredAwaitingSnapshot.begin(); it != m_restoredAwaitingSnapshot.end();) {
        if (requestId > it.value()) {
            m_mutations.remove(it.key());
            it = m_restoredAwaitingSnapshot.erase(it);
        } else ++it;
    }
    // An acknowledged archive remains optimistic until a snapshot confirms its absence.
    for (auto it = m_archivedAwaitingSnapshot.begin(); it != m_archivedAwaitingSnapshot.end();) {
        const QString id = *it;
        const bool stillPresent = ids.contains(id);
        cards.erase(std::remove_if(cards.begin(), cards.end(), [&](const QVariantMap &card) {
            return stillPresent && card.value(QStringLiteral("id")).toString() == id;
        }), cards.end());
        it = stillPresent ? std::next(it) : m_archivedAwaitingSnapshot.erase(it);
    }
    // A snapshot received while an operation is pending must not resurrect its
    // removed card or overwrite its local note/pin state.
    for (const auto &mutation : m_mutations) {
        QHash<QString, QVariantMap> before, after;
        // An unconfirmed inverse stays queued after auth/transport failure,
        // but its card remains archived until the provider confirms restoration.
        for (const auto &card : mutation.restoreDeferred ? mutation.after : mutation.before)
            before.insert(card.value(QStringLiteral("id")).toString(), card);
        for (const auto &card : mutation.restoreDeferred ? mutation.before : mutation.after)
            after.insert(card.value(QStringLiteral("id")).toString(), card);
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
        QSet<QString> restore;
        for (const auto &card : mutation.restoreDeferred ? mutation.before : mutation.after) {
            const QString id = card.value(QStringLiteral("id")).toString();
            if (!before.contains(id) && std::none_of(cards.begin(), cards.end(), [&](const auto &entry) { return entry.value(QStringLiteral("id")).toString() == id; })) restore.insert(id);
        }
        restoreCardOrder(cards, mutation.restoreDeferred ? mutation.before : mutation.after, restore);
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
        }
    }
    const auto before = m_cards;
    injectSenderPhotos(cards);
    replaceCards(cards);
    persistCards(before, cards, {}, {}, requestId);
    // Download originals during synchronization, advancing on durable completion.
    emit mailSyncStateChanged();
    queueMailBodyPrefetch();
    // R67: same pacing for sender photos; only sender_has_photo cards without
    // a cached file fetch, and 404/403 parks until eligibility re-marks.
    queueSenderPhotoPrefetch();
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
QString CardStore::createReminder(const QString &title, const QString &dueAt, const QString &recurrence) {
    if (title.trimmed().isEmpty() || !QDateTime::fromString(dueAt, Qt::ISODate).isValid() ||
        (!recurrence.isEmpty() && recurrence != QStringLiteral("daily") && recurrence != QStringLiteral("weekly"))) return {};
    const QString id = QUuid::createUuid().toString(QUuid::WithoutBraces);
    return enqueueOp(id, QStringLiteral("create_reminder"), {{QStringLiteral("id"), id},
        {QStringLiteral("title"), title.trimmed()}, {QStringLiteral("due_at"), dueAt}, {QStringLiteral("recurrence"), recurrence}}).isEmpty() ? QString() : id;
}
QString CardStore::sendAgentMessage(const QString &threadId, const QString &text, const QString &clientMsgId) {
    if (QUuid(threadId).isNull() || text.trimmed().isEmpty() || text.toUtf8().size() > 4096) return {};
    const QString id = clientMsgId.isEmpty() ? QUuid::createUuid().toString(QUuid::WithoutBraces) : clientMsgId;
    if (QUuid(id).isNull()) return {};
    for (const auto &op : m_outbox) {
        if (op.value(QStringLiteral("op_id")).toString() != id) continue;
        const auto args = op.value(QStringLiteral("args")).toMap();
        if (op.value(QStringLiteral("type")) != QStringLiteral("agent_message") ||
            args.value(QStringLiteral("thread_id")) != threadId || args.value(QStringLiteral("text")) != text) return {};
        refresh();
        return id;
    }
    return enqueueOp(id, QStringLiteral("agent_message"), {{QStringLiteral("thread_id"), threadId},
        {QStringLiteral("client_msg_id"), id}, {QStringLiteral("text"), text}}).isEmpty() ? QString() : id;
}
QVariantList CardStore::pendingAgentMessages(const QString &threadId) const {
    QVariantList messages;
    for (const auto &op : m_outbox) {
        const auto args = op.value(QStringLiteral("args")).toMap();
        if (op.value(QStringLiteral("type")) != QStringLiteral("agent_message") || args.value(QStringLiteral("thread_id")) != threadId) continue;
        QVariantMap message = args;
        message.insert(QStringLiteral("id"), args.value(QStringLiteral("client_msg_id")));
        message.insert(QStringLiteral("direction"), QStringLiteral("out"));
        message.insert(QStringLiteral("seq"), 0);
        message.insert(QStringLiteral("delivery_state"), m_failedWrites.contains(op.value(QStringLiteral("op_id")).toString()) ? QStringLiteral("indeterminate") : QStringLiteral("pending"));
        messages.append(message);
    }
    return messages;
}
QString CardStore::enqueueOp(const QString &cardId, const QString &type, const QVariantMap &args) {
    if (!m_open || QUuid(cardId).isNull()) return {};
    static const QSet<QString> types = {QStringLiteral("delete_journal"), QStringLiteral("done"), QStringLiteral("archive"), QStringLiteral("spam"), QStringLiteral("note"), QStringLiteral("pin"), QStringLiteral("unpin"),
        QStringLiteral("reorder_pins"), QStringLiteral("reorder_cards"), QStringLiteral("create_card"), QStringLiteral("create_reminder"), QStringLiteral("agent_message"), QStringLiteral("snooze"), QStringLiteral("bundle_archive"), QStringLiteral("bundle_spam"), QStringLiteral("bundle_done"), QStringLiteral("take_out")};
    if (!types.contains(type)) return {};
    if (type == QStringLiteral("delete_journal") && std::none_of(m_cards.cbegin(), m_cards.cend(), [&](const auto &card) {
        return card.value(QStringLiteral("id")).toString() == cardId && card.value(QStringLiteral("source")).toString() == QStringLiteral("journal");
    })) return {};
    if (type == QStringLiteral("spam") || type == QStringLiteral("bundle_spam")) {
        for (const QVariantMap &op : m_outbox)
            if (op.value(QStringLiteral("card_id")).toString() == cardId &&
                op.value(QStringLiteral("type")).toString() == type &&
                op.value(QStringLiteral("args")).toMap() == args)
                return op.value(QStringLiteral("op_id")).toString();
    }
    const QString id = type == QStringLiteral("agent_message") ? cardId : QUuid::createUuid().toString(QUuid::WithoutBraces);
    const QVariantMap payload{{QStringLiteral("op_id"), id}, {QStringLiteral("card_id"), cardId},
        {QStringLiteral("type"), type}, {QStringLiteral("args"), args}};

    QList<QVariantMap> cards = m_cards;
    auto isPinned = [](const QVariantMap &card) {
        const QVariant rank = card.value(QStringLiteral("pinned_rank"));
        return card.contains(QStringLiteral("pinned_rank")) && rank.isValid() && !rank.isNull();
    };
    bool changed = false;
    if (type == QStringLiteral("done") || type == QStringLiteral("archive") || type == QStringLiteral("spam") || type == QStringLiteral("snooze") || type == QStringLiteral("delete_journal")) {
        const auto oldSize = cards.size();
        cards.erase(std::remove_if(cards.begin(), cards.end(), [&](const QVariantMap &card) {
            return card.value(QStringLiteral("id")).toString() == cardId;
        }), cards.end());
        changed = cards.size() != oldSize;
    } else if (type == QStringLiteral("bundle_archive") || type == QStringLiteral("bundle_spam") || type == QStringLiteral("bundle_done")) {
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
    } else if (type == QStringLiteral("create_reminder")) {
        cards.prepend({{QStringLiteral("id"), cardId}, {QStringLiteral("title"), args.value(QStringLiteral("title"))},
            {QStringLiteral("source"), QStringLiteral("reminder")}, {QStringLiteral("source_kind"), QStringLiteral("reminder")},
            {QStringLiteral("state"), QStringLiteral("open")}, {QStringLiteral("section"), QStringLiteral("later")},
            {QStringLiteral("at"), args.value(QStringLiteral("due_at"))}, {QStringLiteral("recurrence"), args.value(QStringLiteral("recurrence"))}});
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
    if (ownerWrite(payload)) {
        QVariantList authored;
        QVariantMap patches;
        for (const auto &card : std::as_const(cards)) {
            const auto old = std::find_if(before.cbegin(), before.cend(), [&](const auto &entry) { return entry.value(QStringLiteral("id")) == card.value(QStringLiteral("id")); });
            if (old != before.cend() && *old == card) continue;
            authored.append(card);
            QVariantMap patch;
            for (auto it = card.begin(); it != card.end(); ++it)
                if (old == before.cend() || old->value(it.key()) != it.value()) patch.insert(it.key(), it.value());
            patches.insert(card.value(QStringLiteral("id")).toString(), patch);
        }
        QVariantList removed;
        for (const auto &card : before)
            if (std::none_of(cards.cbegin(), cards.cend(), [&](const auto &row) { return row.value(QStringLiteral("id")) == card.value(QStringLiteral("id")); }))
                removed.append(card);
        const QVariantMap draft{{QStringLiteral("op_id"), id}, {QStringLiteral("op"), payload}, {QStringLiteral("cards"), authored},
            {QStringLiteral("patches"), patches}, {QStringLiteral("removed_cards"), removed}};
        if (!appendDraftRecord(draft)) return {};
        m_drafts.insert(id, draft);
    }
    m_outbox.append(payload);
    if (changed) {
        m_mutations.insert(id, {before, cards});
        replaceCards(cards);
    }
    if (changed && type == QStringLiteral("archive")) {
        m_bundleArchives.insert(id, {before, {{cardId, id}}, QDeadlineTimer(m_bundleUndoDurationMs), {}});
        m_bundleUndoTimer->start(std::max(1, bundleArchiveUndoRemainingMs()));
        emit bundleArchiveUndoChanged();
    }
    persistCards(before, cards, {payload});
    emit pendingOpsChanged();
    return id;
}
bool CardStore::bundleArchiveUndoActive() const {
    for (auto it = m_bundleArchives.cbegin(); it != m_bundleArchives.cend(); ++it)
        if (!it->deadline.hasExpired()) return true;
    return false;
}
int CardStore::bundleArchiveUndoRemainingMs() const {
    qint64 remaining = 0;
    for (auto it = m_bundleArchives.cbegin(); it != m_bundleArchives.cend(); ++it)
        if (!it->deadline.hasExpired()) remaining = std::max(remaining, it->deadline.remainingTime());
    return static_cast<int>(remaining);
}
void CardStore::setBundleArchiveUndoDurationForTest(int milliseconds) {
    m_bundleUndoDurationMs = std::max(1, milliseconds);
}
void CardStore::expireBundleArchives() {
    QStringList expired;
    for (auto it = m_bundleArchives.begin(); it != m_bundleArchives.end();) {
        if (it->deadline.hasExpired()) { expired.append(it.key()); it = m_bundleArchives.erase(it); }
        else ++it;
    }
    emit bundleArchiveUndoChanged();
    for (const QString &token : expired) emit bundleArchiveExpired(token);
    if (!m_bundleArchives.isEmpty())
        m_bundleUndoTimer->start(std::max(1, bundleArchiveUndoRemainingMs()));
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
    BundleArchive entry{before, {}, QDeadlineTimer(m_bundleUndoDurationMs), {}};
    QList<QVariantMap> ops;
    const QString token = QUuid::createUuid().toString(QUuid::WithoutBraces);
    for (const QVariantMap &card : before) {
        const QString cardId = card.value(QStringLiteral("id")).toString();
        if (!archived.contains(cardId)) continue;
        const QString opId = QUuid::createUuid().toString(QUuid::WithoutBraces);
        m_bundleFailureGroups.insert(opId, token);
        ops.append({{QStringLiteral("op_id"), opId}, {QStringLiteral("card_id"), cardId},
            {QStringLiteral("type"), QStringLiteral("archive")}, {QStringLiteral("args"), QVariantMap{}}});
        // Each member rolls back against the same complete original order,
        // while its delta still removes only that member.
        QList<QVariantMap> withoutCard = before;
        withoutCard.erase(std::remove_if(withoutCard.begin(), withoutCard.end(), [&](const QVariantMap &other) {
            return other.value(QStringLiteral("id")).toString() == cardId;
        }), withoutCard.end());
        m_mutations.insert(opId, {before, withoutCard});
        entry.ops.append({cardId, opId});
    }
    m_outbox.append(ops);
    replaceCards(after);
    persistCards(before, after, ops);
    emit pendingOpsChanged();
    m_bundleArchives.insert(token, std::move(entry));
    m_bundleUndoTimer->start(std::max(1, bundleArchiveUndoRemainingMs()));
    emit bundleArchiveUndoChanged();
    return {{QStringLiteral("token"), token}, {QStringLiteral("count"), archived.size()}};
}
void CardStore::undoQueuedOp(const QString &opId) {
    const auto owner = std::find_if(m_outbox.cbegin(), m_outbox.cend(), [&](const auto &op) { return op.value(QStringLiteral("op_id")).toString() == opId; });
    if (owner != m_outbox.cend() && ownerWrite(*owner)) { qWarning() << "Refusing cancellation of owner op" << opId; return; }
    if (!m_open || m_inFlight.contains(opId) || !m_mutations.contains(opId)) return;
    const auto queued = std::any_of(m_outbox.cbegin(), m_outbox.cend(),
        [&](const QVariantMap &op) { return op.value(QStringLiteral("op_id")).toString() == opId; });
    if (!queued) return;
    m_outbox.erase(std::remove_if(m_outbox.begin(), m_outbox.end(),
        [&](const QVariantMap &op) { return op.value(QStringLiteral("op_id")).toString() == opId; }), m_outbox.end());
    m_durableOps.remove(opId);
    persistCards(m_cards, m_cards, {}, {opId});
    undoOp(opId);
    emit pendingOpsChanged();
}
bool CardStore::undoBundleArchive(const QString &token) {
    if (!m_open || !m_bundleArchives.contains(token)) return false;
    const BundleArchive entry = m_bundleArchives.take(token);
    if (entry.deadline.hasExpired()) return false;
    emit bundleArchiveUndoChanged();
    if (!m_bundleArchives.isEmpty()) m_bundleUndoTimer->start(std::max(1, bundleArchiveUndoRemainingMs()));
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
        const bool queued = !entry.sent.contains(opId) && std::any_of(m_outbox.begin(), m_outbox.end(),
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
            m_archivedAwaitingSnapshot.remove(cardId);
        }
    }
    if (restore.isEmpty()) return false;
    QList<QVariantMap> cards = m_cards;
    restoreCardOrder(cards, entry.before, restore);
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
    m_outbox.append(reverseOps);
    replaceCards(cards);
    persistCards(current, cards, reverseOps, cancelled);
    emit pendingOpsChanged();
    return true;
}
void CardStore::flush() {
    if (!m_online || !m_open) return;
    // Fill the bounded pipeline; every slot frees on its own ACK or failure.
    // Per-op deferral (m_deferredWrites) parks one card, never the whole queue.
    bool progressed = true;
    while (progressed && (int)m_inFlight.size() < m_maxInFlight)
        progressed = flushNext();
    if (m_outbox.isEmpty() && m_inFlight.isEmpty()) emit flushFinished();
}
bool CardStore::flushNext() {
    if (!m_online || !m_rateCooldown.hasExpired()) return false;
    if (m_outbox.isEmpty()) return false;
    // Preserve per-card order across the pipeline: a card with an op already
    // on the wire or parked in backoff blocks only its own later ops.
    QSet<QString> flyingCards;
    bool reorderFlying = false;
    for (const auto &id : std::as_const(m_inFlight)) {
        const auto sent = std::find_if(m_outbox.cbegin(), m_outbox.cend(), [&](const auto &op) {
            return op.value(QStringLiteral("op_id")).toString() == id;
        });
        if (sent == m_outbox.cend()) continue;
        flyingCards.insert(sent->value(QStringLiteral("card_id")).toString());
        const QString flyingType = sent->value(QStringLiteral("type")).toString();
        if (flyingType == QStringLiteral("reorder_cards") || flyingType == QStringLiteral("reorder_pins"))
            reorderFlying = true;
    }
    QSet<QString> blockedCards;
    const auto next = std::find_if(m_outbox.cbegin(), m_outbox.cend(), [&](const QVariantMap &op) {
        const QString opId = op.value(QStringLiteral("op_id")).toString();
        if (m_inFlight.contains(opId) || m_deferredWrites.contains(opId)) {
            blockedCards.insert(op.value(QStringLiteral("card_id")).toString());
            return false;
        }
        if (flyingCards.contains(op.value(QStringLiteral("card_id")).toString())) return false;
        if (reorderFlying) return false;
        const QString type = op.value(QStringLiteral("type")).toString();
        if ((type == QStringLiteral("reorder_cards") || type == QStringLiteral("reorder_pins")) && !m_inFlight.isEmpty())
            return false; // Reorder ops fly alone against a coherent open set.
        return !blockedCards.contains(op.value(QStringLiteral("card_id")).toString());
    });
    if (next == m_outbox.cend()) return false;
    const QVariantMap payload = *next;
    const QString id = payload.value(QStringLiteral("op_id")).toString();
    if (!m_durableOps.contains(id)) return false;
    m_inFlight.insert(id);
    for (auto it = m_bundleArchives.begin(); it != m_bundleArchives.end(); ++it)
        for (const auto &[cardId, opId] : it->ops)
            if (opId == id) it->sent.insert(id);
    if (m_transport) {
        QPointer<CardStore> self(this);
        m_transport->postOp(QStringLiteral("/v1/ops"), payload, [self,id](int status, const QVariantMap &response) {
            if (self) self->reportPostResult(id, status, response);
        });
    } else {
        emit requestPost(id, QStringLiteral("/v1/ops"), payload);
    }
    return true;
}
void CardStore::dropServerDeletedCard(const QString &cardId) {
    if (cardId.isEmpty()) return;
    // A remaining op still writing this card keeps the row; the next snapshot
    // or the op's own retirement settles it. Reorder and agent-message ops
    // carry a nominal card id, never a row subject.
    for (const auto &op : std::as_const(m_outbox)) {
        if (op.value(QStringLiteral("card_id")).toString() != cardId) continue;
        const QString type = op.value(QStringLiteral("type")).toString();
        if (type == QStringLiteral("reorder_cards") || type == QStringLiteral("reorder_pins") ||
            type == QStringLiteral("agent_message")) continue;
        return;
    }
    auto matches = [&](const QVariantMap &card) { return card.value(QStringLiteral("id")).toString() == cardId; };
    if (std::none_of(m_cards.cbegin(), m_cards.cend(), matches)) return;
    const auto before = m_cards;
    QList<QVariantMap> after = m_cards;
    after.erase(std::remove_if(after.begin(), after.end(), matches), after.end());
    replaceCards(after);
    persistCards(before, after);
}
void CardStore::retireRejectedOperation(const QString &opId, const QVariantMap &replacement, const QString &retiredCardId) {
    // Retirement is not an ACK; original journal bytes remain available.
    const QString replacementId = replacement.value(QStringLiteral("op_id")).toString();
    const QVariantMap draft{{QStringLiteral("op_id"), replacementId}, {QStringLiteral("op"), replacement}};
    QVariantMap record{{QStringLiteral("op_id"), opId}, {QStringLiteral("retired"), true}};
    if (!replacement.isEmpty()) record.insert(QStringLiteral("replacement"), draft);
    // One durable append binds retirement and replacement across process death.
    if (!appendDraftRecord(record)) { m_inFlight.remove(opId); return; }
    if (!replacement.isEmpty()) m_drafts.insert(replacementId, draft);
    m_drafts.remove(opId);
    QMetaObject::invokeMethod(m_storage, [this, opId, replacement, replacementId, retiredCardId] {
        bool ok = m_db.transaction();
        QSqlQuery q(m_db);
        q.prepare(QStringLiteral("INSERT OR IGNORE INTO retired_ops VALUES(?)"));
        q.addBindValue(opId);
        if (ok) ok = q.exec();
        q.prepare(QStringLiteral("DELETE FROM outbox WHERE op_id=?"));
        q.addBindValue(opId);
        if (ok) ok = q.exec();
        if (ok && !replacement.isEmpty()) {
            q.prepare(QStringLiteral("INSERT INTO outbox(op_id,payload,owner_write) VALUES(?,?,1)"));
            q.addBindValue(replacementId);
            q.addBindValue(QString::fromUtf8(QJsonDocument(QJsonObject::fromVariantMap(replacement)).toJson(QJsonDocument::Compact)));
            ok = q.exec();
        }
        if (ok) ok = m_db.commit();
        if (!ok) m_db.rollback();
        const QString error = q.lastError().text();
        QMetaObject::invokeMethod(this, [this, opId, replacement, replacementId, retiredCardId, ok, error] {
            m_inFlight.remove(opId);
            if (!ok) { emit storageError(error); return; }
            m_outbox.erase(std::remove_if(m_outbox.begin(), m_outbox.end(), [&](const auto &op) {
                return op.value(QStringLiteral("op_id")).toString() == opId;
            }), m_outbox.end());
            m_mutations.remove(opId);
            m_failedWrites.remove(opId);
            m_deferredWrites.remove(opId);
            m_durableOps.remove(opId);
            if (!replacement.isEmpty()) { m_outbox.append(replacement); m_durableOps.insert(replacementId); }
            else dropServerDeletedCard(retiredCardId);
            emit pendingOpsChanged();
            refresh();
            flush();
        }, Qt::QueuedConnection);
    }, Qt::QueuedConnection);
}
void CardStore::reportPostResult(const QString &opId, int status, const QVariantMap &response) {
    if (!m_inFlight.contains(opId)) return;
    const bool acknowledged = status >= 200 && status < 300 && response.value(QStringLiteral("ok")) == QVariant(true);
    const bool semanticRejection = status == 409 || status == 422;
    const auto queued = std::find_if(m_outbox.cbegin(), m_outbox.cend(), [&](const QVariantMap &op) {
        return op.value(QStringLiteral("op_id")).toString() == opId;
    });
    if (queued == m_outbox.cend()) return;
    const QVariantMap operation = *queued;
    const QString type = operation.value(QStringLiteral("type")).toString();
    const QString error = response.value(QStringLiteral("error")).toString();
    const bool missing = status == 404 || (status == 422 && error == QStringLiteral("card not found"));
    const bool staleOrder = type == QStringLiteral("reorder_cards") &&
        (missing || (status == 422 && error == QStringLiteral("card order must include every open card")));
    if (staleOrder) {
        m_rejectedReorder = opId;
        m_rejectedReorderSnapshot = ++m_cardsRequestSequence;
        emit requestCards(QStringLiteral("/v1/cards"), m_rejectedReorderSnapshot);
        return;
    }
    if (missing) {
        // The server confirms the card is gone. Retire the op, then drop the
        // local row unless a remaining op still writes it. Reorder and
        // agent-message ids name no row; the next snapshot settles the rest.
        static const QSet<QString> rowSubjects{QStringLiteral("done"), QStringLiteral("archive"),
            QStringLiteral("spam"), QStringLiteral("snooze"), QStringLiteral("delete_journal"),
            QStringLiteral("note"), QStringLiteral("pin"), QStringLiteral("unpin"),
            QStringLiteral("create_card"), QStringLiteral("create_reminder"),
            QStringLiteral("gmail.label_add")};
        const QString subject = rowSubjects.contains(type) ? operation.value(QStringLiteral("card_id")).toString() : QString();
        retireRejectedOperation(opId, {}, subject);
        return;
    }
    // HTTP 4xx is permanent only for reversible actions, never for a write.
    // A server permission/schema failure must not erase the owner's content.
    const bool ownerWrite = CardStore::ownerWrite(operation);
    const bool clientRejection = status >= 400 && status < 500 && status != 401 && status != 403 && status != 429;
    const bool rejected = clientRejection && !ownerWrite;
    const bool terminal = acknowledged || rejected;
    const QString failureGroup = terminal ? m_bundleFailureGroups.take(opId) : QString();
    const bool notifyFailure = failureGroup.isEmpty() || !m_reportedBundleFailures.contains(failureGroup);
    if (!failureGroup.isEmpty() && !acknowledged) m_reportedBundleFailures.insert(failureGroup);
    if (!failureGroup.isEmpty() && !m_bundleFailureGroups.values().contains(failureGroup))
        m_reportedBundleFailures.remove(failureGroup);
    if (!acknowledged) {
        if (status == 422)
            qWarning().noquote() << "Operation rejected:" << opId << status << QJsonDocument(QJsonObject::fromVariantMap(response)).toJson(QJsonDocument::Compact);
        if (ownerWrite) {
            m_failedWrites.insert(opId);
            m_deferredWrites.insert(opId);
            m_inFlight.remove(opId);
            emit pendingOpsChanged();
            const int delay = status == 429 ? kRateLimitRetryMs : qMin(60000, 1000 * (1 << qMin(m_retryAttempt, 6)));
            m_retryAttempt = qMin(m_retryAttempt + 1, 6);
            m_retryTimer.start(delay);
            flush(); // Unrelated queued actions must still progress.
            return;
        }
        if (rejected) {
            if (semanticRejection && operation.value(QStringLiteral("type")).toString() == QStringLiteral("archive")) {
                const QString cardId = operation.value(QStringLiteral("card_id")).toString();
                for (const auto &pending : m_outbox) {
                    if (pending.value(QStringLiteral("card_id")).toString() != cardId ||
                        pending.value(QStringLiteral("type")).toString() != QStringLiteral("gmail.label_add") ||
                        pending.value(QStringLiteral("args")).toMap().value(QStringLiteral("label")).toString() != QStringLiteral("INBOX")) continue;
                    auto inverse = m_mutations.find(pending.value(QStringLiteral("op_id")).toString());
                    if (inverse != m_mutations.end()) inverse->archiveRejected = true;
                }
            }
            m_retryAttempt = 0;
            const bool canUndoLocally = m_mutations.contains(opId) && !m_mutations.value(opId).archiveRejected;
            QMetaObject::invokeMethod(m_storage, [this, opId, canUndoLocally, operation, notifyFailure, status, response] {
                QSqlQuery q(m_db);
                q.prepare(QStringLiteral("DELETE FROM outbox WHERE op_id=?")); q.addBindValue(opId);
                const bool ok = q.exec();
                const QString error = q.lastError().text();
                QList<QVariantMap> retained;
                const QString type = operation.value(QStringLiteral("type")).toString();
                const bool removal = type == QStringLiteral("snooze") || type == QStringLiteral("archive") || type == QStringLiteral("spam");
                if (ok && removal && q.exec(QStringLiteral("SELECT payload FROM cards ORDER BY position")))
                    while (q.next()) retained.append(QJsonDocument::fromJson(q.value(0).toByteArray()).object().toVariantMap());
                QMetaObject::invokeMethod(this, [this, opId, canUndoLocally, ok, error, retained, operation, notifyFailure, status, response] {
                    if (!ok) {
                        m_inFlight.remove(opId);
                        m_deferredWrites.insert(opId);
                        emit storageError(error);
                        const int delay = qMin(60000, 1000 * (1 << qMin(m_retryAttempt, 6)));
                        m_retryAttempt = qMin(m_retryAttempt + 1, 6);
                        m_retryTimer.start(delay);
                        flush();
                        return;
                    }
                    m_inFlight.remove(opId);
                    m_outbox.erase(std::remove_if(m_outbox.begin(), m_outbox.end(), [&](const QVariantMap &op) {
                        return op.value(QStringLiteral("op_id")).toString() == opId;
                    }), m_outbox.end());
                    undoOp(opId);
                    // A restart has no Mutation; the retained durable row still
                    // carries the latest snapshot plus unacknowledged owner edits.
                    auto cards = m_cards;
                    const QString id = operation.value(QStringLiteral("card_id")).toString();
                    if (std::none_of(cards.cbegin(), cards.cend(), [&](const auto &row) { return row.value(QStringLiteral("id")).toString() == id; }))
                        restoreCardOrder(cards, retained, {id});
                    replaceCards(cards);
                    m_durableOps.remove(opId);
                    m_mutations.remove(opId);
                    if (notifyFailure) emit operationFailed(opId, status, response);
                    emit pendingOpsChanged();
                    if (!canUndoLocally) refresh();
                    flush();
                }, Qt::QueuedConnection);
            }, Qt::QueuedConnection);
            return;
        }
        const QString cardId = operation.value(QStringLiteral("card_id")).toString();
        for (const auto &pending : m_outbox) {
            if (pending.value(QStringLiteral("card_id")).toString() != cardId ||
                pending.value(QStringLiteral("type")).toString() != QStringLiteral("gmail.label_add") ||
                pending.value(QStringLiteral("args")).toMap().value(QStringLiteral("label")).toString() != QStringLiteral("INBOX")) continue;
            const QString inverseId = pending.value(QStringLiteral("op_id")).toString();
            const auto inverse = m_mutations.constFind(inverseId);
            if (inverse == m_mutations.cend() || inverse->archiveRejected || inverse->restoreDeferred) continue;
            Mutation deferred = *inverse;
            undoOp(inverseId);
            deferred.restoreDeferred = true;
            m_mutations.insert(inverseId, deferred);
        }
        // Park only the failed card in backoff; the rest of the pipeline keeps
        // draining. A 429 additionally pauses the pump for one bounded slice:
        // the server's per-device token bucket refills continuously, so a
        // second is enough for a fresh batch of requests; retain the same op.
        m_inFlight.remove(opId);
        m_deferredWrites.insert(opId);
        int delay = qMin(60000, 1000 * (1 << qMin(m_retryAttempt, 6)));
        if (status == 429) {
            delay = kRateLimitRetryMs;
            m_rateCooldown = QDeadlineTimer(delay, Qt::PreciseTimer);
        }
        m_retryAttempt = qMin(m_retryAttempt + 1, 6);
        m_retryTimer.start(delay);
        flush();
        return;
    }
    // The durable ACK is group-committed (commitAcks): every ACK that lands
    // while a commit is running shares the next journal fsync and the next
    // SQLite transaction. The op keeps its in-flight slot until that commit.
    QStringList removedIds;
    if (type == QStringLiteral("done") || type == QStringLiteral("archive") || type == QStringLiteral("spam") || type == QStringLiteral("snooze") || type == QStringLiteral("delete_journal"))
        removedIds.append(operation.value(QStringLiteral("card_id")).toString());
    for (const auto &value : m_drafts.value(opId).value(QStringLiteral("removed_cards")).toList())
        removedIds.append(value.toMap().value(QStringLiteral("id")).toString());
    removedIds.erase(std::remove_if(removedIds.begin(), removedIds.end(), [&](const auto &id) {
        return std::any_of(m_cards.cbegin(), m_cards.cend(), [&](const auto &card) { return card.value(QStringLiteral("id")).toString() == id; });
    }), removedIds.end());
    m_ackQueue.append({opId, response, operation, removedIds, ownerWrite});
    if (!m_ackCommitting && !m_ackScheduled) {
        m_ackScheduled = true; // Queued: ACKs delivered in the same event-loop pass coalesce.
        QMetaObject::invokeMethod(this, &CardStore::commitAcks, Qt::QueuedConnection);
    }
}
void CardStore::commitAcks() {
    m_ackScheduled = false;
    if (m_ackCommitting || m_ackQueue.isEmpty() || !m_open) return;
    QList<PendingAck> batch;
    batch.swap(m_ackQueue);
    QList<QVariantMap> records;
    for (const auto &ack : std::as_const(batch))
        if (ack.ownerWrite) records.append({{QStringLiteral("op_id"), ack.opId}, {QStringLiteral("acked"), true}});
    if (!records.isEmpty() && !appendDraftRecords(records)) {
        for (const auto &ack : std::as_const(batch)) m_inFlight.remove(ack.opId);
        return; // Replay the same ops; never prune on a failed durable ACK record.
    }
    for (const auto &ack : std::as_const(batch)) m_drafts.remove(ack.opId);
    m_ackCommitting = true;
    QMetaObject::invokeMethod(m_storage, [this, batch] {
        bool ok = m_db.transaction();
        QSqlQuery q(m_db);
        QString failure;
        for (const auto &ack : batch) {
            // Same statements, same order as the former per-op commit; one fsync.
            q.prepare(QStringLiteral("INSERT OR IGNORE INTO acknowledged_ops(op_id) VALUES(?)"));
            q.addBindValue(ack.opId);
            if (ok) ok = q.exec();
            q.prepare(QStringLiteral("DELETE FROM outbox WHERE op_id=?")); q.addBindValue(ack.opId);
            if (ok) ok = q.exec();
            q.prepare(QStringLiteral("DELETE FROM acknowledged_ops WHERE op_id=?")); q.addBindValue(ack.opId);
            if (ok) ok = q.exec();
            // Delete only acknowledged removals, through the normal owner guard.
            // In particular a pending note/create must never lose its durable row.
            for (const auto &id : ack.removedIds) {
                q.prepare(QStringLiteral("DELETE FROM cards WHERE id=? AND NOT EXISTS(SELECT 1 FROM outbox WHERE owner_write=1 AND json_extract(payload,'$.card_id')=?)"));
                q.addBindValue(id);
                q.addBindValue(id);
                if (ok) ok = q.exec();
            }
            if (!ok && failure.isEmpty()) failure = q.lastError().text();
        }
        if (ok) ok = m_db.commit();
        if (!ok) m_db.rollback();
        const QString error = failure.isEmpty() ? q.lastError().text() : failure;
        QMetaObject::invokeMethod(this, [this, batch, ok, error] {
            m_ackCommitting = false;
            for (const auto &ack : batch) m_inFlight.remove(ack.opId);
            if (!ok) {
                for (const auto &ack : batch) m_deferredWrites.insert(ack.opId);
                emit storageError(error);
                const int delay = qMin(60000, 1000 * (1 << qMin(m_retryAttempt, 6)));
                m_retryAttempt = qMin(m_retryAttempt + 1, 6);
                m_retryTimer.start(delay);
                flush();
                if (!m_ackQueue.isEmpty() && !m_ackScheduled) commitAcks();
                return;
            }
            m_retryAttempt = 0;
            for (const auto &ack : batch) {
                const QString &opId = ack.opId;
                const auto acknowledgedOp = std::find_if(m_outbox.cbegin(), m_outbox.cend(), [&](const QVariantMap &op) {
                    return op.value(QStringLiteral("op_id")).toString() == opId;
                });
                for (const auto &id : ack.removedIds) {
                    // Undo may already own this card while the removal ACK was pending.
                    if (std::none_of(m_cards.cbegin(), m_cards.cend(), [&](const QVariantMap &card) { return card.value(QStringLiteral("id")).toString() == id; }))
                        m_archivedAwaitingSnapshot.insert(id);
                }
                const bool restored = acknowledgedOp != m_outbox.cend() &&
                    acknowledgedOp->value(QStringLiteral("type")).toString() == QStringLiteral("gmail.label_add") &&
                    acknowledgedOp->value(QStringLiteral("args")).toMap().value(QStringLiteral("label")).toString() == QStringLiteral("INBOX") &&
                    m_mutations.contains(opId);
                if (restored) {
                    m_archivedAwaitingSnapshot.remove(acknowledgedOp->value(QStringLiteral("card_id")).toString());
                    m_restoredAwaitingSnapshot.insert(opId, m_cardsRequestSequence);
                }
                m_outbox.erase(std::remove_if(m_outbox.begin(), m_outbox.end(), [&](const QVariantMap &op) {
                    return op.value(QStringLiteral("op_id")).toString() == opId;
                }), m_outbox.end());
                m_failedWrites.remove(opId);
                m_deferredWrites.remove(opId);
                m_durableOps.remove(opId);
                if (!restored) m_mutations.remove(opId);
                emit operationAcknowledged(ack.operation, ack.response);
                emit pendingOpsChanged();
                if (restored) refresh();
            }
            flush();
            if (!m_ackQueue.isEmpty() && !m_ackScheduled) commitAcks();
        }, Qt::QueuedConnection);
    }, Qt::QueuedConnection);
}
QVariantMap CardStore::cachedMailBody(const QString &cardId) const {
    if (!m_bodyCached.contains(cardId)) return {};
    const auto hit = m_bodyLru.constFind(cardId);
    if (hit != m_bodyLru.cend()) {
        m_bodyLruOrder.removeOne(cardId);
        m_bodyLruOrder.append(cardId);
        return *hit;
    }
    const QVariantMap body = readBodyFromDb(cardId);
    if (!body.isEmpty()) lruInsert(cardId, body);
    return body;
}
QVariantMap CardStore::readBodyFromDb(const QString &cardId) const {
    QVariantMap body;
    // Same storage thread and FIFO queue as the writers, so a read always
    // observes every body write queued before it.
    QMetaObject::invokeMethod(m_storage, [&] {
        QSqlQuery q(m_db);
        q.prepare(QStringLiteral("SELECT html,source_url,messages FROM mail_bodies WHERE card_id=?"));
        q.addBindValue(cardId);
        if (!q.exec() || !q.next()) return;
        const QJsonDocument messages = QJsonDocument::fromJson(q.value(2).toByteArray());
        body = {{QStringLiteral("html"), q.value(0)}, {QStringLiteral("source_url"), q.value(1)},
                {QStringLiteral("messages"), messages.isArray() ? messages.array().toVariantList() : QVariantList{}}};
    }, Qt::BlockingQueuedConnection);
    return body;
}
static qint64 bodyBytes(const QVariantMap &body) {
    qint64 bytes = 2 * (body.value(QStringLiteral("html")).toString().size() + body.value(QStringLiteral("source_url")).toString().size());
    for (const QVariant &m : body.value(QStringLiteral("messages")).toList())
        bytes += 2 * m.toMap().value(QStringLiteral("html")).toString().size();
    return bytes;
}
void CardStore::lruRemove(const QString &cardId) const {
    const auto it = m_bodyLru.find(cardId);
    if (it == m_bodyLru.end()) return;
    m_bodyLruBytes -= bodyBytes(*it);
    m_bodyLru.erase(it);
    m_bodyLruOrder.removeOne(cardId);
}
void CardStore::lruInsert(const QString &cardId, const QVariantMap &body) const {
    lruRemove(cardId);
    m_bodyLru.insert(cardId, body);
    m_bodyLruOrder.append(cardId);
    m_bodyLruBytes += bodyBytes(body);
    // Always keep the newest body, even when it alone exceeds the cap.
    while (m_bodyLruBytes > kBodyLruBytes && m_bodyLruOrder.size() > 1) lruRemove(m_bodyLruOrder.first());
}
int CardStore::mailSyncPending() const {
    int pending = 0;
    for (const QVariantMap &card : m_cards) {
        if (card.value(QStringLiteral("source")).toString() != QStringLiteral("mail") &&
            !card.value(QStringLiteral("has_body")).toBool()) continue;
        const QString id = card.value(QStringLiteral("id")).toString();
        if (!m_bodyCached.contains(id) || m_bodyPersisting.contains(id)) ++pending;
    }
    return pending;
}
bool CardStore::mailSyncComplete() const {
    return m_latestCardsResponse != 0 && m_durableBodySnapshot == m_latestCardsResponse && mailSyncPending() == 0;
}
void CardStore::queueMailBodyPrefetch() {
    if (!m_online || !m_open) return;
    QSet<QString> eligible;
    for (const QVariantMap &card : m_cards) {
        if (card.value(QStringLiteral("source")).toString() != QStringLiteral("mail") &&
            !card.value(QStringLiteral("has_body")).toBool()) continue;
        const QString id = card.value(QStringLiteral("id")).toString();
        if (m_bodyCached.contains(id) || m_bodyRequests.contains(id) || m_bodyPersisting.contains(id)) continue;
        eligible.insert(id);
    }
    m_bodyPrefetchQueue.erase(std::remove_if(m_bodyPrefetchQueue.begin(), m_bodyPrefetchQueue.end(),
        [&](const QString &id) { return !eligible.contains(id); }), m_bodyPrefetchQueue.end());
    QSet<QString> queued(m_bodyPrefetchQueue.begin(), m_bodyPrefetchQueue.end());
    for (const QVariantMap &card : m_cards) {
        const QString id = card.value(QStringLiteral("id")).toString();
        if (eligible.contains(id) && !queued.contains(id)) {
            m_bodyPrefetchQueue.append(id);
            queued.insert(id);
        }
    }
    prefetchNextMailBody();
}
void CardStore::prefetchNextMailBody() {
    if (!m_online || !m_open) return;
    m_bodyPrefetchTimer.stop();
    if (!m_bodyBackoff.hasExpired()) {
        m_bodyPrefetchTimer.start(int(m_bodyBackoff.remainingTime()));
        return;
    }
    // Two outstanding downloads bound memory/server work; a completed durable
    // write releases a slot immediately, not after an arbitrary per-body sleep.
    qint64 retryIn = 60000;
    for (int index = 0; index < m_bodyPrefetchQueue.size() && m_bodyRequests.size() + m_bodyPersisting.size() < 2;) {
        const QString id = m_bodyPrefetchQueue.at(index);
        const auto retry = m_bodyRetryAfter.constFind(id);
        if (retry != m_bodyRetryAfter.cend() && !retry->hasExpired()) {
            retryIn = qMin(retryIn, retry->remainingTime());
            ++index;
            continue;
        }
        m_bodyPrefetchQueue.removeAt(index);
        m_bodyRequests.insert(id);
        emit requestMailBodyGet(id, QStringLiteral("/v1/cards/") + id + QStringLiteral("/body"));
    }
    if (!m_bodyPrefetchQueue.isEmpty() && m_bodyRequests.size() + m_bodyPersisting.size() < 2)
        m_bodyPrefetchTimer.start(int(retryIn));
}
void CardStore::requestMailBody(const QString &cardId) {
    if (!m_open || cardId.isEmpty()) return;
    if (!cachedMailBody(cardId).isEmpty()) {
        emit mailBodyChanged(cardId);
        return;
    }
    if (!m_online) return;
    // Mail cards keep their existing fetchable body; non-mail cards opt in
    // via has_body from /v1/cards. Cards without either have no body route.
    // A reader opened on an id the open-list snapshot no longer contains
    // (archived elsewhere, stale bundle row) must still stop loading: with
    // no cached body and no fetch possible, report the failure so the reader
    // shows its error text instead of a blank pane stuck on loading.
    const QVariantMap *card = nullptr;
    for (const QVariantMap &c : m_cards) {
        if (c.value(QStringLiteral("id")).toString() == cardId) { card = &c; break; }
    }
    if (!card) { emit mailBodyFailed(cardId); return; }
    const bool isMail = card->value(QStringLiteral("source")).toString() == QStringLiteral("mail");
    if (!isMail && !card->value(QStringLiteral("has_body")).toBool()) { emit mailBodyFailed(cardId); return; }
    if (m_bodyRequests.contains(cardId) || m_bodyPersisting.contains(cardId)) return;
    m_bodyPrefetchQueue.removeAll(cardId);
    m_bodyPrefetchQueue.prepend(cardId);
    // An explicit reader retry can bypass this card's ordinary failure delay,
    // but never the shared 429 cooldown or the bounded outstanding request set.
    m_bodyRetryAfter.remove(cardId);
    prefetchNextMailBody();
}
void CardStore::applyRemoteMailBody(const QString &cardId, const QVariantMap &body) {
    if (!m_open || cardId.isEmpty()) return;
    m_bodyRequests.remove(cardId);
    m_bodyPrefetchQueue.removeAll(cardId);
    const QString html = body.value(QStringLiteral("html")).toString();
    QString sourceUrl = body.value(QStringLiteral("source_url")).toString();
    const bool hasMessages = body.contains(QStringLiteral("messages"));
    // Earlier-message originals survive a body response that omits them: read
    // them from SQLite only in that case, never to hold them resident.
    const QVariantList messages = hasMessages ? body.value(QStringLiteral("messages")).toList()
        : cachedMailBody(cardId).value(QStringLiteral("messages")).toList();
    const QString messagesJson = QString::fromUtf8(QJsonDocument(QJsonArray::fromVariantList(messages)).toJson(QJsonDocument::Compact));
    // Body responses omit empty source_url; SQLite requires a non-null string.
    if (sourceUrl.isNull()) sourceUrl = QStringLiteral("");
    // A malformed/error response must not replace a usable cached body.
    if (html.trimmed().isEmpty()) { reportMailBodyFailed(cardId, 422); return; }
    const bool hadBody = m_bodyCached.contains(cardId);
    m_bodyPersisting.insert(cardId);
    m_bodyCached.insert(cardId);
    lruRemove(cardId);
    emit mailSyncStateChanged();
    QMetaObject::invokeMethod(m_storage, [this, cardId, html, sourceUrl, messagesJson, hadBody] {
        QSqlQuery q(m_db);
        q.prepare(QStringLiteral("INSERT INTO mail_bodies(card_id,html,source_url,messages) VALUES(?,?,?,?) "
                                 "ON CONFLICT(card_id) DO UPDATE SET html=excluded.html, source_url=excluded.source_url, messages=excluded.messages"));
        q.addBindValue(cardId); q.addBindValue(html); q.addBindValue(sourceUrl);
        q.addBindValue(messagesJson);
        const bool ok = q.exec();
        const QString error = q.lastError().text();
        QMetaObject::invokeMethod(this, [this, cardId, hadBody, ok, error] {
            m_bodyPersisting.remove(cardId);
            if (!ok) {
                if (!hadBody) m_bodyCached.remove(cardId);
                lruRemove(cardId);
                emit storageError(error);
                reportMailBodyFailed(cardId);
                return;
            }
            m_bodyRetryAfter.remove(cardId);
            m_bodyRetryAttempts.remove(cardId);
            emit mailBodyChanged(cardId);
            emit mailSyncStateChanged();
            queueMailBodyPrefetch();
        }, Qt::QueuedConnection);
    }, Qt::QueuedConnection);
}
void CardStore::reportMailBodyFailed(const QString &cardId, int status) {
    m_bodyRequests.remove(cardId);
    const int attempt = m_bodyRetryAttempts.value(cardId);
    const int delay = status == 429 ? 60000 : qMin(60000, 1000 * (1 << qMin(attempt, 6)));
    m_bodyRetryAttempts.insert(cardId, qMin(attempt + 1, 6));
    m_bodyRetryAfter.insert(cardId, QDeadlineTimer(delay, Qt::PreciseTimer));
    // The server shares a 60/min device window across routes and supplies no
    // Retry-After. Park the entire body queue on 429, not just the rejected card.
    if (status == 429) m_bodyBackoff = QDeadlineTimer(delay, Qt::PreciseTimer);
    emit mailBodyFailed(cardId);
    emit mailSyncStateChanged();
    queueMailBodyPrefetch();
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
void CardStore::injectSenderPhotos(QList<QVariantMap> &cards) {
    // Device-local file URLs render without extra lookups. Only cards the
    // server flags sender_has_photo keep a file URL; everything else shows
    // initials, so a stale cache never renders after the flag flips false.
    for (QVariantMap &card : cards) {
        const QString id = card.value(QStringLiteral("id")).toString();
        const QString fileUrl = m_senderPhotos.value(id);
        if (!fileUrl.isEmpty() && card.value(QStringLiteral("sender_has_photo")).toBool())
            card.insert(QStringLiteral("sender_photo"), fileUrl);
        else
            card.remove(QStringLiteral("sender_photo"));
    }
}
void CardStore::queueSenderPhotoPrefetch() {
    if (!m_online || !m_open) return;
    for (auto it = m_photoRequests.begin(); it != m_photoRequests.end();) {
        if (it.value().hasExpired()) it = m_photoRequests.erase(it);
        else ++it;
    }
    QSet<QString> eligible;
    for (const QVariantMap &card : m_cards) {
        if (!card.value(QStringLiteral("sender_has_photo")).toBool()) continue;
        const QString id = card.value(QStringLiteral("id")).toString();
        if (id.isEmpty() || !m_senderPhotos.value(id).isEmpty() || m_photoRequests.contains(id)) continue;
        if (m_photoUnavailable.contains(id)) continue;
        eligible.insert(id);
    }
    m_photoPrefetchQueue.erase(std::remove_if(m_photoPrefetchQueue.begin(), m_photoPrefetchQueue.end(),
        [&](const QString &id) { return !eligible.contains(id); }), m_photoPrefetchQueue.end());
    QSet<QString> queued(m_photoPrefetchQueue.begin(), m_photoPrefetchQueue.end());
    for (const QVariantMap &card : m_cards) {
        const QString id = card.value(QStringLiteral("id")).toString();
        if (eligible.contains(id) && !queued.contains(id)) {
            m_photoPrefetchQueue.append(id);
            queued.insert(id);
        }
    }
    // Drop negative marks for cards that left the eligible set so a later
    // grant or re-appearing photo retries instead of sticking forever.
    for (auto it = m_photoUnavailable.begin(); it != m_photoUnavailable.end();) {
        if (!eligible.contains(*it) && !m_senderPhotos.contains(*it)) it = m_photoUnavailable.erase(it);
        else ++it;
    }
    if (m_photoPrefetchTimer.isActive()) return;
    if (m_photoPrefetchQueue.isEmpty() && m_photoRequests.isEmpty()) return;
    if (!m_nextPhotoPrefetch.hasExpired()) {
        m_photoPrefetchTimer.start(int(m_nextPhotoPrefetch.remainingTime()));
        return;
    }
    prefetchNextSenderPhoto();
}
void CardStore::prefetchNextSenderPhoto() {
    if (!m_online || !m_open) return;
    m_nextPhotoPrefetch = QDeadlineTimer(3000, Qt::PreciseTimer);
    m_photoPrefetchTimer.start(3000);
    if (m_photoPrefetchQueue.isEmpty()) return;
    const QString id = m_photoPrefetchQueue.takeFirst();
    m_photoRequests.insert(id, QDeadlineTimer(60000, Qt::PreciseTimer));
    emit requestSenderPhotoGet(id, QStringLiteral("/v1/cards/") + id + QStringLiteral("/sender-photo"));
}
QString CardStore::cachedSenderPhoto(const QString &cardId) const {
    if (!m_open || cardId.isEmpty()) return {};
    return m_senderPhotos.value(cardId, QString());
}
void CardStore::requestSenderPhoto(const QString &cardId) {
    if (!m_open || cardId.isEmpty()) return;
    if (!m_senderPhotos.value(cardId).isEmpty()) {
        emit senderPhotoChanged(cardId);
        return;
    }
    if (!m_online || m_photoUnavailable.contains(cardId)) return;
    const QVariantMap *card = nullptr;
    for (const QVariantMap &c : m_cards) {
        if (c.value(QStringLiteral("id")).toString() == cardId) { card = &c; break; }
    }
    if (!card || !card->value(QStringLiteral("sender_has_photo")).toBool()) return;
    m_photoPrefetchQueue.removeAll(cardId);
    const auto inFlight = m_photoRequests.constFind(cardId);
    if (inFlight != m_photoRequests.cend() && !inFlight.value().hasExpired()) return;
    m_photoRequests.insert(cardId, QDeadlineTimer(60000, Qt::PreciseTimer));
    if (!m_photoPrefetchTimer.isActive()) m_photoPrefetchTimer.start(3000);
    emit requestSenderPhotoGet(cardId, QStringLiteral("/v1/cards/") + cardId + QStringLiteral("/sender-photo"));
}
void CardStore::applyRemoteSenderPhoto(const QString &cardId, const QByteArray &bytes, const QString &contentType) {
    if (!m_open || cardId.isEmpty()) return;
    m_photoRequests.remove(cardId);
    m_photoPrefetchQueue.removeAll(cardId);
    // Bounded server-mediated bytes only: 1..512 KiB, image magic, no
    // provider URLs or tokens touch the device cache. The server already
    // DecodeConfig-verified; the magic check is defense in depth so a
    // corrupt transport never persists.
    if (bytes.size() < 1 || bytes.size() > (512 << 10)) { emit senderPhotoFailed(cardId); return; }
    const auto hasMagic = [&] {
        const auto u = reinterpret_cast<const unsigned char *>(bytes.constData());
        const int n = bytes.size();
        if (n >= 3 && u[0] == 0xFF && u[1] == 0xD8 && u[2] == 0xFF) return true;
        if (n >= 8 && u[0] == 0x89 && u[1] == 0x50 && u[2] == 0x4E && u[3] == 0x47) return true;
        if (n >= 6 && u[0] == 0x47 && u[1] == 0x49 && u[2] == 0x46 && u[3] == 0x38) return true;
        if (n >= 12 && u[0] == 0x52 && u[1] == 0x49 && u[2] == 0x46 && u[3] == 0x46
            && u[8] == 0x57 && u[9] == 0x45 && u[10] == 0x42 && u[11] == 0x50) return true;
        return false;
    };
    if (!hasMagic()) { emit senderPhotoFailed(cardId); return; }
    if (cardId.contains(QLatin1Char('/')) || cardId.contains(QLatin1Char('\\')) ||
        cardId.contains(QStringLiteral("..")) || cardId.contains(QChar(u'\0')) ||
        cardId.startsWith(QLatin1Char('.'))) { emit senderPhotoFailed(cardId); return; }
    QString ext = QStringLiteral(".jpg");
    const QString ct = contentType.toLower().split(QLatin1Char(';')).first().trimmed();
    if (ct == QStringLiteral("image/png")) ext = QStringLiteral(".png");
    else if (ct == QStringLiteral("image/gif")) ext = QStringLiteral(".gif");
    else if (ct == QStringLiteral("image/webp")) ext = QStringLiteral(".webp");
    const QString base = QStandardPaths::writableLocation(QStandardPaths::CacheLocation);
    if (base.isEmpty()) { emit senderPhotoFailed(cardId); return; }
    QDir dir(base + QStringLiteral("/sender-photos/") + cardId);
    if (!dir.mkpath(QStringLiteral("."))) { emit senderPhotoFailed(cardId); return; }
    const QString filePath = dir.filePath(QStringLiteral("photo") + ext);
    if (QDir::cleanPath(filePath) != QDir::cleanPath(dir.absolutePath() + QStringLiteral("/photo") + ext)) { emit senderPhotoFailed(cardId); return; }
    QFile file(filePath);
    if (!file.open(QIODevice::WriteOnly | QIODevice::Truncate) || file.write(bytes) != bytes.size()) { emit senderPhotoFailed(cardId); return; }
    file.close();
    const QString fileUrl = QUrl::fromLocalFile(filePath).toString();
    m_senderPhotos.insert(cardId, fileUrl);
    m_photoUnavailable.remove(cardId);
    QMetaObject::invokeMethod(m_storage, [this, cardId, fileUrl] {
        QSqlQuery q(m_db);
        q.prepare(QStringLiteral("INSERT INTO sender_photo_files(card_id,file_url) VALUES(?,?) "
                                 "ON CONFLICT(card_id) DO UPDATE SET file_url=excluded.file_url"));
        q.addBindValue(cardId); q.addBindValue(fileUrl);
        const bool ok = q.exec();
        const QString error = q.lastError().text();
        QMetaObject::invokeMethod(this, [this, cardId, ok, error] {
            if (!ok) emit storageError(error);
        }, Qt::QueuedConnection);
    }, Qt::QueuedConnection);
    for (int row = 0; row < m_cards.size(); ++row) {
        if (m_cards[row].value(QStringLiteral("id")).toString() != cardId) continue;
        m_cards[row].insert(QStringLiteral("sender_photo"), fileUrl);
        emit dataChanged(index(row), index(row), {CardRole, SenderPhotoRole});
        break;
    }
    emit senderPhotoChanged(cardId);
}
void CardStore::reportSenderPhotoFailed(const QString &cardId, int status) {
    m_photoRequests.remove(cardId);
    // 404 genuinely unavailable and 403 authorization_required must not spin:
    // park until the next snapshot re-marks eligibility. Transport/offline
    // failures stay retryable through the prefetch queue.
    if (status == 404 || status == 403 || status == 401)
        m_photoUnavailable.insert(cardId);
    else
        m_photoPrefetchQueue.removeAll(cardId);
    emit senderPhotoFailed(cardId);
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
    case SenderPhotoRole: return card.value(QStringLiteral("sender_photo"), QString());
    default: return {};
    }
}
QHash<int,QByteArray> CardStore::roleNames() const {
    return {{CardRole,"card"},{CardIdRole,"cardId"},{TitleRole,"title"},{SectionRole,"section"},{SenderNameRole,"sender_name"},{SenderPhotoRole,"sender_photo"}};
}
