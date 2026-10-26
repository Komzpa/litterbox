// SPDX-License-Identifier: MIT
#pragma once
#include "CardStore.h"
#include "api.h"
#include <QHash>
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
