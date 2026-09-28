#pragma once

#include <QHash>
#include <QJSValue>
#include <QJsonObject>
#include <QJsonValue>
#include <QNetworkRequest>
#include <QObject>
#include <QPointer>
#include <QVariantMap>

class QNetworkAccessManager;
class QNetworkReply;
class QJSEngine;

// Api exposes the Litterbox server contract to QML as a small request
// object: get/post/del return a request id answered asynchronously by
// requestFinished/requestFailed signals. Bearer auth is sent whenever a
// token is set; the dev server takes the tenant from the connection
// instead, in which case the token stays empty.
class Api : public QObject
{
    Q_OBJECT
    Q_PROPERTY(QString baseUrl READ baseUrl WRITE setBaseUrl NOTIFY baseUrlChanged)
    Q_PROPERTY(QString token READ token WRITE setToken NOTIFY tokenChanged)
    Q_PROPERTY(int clientApi READ clientApi CONSTANT)
public:
    explicit Api(QObject *parent = nullptr);

    QString baseUrl() const;
    void setBaseUrl(const QString &url);
    QString token() const;
    void setToken(const QString &token);
    int clientApi() const { return 1; }

    Q_INVOKABLE int get(const QString &path, const QJSValue &callback = QJSValue());
    Q_INVOKABLE int post(const QString &path, const QJsonObject &body = QJsonObject(),
                         const QJSValue &callback = QJSValue());
    Q_INVOKABLE int del(const QString &path, const QJSValue &callback = QJSValue());
    void setEngine(QJSEngine *engine) { m_engine = engine; }
    // Streaming GET for the tenant-scoped SSE endpoint; consumers parse
    // streamLine and reconnect on streamFinished.
    Q_INVOKABLE int stream(const QString &path);

signals:
    void baseUrlChanged();
    void tokenChanged();
    void requestFinished(int requestId, const QJsonValue &data, int statusCode);
    void requestFailed(int requestId, int statusCode, const QString &error);
    void streamLine(int requestId, const QString &line);
    void streamFinished(int requestId, const QString &error);

private:
    QNetworkRequest buildRequest(const QString &path) const;
    void trackJsonReply(int id, QNetworkReply *reply);
    void trackStreamReply(int id, QNetworkReply *reply);

    QNetworkAccessManager *m_nam;
    QString m_baseUrl;
    QString m_token;
    int m_nextId = 1;
    QJSEngine *m_engine = nullptr;
    QHash<int, QJSValue> m_callbacks;
    QHash<int, QPointer<QNetworkReply>> m_jsonReplies;
    QHash<int, QPointer<QNetworkReply>> m_streamReplies;
};
