// SPDX-License-Identifier: MIT
#pragma once

#include <QAbstractListModel>
#include <QTimer>
#include <QDeadlineTimer>
#include <QSqlDatabase>
#include <QStringList>
#include <QVariantMap>
#include <QThread>
#include <QSet>
#include <functional>

namespace cardstore {
class OpTransport {
public:
    virtual ~OpTransport() = default;
    virtual void postOp(const QString &path, const QVariantMap &body,
                        std::function<void(int, const QVariantMap &)> completed) = 0;
};
}

// Cache-first QML model. SQLite belongs to the storage worker; QML sees
// optimistic row-level changes. Operations transmit only after durable commit.
// Rejected actions roll back; owner-written content stays durable and retryable.
class CardStore : public QAbstractListModel {
    Q_OBJECT
    Q_PROPERTY(bool online READ online WRITE setOnline NOTIFY onlineChanged)
    Q_PROPERTY(QString offlineReason READ offlineReason NOTIFY offlineReasonChanged)
    Q_PROPERTY(int pendingOps READ pendingOps NOTIFY pendingOpsChanged)
    Q_PROPERTY(bool saveFailed READ saveFailed NOTIFY pendingOpsChanged)
    Q_PROPERTY(int mailSyncPending READ mailSyncPending NOTIFY mailSyncStateChanged)
    Q_PROPERTY(bool mailSyncComplete READ mailSyncComplete NOTIFY mailSyncStateChanged)
public:
    enum Role { CardRole = Qt::UserRole + 1, CardIdRole, TitleRole, SectionRole, SenderNameRole, SenderPhotoRole };
    Q_ENUM(Role)
    explicit CardStore(QObject *parent = nullptr);
    ~CardStore() override;
    Q_INVOKABLE bool open(const QString &path);
    bool online() const { return m_online; }
    void setOnline(bool online);
    QString offlineReason() const { return m_offlineReason; }
    void reportOffline(const QString &reason);
    int pendingOps() const;
    bool saveFailed() const { return !m_failedWrites.isEmpty(); }
    Q_INVOKABLE bool retainedWriteFailed(const QString &opId) const { return m_failedWrites.contains(opId); }
    void setTransport(cardstore::OpTransport *transport);
    void setMaxInFlight(int ops) { m_maxInFlight = qMax(1, ops); }
    static void registerQml(const char *uri, int major, int minor);
    Q_INVOKABLE void refresh();
    Q_INVOKABLE void retrySync();
    // Request IDs order HTTP responses, not local mutations or ACKs. Direct
    // cache/test snapshots (ID zero) are authoritative at application time.
    Q_INVOKABLE bool applyRemoteCards(const QVariantMap &sections, quint64 requestId = 0);
    // Semantic reminder label is independent of the token-bound source identity.
    Q_INVOKABLE QString sourceLabel(const QVariantMap &card) const {
        if (card.value(QStringLiteral("source_kind")).toString() == QStringLiteral("reminder"))
            return QStringLiteral("reminder");
        return card.value(QStringLiteral("source")).toString();
    }
    Q_INVOKABLE bool createCard(const QString &title, const QString &summary = {},
                                const QString &kind = QStringLiteral("manual"));
    Q_INVOKABLE QString createReminder(const QString &title, const QString &dueAt, const QString &recurrence = {});
    Q_INVOKABLE QString sendAgentMessage(const QString &threadId, const QString &text, const QString &clientMsgId = {});
    Q_INVOKABLE QVariantList pendingAgentMessages(const QString &threadId) const;
    Q_INVOKABLE QStringList cardIds() const;
    Q_INVOKABLE bool moveCard(const QString &cardId, int delta);
    Q_INVOKABLE bool moveCardTo(const QString &cardId, int targetIndex);
    Q_INVOKABLE QStringList pinnedCardIds() const;
    Q_INVOKABLE QString reorderCards(const QStringList &ids);
    Q_INVOKABLE QString enqueueOp(const QString &cardId, const QString &type,
                                  const QVariantMap &args = {});
    Q_INVOKABLE QString dismiss(const QString &cardId) { return enqueueOp(cardId, QStringLiteral("done")); }
    Q_INVOKABLE QString saveNote(const QString &cardId, const QString &note) {
        return enqueueOp(cardId, QStringLiteral("note"), {{QStringLiteral("note"), note}});
    }
    // Bundle archive acts at once, with no confirmation. It queues one
    // per-card `archive` op for every open unpinned card in the bundle (R22),
    // so each message is archived in its originating Gmail account (R7), and
    // returns {token, count}; an empty map means nothing was archived.
    Q_PROPERTY(bool bundleArchiveUndoActive READ bundleArchiveUndoActive NOTIFY bundleArchiveUndoChanged)
    Q_PROPERTY(int bundleArchiveUndoRemainingMs READ bundleArchiveUndoRemainingMs NOTIFY bundleArchiveUndoChanged)
    Q_INVOKABLE void setBundleArchiveUndoDurationForTest(int milliseconds);
    bool bundleArchiveUndoActive() const;
    int bundleArchiveUndoRemainingMs() const;
    Q_INVOKABLE QVariantMap archiveBundleNow(const QString &bundleId);
    // Single-card archive enqueueOp IDs are also tokens for this same Undo window.
    Q_INVOKABLE bool undoBundleArchive(const QString &token);
    // Cancels a queued op before it is sent and restores its optimistic rows.
    Q_INVOKABLE void undoQueuedOp(const QString &opId);
    Q_INVOKABLE void flush();
    Q_INVOKABLE void reportPostResult(const QString &opId, int status,
                                     const QVariantMap &response);
    // Durable per-card body cache, independent of the open-card set.
    // Mail and has_body agent result bodies share the same mail_bodies table
    // so full results and file links survive restart and read offline.
    // cachedMailBody returns {html, source_url, messages} or an empty map on a miss.
    // messages is a JSON array retained verbatim for offline thread reading.
    Q_INVOKABLE QVariantMap cachedMailBody(const QString &cardId) const;
    // Pending counts fetchable open cards missing a durable original. Completion
    // additionally requires the current metadata snapshot's SQLite commit;
    // connectivity alone is never evidence of completed synchronization.
    int mailSyncPending() const;
    bool mailSyncComplete() const;
    // Fetch a mail or has_body agent card's body through the authenticated
    // API while online. Plain cards without has_body have no fetchable body.
    Q_INVOKABLE void requestMailBody(const QString &cardId);
    // Remote body entrypoints: applyRemoteMailBody persists before signalling;
    // reportMailBodyFailed never deletes already-cached content.
    Q_INVOKABLE void applyRemoteMailBody(const QString &cardId, const QVariantMap &body);
    Q_INVOKABLE void reportMailBodyFailed(const QString &cardId, int status = 0);
    // Open a file embedded as a data: link in the cached body. Accepts only a
    // data: URL present byte-for-byte in that card's cached HTML, decodes its
    // base64 payload, writes it under CacheLocation/card-files/<cardId>/<name>
    // using only the sanitized name= parameter, and returns the local file URL
    // ("" on any mismatch). Never uses a producer-supplied path.
    Q_INVOKABLE QString openCachedFile(const QString &cardId, const QString &dataUrl) const;
    // R67: device-cached Google sender photos (server-mediated bytes only).
    // sender_photo is a local file:// URL ("" when absent); sender_has_photo
    // comes from the server payload. requestSenderPhoto emits
    // requestSenderPhotoGet for the authenticated binary route while online;
    // applyRemoteSenderPhoto validates, persists, and injects the file URL;
    // reportSenderPhotoFailed never deletes cached bytes. 404 means
    // genuinely unavailable (initials); 403 means authorization_required.
    Q_INVOKABLE QString cachedSenderPhoto(const QString &cardId) const;
    Q_INVOKABLE void requestSenderPhoto(const QString &cardId);
    Q_INVOKABLE void applyRemoteSenderPhoto(const QString &cardId, const QByteArray &bytes, const QString &contentType);
    Q_INVOKABLE void reportSenderPhotoFailed(const QString &cardId, int status = 0);
    int rowCount(const QModelIndex &parent = {}) const override;
    QVariant data(const QModelIndex &index, int role) const override;
    QHash<int, QByteArray> roleNames() const override;
signals:
    void bundleArchiveUndoChanged();
    void bundleArchiveExpired(const QString &token);
    void onlineChanged();
    void offlineReasonChanged();
    void pendingOpsChanged();
    void requestPost(const QString &opId, const QString &path, const QVariantMap &body);
    void requestCards(const QString &path, quint64 requestId);
    void requestMailBodyGet(const QString &cardId, const QString &path);
    void requestSenderPhotoGet(const QString &cardId, const QString &path);
    void mailBodyChanged(const QString &cardId);
    void mailBodyFailed(const QString &cardId);
    void mailSyncStateChanged();
    void senderPhotoChanged(const QString &cardId);
    void senderPhotoFailed(const QString &cardId);
    void operationFailed(const QString &opId, int status, const QVariantMap &response);
    void operationAcknowledged(const QVariantMap &operation, const QVariantMap &response);
    void storageError(const QString &message);
    void flushFinished();
private:
    void replaceCards(QList<QVariantMap> cards);
    // Commits card rows, inserted ops and cancelled op ids in one transaction.
    void persistCards(const QList<QVariantMap> &before, const QList<QVariantMap> &after,
                      const QList<QVariantMap> &ops = {}, const QStringList &cancelledOps = {},
                      quint64 bodySnapshot = 0);
    bool writeCards(const QList<QVariantMap> &before, const QList<QVariantMap> &after);
    void undoOp(const QString &opId);
    static bool ownerWrite(const QVariantMap &op);
    void retainOwnerRows(QList<QVariantMap> &cards) const;
    void projectPendingRemovals(QList<QVariantMap> &cards) const;
    bool appendDraftRecord(const QVariantMap &record);
    bool appendDraftRecords(const QList<QVariantMap> &records);
    // ACKed ops waiting for their group commit; see commitAcks().
    struct PendingAck {
        QString opId;
        QVariantMap response;
        QVariantMap operation;
        QStringList removedIds;
        bool ownerWrite;
    };
    void commitAcks();
    QList<PendingAck> m_ackQueue;
    bool m_ackCommitting = false;
    bool m_ackScheduled = false;
    // Drops the local row the server confirmed gone (404 or 422 card-not-found
    // retirement) unless a remaining op still writes that card. Card-id carriers
    // with no card subject (reorder_*, agent_message) never trigger a drop.
    void dropServerDeletedCard(const QString &cardId);
    void retireRejectedOperation(const QString &opId, const QVariantMap &replacement = {}, const QString &retiredCardId = {});
    QString m_rejectedReorder;
    quint64 m_rejectedReorderSnapshot = 0;
    QString m_draftPath;
    bool m_draftDirSynced = false;
    QHash<QString, QVariantMap> m_drafts;
    QStringList m_draftOrder;
    // Bounded outbox pipeline: up to m_maxInFlight POSTs share the wire.
    // Per-card FIFO holds (a card's next op waits for its in-flight op) and
    // reorder_* ops fly alone so the server always sees a coherent open set.
    int m_maxInFlight = 32;
    bool flushNext();
    void queueMailBodyPrefetch();
    void prefetchNextMailBody();
    void queueSenderPhotoPrefetch();
    void prefetchNextSenderPhoto();
    void injectSenderPhotos(QList<QVariantMap> &cards);
    bool execute(const QString &sql);
    void deriveBundles(QList<QVariantMap> &cards);
    QString m_connection;
    QSqlDatabase m_db;
    QList<QVariantMap> m_cards;
    QThread m_storageThread;
    QObject *m_storage = nullptr;
    bool m_open = false;
    QList<QVariantMap> m_outbox;
    QSet<QString> m_durableOps;
    QSet<QString> m_failedWrites;
    QSet<QString> m_deferredWrites;
    struct Mutation {
        QList<QVariantMap> before, after;
        bool archiveRejected = false;
        bool restoreDeferred = false;
    };
    QHash<QString, Mutation> m_mutations;
    // One Undo window per single-card or bundle archive, retained across ACK.
    // Transmitted ops remain ordered before their inverse even after a lost ACK.
    struct BundleArchive {
        QList<QVariantMap> before;
        QList<QPair<QString, QString>> ops;
        QDeadlineTimer deadline;
        QSet<QString> sent;
    };
    QHash<QString, BundleArchive> m_bundleArchives;
    QHash<QString, QString> m_bundleFailureGroups;
    QSet<QString> m_reportedBundleFailures;
    void expireBundleArchives();
    QSet<QString> m_archivedAwaitingSnapshot;
    // Retain Undo only for requests issued before the inverse ACK. The first
    // post-ACK snapshot is authoritative even when another device archived it.
    QHash<QString, quint64> m_restoredAwaitingSnapshot;
    quint64 m_cardsRequestSequence = 0;
    quint64 m_latestCardsResponse = 0;
    int m_bundleUndoDurationMs = 8000;
    QTimer *m_bundleUndoTimer = nullptr;
    // Body ids only; html/messages stay in SQLite and are read on demand
    // through a byte-bounded LRU (R9 bodies on demand).
    QSet<QString> m_bodyCached;
    mutable QHash<QString, QVariantMap> m_bodyLru;
    mutable QStringList m_bodyLruOrder; // most recent last
    mutable qint64 m_bodyLruBytes = 0;
    QVariantMap readBodyFromDb(const QString &cardId) const;
    void lruRemove(const QString &cardId) const;
    void lruInsert(const QString &cardId, const QVariantMap &body) const;
public:
    // Bytes (UTF-16) of body HTML currently resident in the LRU, for tests.
    qint64 residentBodyBytes() const { return m_bodyLruBytes; }
    static constexpr qint64 kBodyLruBytes = 24 * 1024 * 1024;
private:
    QStringList m_bodyPrefetchQueue;
    QSet<QString> m_bodyRequests;
    QTimer m_bodyPrefetchTimer;
    QDeadlineTimer m_bodyBackoff{0, Qt::PreciseTimer};
    QHash<QString, QDeadlineTimer> m_bodyRetryAfter;
    QHash<QString, int> m_bodyRetryAttempts;
    QSet<QString> m_bodyPersisting;
    quint64 m_durableBodySnapshot = 0;
    // R67: device-cached sender-photo file URLs by card id (local file://,
    // never provider URLs). Persisted in sender_photo_files; injected into
    // card maps as sender_photo so delegates render without extra lookups.
    QHash<QString, QString> m_senderPhotos;
    QStringList m_photoPrefetchQueue;
    QHash<QString, QDeadlineTimer> m_photoRequests;
    QTimer m_photoPrefetchTimer;
    QDeadlineTimer m_nextPhotoPrefetch{0, Qt::PreciseTimer};
    // Cards whose photo route answered 404/403: genuinely unavailable or
    // authorization_required. Excluded from prefetch until the next snapshot
    // re-marks eligibility; never logged with addresses.
    QSet<QString> m_photoUnavailable;
    bool m_online = false;
    QString m_offlineReason;
    // Bounded pipeline of sent-but-unacknowledged op ids (cap m_maxInFlight).
    QSet<QString> m_inFlight;
    // A 429 parks the whole pump until the shared per-device window recovers.
    QDeadlineTimer m_rateCooldown{0, Qt::PreciseTimer};
    QTimer m_retryTimer;
    int m_retryAttempt = 0;
    cardstore::OpTransport *m_transport = nullptr;
};
