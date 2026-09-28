// SPDX-License-Identifier: MIT
#pragma once

#include <QAbstractListModel>
#include <QSqlDatabase>
#include <QVariantMap>
#include <functional>

namespace cardstore {
class OpTransport {
public:
    virtual ~OpTransport() = default;
    virtual void postOp(const QString &path, const QVariantMap &body,
                        std::function<void(int, const QVariantMap &)> completed) = 0;
};
}

// Cache-first QML model. The bridge sends requestPost/requestCards through
// the shell's authenticated api. Every action is committed to SQLite before
// transmission. Failed operations remain queued, including rejected 4xx ops.
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
    Q_INVOKABLE QString enqueueOp(const QString &cardId, const QString &type,
                                  const QVariantMap &args = {});
    Q_INVOKABLE QString dismiss(const QString &cardId) { return enqueueOp(cardId, QStringLiteral("done")); }
    Q_INVOKABLE QString saveNote(const QString &cardId, const QString &note) {
        return enqueueOp(cardId, QStringLiteral("note"), {{QStringLiteral("note"), note}});
    }
    Q_INVOKABLE void flush();
    Q_INVOKABLE void reportPostResult(const QString &opId, int status,
                                     const QVariantMap &response);
    int rowCount(const QModelIndex &parent = {}) const override;
    QVariant data(const QModelIndex &index, int role) const override;
    QHash<int, QByteArray> roleNames() const override;
signals:
    void onlineChanged();
    void pendingOpsChanged();
    void requestPost(const QString &opId, const QString &path, const QVariantMap &body);
    void requestCards(const QString &path);
    void operationFailed(const QString &opId, int status);
    void storageError(const QString &message);
    void flushFinished();
private:
    void reloadCache();
    void flushNext();
    bool execute(const QString &sql);
    QString m_connection;
    QSqlDatabase m_db;
    QList<QVariantMap> m_cards;
    bool m_online = false;
    QString m_inFlight;
    cardstore::OpTransport *m_transport = nullptr;
};
