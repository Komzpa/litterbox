#include "api.h"

#include <QJSEngine>
#include <QJsonArray>
#include <QJsonDocument>
#include <QNetworkAccessManager>
#include <QNetworkReply>
#include <QNetworkRequest>

Api::Api(QObject *parent)
    : QObject(parent)
    , m_nam(new QNetworkAccessManager(this))
{
    m_jsonPool.setMaxThreadCount(1);
}

Api::~Api() { m_jsonPool.waitForDone(); }

QString Api::baseUrl() const { return m_baseUrl; }

void Api::setBaseUrl(const QString &url)
{
    if (m_baseUrl == url)
        return;
    m_baseUrl = url;
    emit baseUrlChanged();
}
QString Api::resolveBaseUrl(bool testProfileMode, const QString &testServerUrl,
                             const QString &envUrl, const QString &settingsUrl,
                             const QString &compiledDefault)
{
    if (testProfileMode)
        return testServerUrl;
    if (!envUrl.isEmpty())
        return envUrl;
    if (!settingsUrl.isEmpty())
        return settingsUrl;
    return compiledDefault;
}

QString Api::token() const { return m_token; }

void Api::setToken(const QString &token)
{
    if (m_token == token)
        return;
    m_token = token;
    emit tokenChanged();
}

QNetworkRequest Api::buildRequest(const QString &path) const
{
    const QUrl url = path.startsWith(QStringLiteral("http")) ? QUrl(path) : QUrl(m_baseUrl + path);
    QNetworkRequest request(url);
    request.setRawHeader("Accept", "application/json");
    request.setRawHeader("X-Litterbox-Api", QByteArray::number(clientApi()));
    if (!m_token.isEmpty())
        request.setRawHeader("Authorization", "Bearer " + m_token.toUtf8());
    return request;
}

namespace {
// QJSEngine::toScriptValue(QVariantList) produces a value that stringifies
// like an array but fails Array.isArray, so convert explicitly. newArray /
// newObject always yield genuine JS values.
QJSValue jsonToJs(QJSEngine *engine, const QJsonValue &value)
{
    if (value.isArray()) {
        const QJsonArray array = value.toArray();
        QJSValue result = engine->newArray(array.size());
        for (qsizetype i = 0; i < array.size(); ++i)
            result.setProperty(i, jsonToJs(engine, array.at(i)));
        return result;
    }
    if (value.isObject()) {
        const QJsonObject object = value.toObject();
        QJSValue result = engine->newObject();
        for (auto it = object.begin(); it != object.end(); ++it)
            result.setProperty(it.key(), jsonToJs(engine, it.value()));
        return result;
    }
    if (value.isString())
        return QJSValue(value.toString());
    if (value.isBool())
        return QJSValue(value.toBool());
    if (value.isDouble())
        return QJSValue(value.toDouble());
    return QJSValue(QJSValue::NullValue);
}
} // namespace

void Api::trackJsonReply(int id, QNetworkReply *reply)
{
    m_jsonReplies.insert(id, reply);
    connect(reply, &QNetworkReply::finished, this, [this, id, reply]() {
        m_jsonReplies.remove(id);
        const int status = reply->attribute(QNetworkRequest::HttpStatusCodeAttribute).toInt();
        const QByteArray body = reply->readAll();
        const QString transportError = reply->error() != QNetworkReply::NoError ? reply->errorString() : QString();
        reply->deleteLater();
        m_jsonPool.start([this, id, status, body, transportError] {
            QString error;
            QJsonValue data(QJsonValue::Undefined);
            if (status < 200 || status >= 300) {
                error = transportError.isEmpty() ? QStringLiteral("HTTP %1: %2").arg(status).arg(QString::fromUtf8(body.left(200))) : transportError;
            } else if (!body.trimmed().isEmpty()) {
                QJsonParseError parseError{};
                const QJsonDocument doc = QJsonDocument::fromJson(body, &parseError);
                if (parseError.error != QJsonParseError::NoError)
                    error = QStringLiteral("invalid JSON: %1").arg(parseError.errorString());
                else
                    data = doc.isArray() ? QJsonValue(doc.array()) : QJsonValue(doc.object());
            }
            // QJSValue and model consumers remain owned by the GUI thread.
            QMetaObject::invokeMethod(this, [this, id, status, data, error] {
                const QJSValue callback = m_callbacks.take(id);
                if (!error.isEmpty()) {
                    if (callback.isCallable() && m_engine)
                        callback.call({m_engine->newErrorObject(QJSValue::GenericError, error), QJSValue(QJSValue::NullValue)});
                    emit requestFailed(id, status, error);
                    return;
                }
                if (callback.isCallable() && m_engine) {
                    QJSValue response = m_engine->newObject();
                    response.setProperty(QStringLiteral("status"), status);
                    response.setProperty(QStringLiteral("body"), data.isUndefined() ? QJSValue(true) : jsonToJs(m_engine, data));
                    callback.call({QJSValue(QJSValue::NullValue), response});
                }
                emit requestFinished(id, data, status);
            }, Qt::QueuedConnection);
        });
    });
}

void Api::trackStreamReply(int id, QNetworkReply *reply)
{
    m_streamReplies.insert(id, reply);
    QByteArray *buffer = new QByteArray;
    connect(reply, &QNetworkReply::readyRead, this, [this, id, reply, buffer]() {
        buffer->append(reply->readAll());
        for (qsizetype pos = buffer->indexOf('\n'); pos != -1; pos = buffer->indexOf('\n')) {
            const QString line = QString::fromUtf8(buffer->left(pos).trimmed());
            buffer->remove(0, pos + 1);
            emit streamLine(id, line);
        }
    });
    connect(reply, &QNetworkReply::finished, this, [this, id, reply, buffer]() {
        m_streamReplies.remove(id);
        const QString error = reply->error() != QNetworkReply::NoError ? reply->errorString() : QString();
        reply->deleteLater();
        delete buffer;
        emit streamFinished(id, error);
    });
}

int Api::get(const QString &path, const QJSValue &callback)
{
    const int id = m_nextId++;
    if (callback.isCallable()) m_callbacks.insert(id, callback);
    trackJsonReply(id, m_nam->get(buildRequest(path)));
    return id;
}

int Api::post(const QString &path, const QJsonObject &body, const QJSValue &callback)
{
    QNetworkRequest request = buildRequest(path);
    request.setHeader(QNetworkRequest::ContentTypeHeader, QStringLiteral("application/json"));
    const int id = m_nextId++;
    if (callback.isCallable()) m_callbacks.insert(id, callback);
    trackJsonReply(id, m_nam->post(request, QJsonDocument(body).toJson(QJsonDocument::Compact)));
    return id;
}

int Api::del(const QString &path, const QJSValue &callback)
{
    const int id = m_nextId++;
    if (callback.isCallable()) m_callbacks.insert(id, callback);
    trackJsonReply(id, m_nam->deleteResource(buildRequest(path)));
    return id;
}

int Api::stream(const QString &path)
{
    QNetworkRequest request = buildRequest(path);
    request.setAttribute(QNetworkRequest::RedirectPolicyAttribute, QNetworkRequest::NoLessSafeRedirectPolicy);
    const int id = m_nextId++;
    trackStreamReply(id, m_nam->get(request));
    return id;
}
