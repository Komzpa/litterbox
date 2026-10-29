// SPDX-License-Identifier: MIT
#pragma once
#include "CardStore.h"
#include "api.h"
#include <QHash>
#include <QElapsedTimer>
#include <QJsonObject>
#include <QUrl>

// The production main.cpp binding, also exercised with a real Api in QtTest.
// Card operations use /v1/ops; chat uses its idempotent client_msg_id ledger.
class CardStoreTransport : public QObject {
public:
    CardStoreTransport(CardStore &store, Api &api) : QObject(&store) {
        connect(&store, &CardStore::requestPost, this,
            [this, &api](const QString &opId, const QString &path, const QVariantMap &payload) {
                const auto args = payload.value(QStringLiteral("args")).toMap();
                const bool message = payload.value(QStringLiteral("type")) == QStringLiteral("agent_message");
                const QString endpoint = message ? QStringLiteral("/v1/a2a/threads/") +
                    QString::fromLatin1(QUrl::toPercentEncoding(args.value(QStringLiteral("thread_id")).toString())) + QStringLiteral("/messages") : path;
                const QVariantMap body = message ? QVariantMap{{QStringLiteral("client_msg_id"), args.value(QStringLiteral("client_msg_id"))},
                    {QStringLiteral("text"), args.value(QStringLiteral("text"))}} : payload;
                const int request = api.post(endpoint, QJsonObject::fromVariantMap(body));
                m_requests.insert(request, opId);
                if (message) m_messageRequests.insert(request);
            });
        connect(&api, &Api::requestFinished, this,
            [this, &store](int id, const QJsonValue &data, int status) {
                if (!m_requests.contains(id)) return;
                auto response = data.toObject().toVariantMap();
                if (m_messageRequests.remove(id) && status >= 200 && status < 300) response.insert(QStringLiteral("ok"), true);
                if (status == 204) response.insert(QStringLiteral("ok"), true);
                store.reportPostResult(m_requests.take(id), status, response);
            });
        connect(&api, &Api::requestFailed, this,
            [this, &store](int id, int status, const QString &, const QJsonValue &data) {
                if (!m_requests.contains(id)) return;
                m_messageRequests.remove(id);
                store.reportPostResult(m_requests.take(id), status, data.toObject().toVariantMap());
            });
    }
private:
    QHash<int, QString> m_requests;
    QSet<int> m_messageRequests;
};

// Events carry no card or device identity. Coalesce them while a snapshot or
// outbox drain is in progress; a bounded fallback keeps remote changes visible
// even when a retained write cannot drain.
class CardEventRefresh : public QObject {
public:
    CardEventRefresh(CardStore &store, const QHash<int, quint64> &requests)
        : QObject(&store), m_store(store), m_requests(requests) {
        m_timer.setSingleShot(true);
        m_timer.setTimerType(Qt::PreciseTimer);
        connect(&m_timer, &QTimer::timeout, this, [this] {
            if (!m_dirty || !m_store.online() || !m_requests.isEmpty()) return;
            m_dirty = false;
            m_firstDirty.invalidate();
            m_store.refresh();
        });
        connect(&store, &CardStore::pendingOpsChanged, this, [this] {
            if (m_dirty && m_store.pendingOps() == 0) schedule();
        });
    }
    void event() { if (!m_dirty) m_firstDirty.start(); m_dirty = true; schedule(); }
    void snapshotFinished() { if (m_dirty) schedule(); }
private:
    void schedule() {
        if (!m_dirty || !m_store.online() || !m_requests.isEmpty()) return;
        const int deadline = m_store.pendingOps() ? 10000 : 500;
        const int remaining = qMax(0, deadline - int(m_firstDirty.elapsed()));
        const int delay = m_store.pendingOps() ? remaining : qMin(150, remaining);
        m_timer.start(delay);
    }
    CardStore &m_store;
    const QHash<int, quint64> &m_requests;
    QTimer m_timer;
    QElapsedTimer m_firstDirty;
    bool m_dirty = false;
};
