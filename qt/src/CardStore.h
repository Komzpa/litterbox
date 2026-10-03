// SPDX-License-Identifier: MIT
#pragma once

#include <QAbstractListModel>
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
// Failures undo their changed rows without resetting unrelated cards.
class CardStore : public QAbstractListModel {
    Q_OBJECT
    Q_PROPERTY(bool online READ online WRITE setOnline NOTIFY onlineChanged)
    Q_PROPERTY(int pendingOps READ pendingOps NOTIFY pendingOpsChanged)
public:
    enum Role { CardRole = Qt::UserRole + 1, CardIdRole, TitleRole, SectionRole };
    Q_ENUM(Role)
    explicit CardStore(QObject *parent = nullptr);
    ~CardStore() override;
    Q_INVOKABLE bool open(const QString &path);
    bool online() const { return m_online; }
    void setOnline(bool online);
    int pendingOps() const;
    void setTransport(cardstore::OpTransport *transport);
    static void registerQml(const char *uri, int major, int minor);
    Q_INVOKABLE void refresh();
    Q_INVOKABLE bool applyRemoteCards(const QVariantMap &sections);
    // Semantic reminder label is independent of the token-bound source identity.
    Q_INVOKABLE QString sourceLabel(const QVariantMap &card) const {
        if (card.value(QStringLiteral("source_kind")).toString() == QStringLiteral("reminder"))
            return QStringLiteral("reminder");
        return card.value(QStringLiteral("source")).toString();
    }
    Q_INVOKABLE bool createCard(const QString &title, const QString &summary = {},
                                const QString &kind = QStringLiteral("manual"));
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
    Q_INVOKABLE void flush();
    Q_INVOKABLE void reportPostResult(const QString &opId, int status,
                                     const QVariantMap &response);
    // Durable per-card body cache, independent of the open-card set.
    // Mail and has_body agent result bodies share the same mail_bodies table
    // so full results and file links survive restart and read offline.
    // cachedMailBody returns {html, source_url} or an empty map on a miss.
    Q_INVOKABLE QVariantMap cachedMailBody(const QString &cardId) const;
    // Fetch a mail or has_body agent card's body through the authenticated
    // API while online. Plain cards without has_body have no fetchable body.
    Q_INVOKABLE void requestMailBody(const QString &cardId);
    // Remote body entrypoints: applyRemoteMailBody persists before signalling;
    // reportMailBodyFailed never deletes already-cached content.
    Q_INVOKABLE void applyRemoteMailBody(const QString &cardId, const QVariantMap &body);
    Q_INVOKABLE void reportMailBodyFailed(const QString &cardId);
    // Open a file embedded as a data: link in the cached body. Accepts only a
    // data: URL present byte-for-byte in that card's cached HTML, decodes its
    // base64 payload, writes it under CacheLocation/card-files/<cardId>/<name>
    // using only the sanitized name= parameter, and returns the local file URL
    // ("" on any mismatch). Never uses a producer-supplied path.
    Q_INVOKABLE QString openCachedFile(const QString &cardId, const QString &dataUrl) const;
    int rowCount(const QModelIndex &parent = {}) const override;
    QVariant data(const QModelIndex &index, int role) const override;
    QHash<int, QByteArray> roleNames() const override;
signals:
    void onlineChanged();
    void pendingOpsChanged();
    void requestPost(const QString &opId, const QString &path, const QVariantMap &body);
    void requestCards(const QString &path);
    void requestMailBodyGet(const QString &cardId, const QString &path);
    void mailBodyChanged(const QString &cardId);
    void mailBodyFailed(const QString &cardId);
    void operationFailed(const QString &opId, int status);
    void storageError(const QString &message);
    void flushFinished();
private:
    void replaceCards(QList<QVariantMap> cards);
    void persistCards(const QList<QVariantMap> &before, const QList<QVariantMap> &after,
                      const QVariantMap &op = {});
    bool writeCards(const QList<QVariantMap> &before, const QList<QVariantMap> &after);
    void undoOp(const QString &opId);
    void flushNext();
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
    struct Mutation { QList<QVariantMap> before, after; };
    QHash<QString, Mutation> m_mutations;
    QHash<QString, QVariantMap> m_mailBodies;
    bool m_online = false;
    QString m_inFlight;
    QString m_failedOp;
    cardstore::OpTransport *m_transport = nullptr;
};
