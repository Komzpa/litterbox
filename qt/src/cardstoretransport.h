// SPDX-License-Identifier: MIT
#pragma once
#include "CardStore.h"
#include "api.h"
#include <QHash>
#include <QJsonObject>

// The production main.cpp binding, also exercised with a real Api in QtTest.
// Every operation uses the server's idempotent ledger, never a legacy endpoint.
class CardStoreTransport : public QObject {
public:
    CardStoreTransport(CardStore &store, Api &api) : QObject(&store) {
        connect(&store, &CardStore::requestPost, this,
            [this, &api](const QString &opId, const QString &path, const QVariantMap &payload) {
                m_requests.insert(api.post(path, QJsonObject::fromVariantMap(payload)), opId);
            });
        connect(&api, &Api::requestFinished, this,
            [this, &store](int id, const QJsonValue &data, int status) {
                if (!m_requests.contains(id)) return;
                auto response = data.toObject().toVariantMap();
                if (status == 204) response.insert(QStringLiteral("ok"), true);
                store.reportPostResult(m_requests.take(id), status, response);
            });
        connect(&api, &Api::requestFailed, this,
            [this, &store](int id, int status, const QString &, const QJsonValue &data) {
                if (!m_requests.contains(id)) return;
                store.reportPostResult(m_requests.take(id), status, data.toObject().toVariantMap());
            });
    }
private:
    QHash<int, QString> m_requests;
};
