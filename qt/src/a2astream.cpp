#include "a2astream.h"
#include "api.h"

#include <QJsonDocument>
#include <QJsonObject>
#include <QNetworkAccessManager>
#include <QNetworkReply>
#include <QNetworkRequest>
#include <QUrl>

A2aStream::A2aStream(Api *api, QObject *parent)
    : QObject(parent), m_api(api), m_nam(new QNetworkAccessManager(this))
{
    m_reconnect.setSingleShot(true);
    connect(&m_reconnect, &QTimer::timeout, this, &A2aStream::open);
    connect(api, &Api::baseUrlChanged, this, &A2aStream::reset);
    connect(api, &Api::tokenChanged, this, &A2aStream::reset);
    m_reconnect.start(0);
}

A2aStream::~A2aStream()
{
    closeReply();
}

void A2aStream::start()
{
    if (m_running) return;
    m_running = true;
    reset();
}

void A2aStream::stop()
{
    m_running = false;
    m_reconnect.stop();
    closeReply();
}

void A2aStream::closeReply()
{
    if (m_reply) {
        disconnect(m_reply, nullptr, this, nullptr);
        m_reply->abort();
        m_reply->deleteLater();
        m_reply = nullptr;
    }
    m_buffer.clear();
    m_event.clear();
    m_data.clear();
    m_accepted = false;
}

void A2aStream::reset()
{
    m_reconnect.stop();
    closeReply();
    m_retrySeconds = 1;
    if (m_running) m_reconnect.start(0);
}

void A2aStream::open()
{
    if (!m_running || m_reply || m_api->token().isEmpty()) return;
    QString base = m_api->baseUrl();
    while (base.endsWith(QLatin1Char('/'))) base.chop(1);
    const QUrl url(base + QStringLiteral("/v1/a2a/events"));
    if ((url.scheme() != QStringLiteral("https") && url.scheme() != QStringLiteral("http")) || url.host().isEmpty()) return;

    QNetworkRequest request(url);
    request.setRawHeader("Accept", "text/event-stream");
    request.setRawHeader("X-Litterbox-Api", QByteArray::number(m_api->clientApi()));
    request.setRawHeader("Authorization", "Bearer " + m_api->token().toUtf8());
    // A streaming reply has a separate lifetime from Api's finite JSON calls.
    // Never forward network error strings or response bodies to logs/signals:
    // a server may echo credentials even when denying the stream.
    m_reply = m_nam->get(request);
    connect(m_reply, &QNetworkReply::metaDataChanged, this, [this] {
        const int status = m_reply->attribute(QNetworkRequest::HttpStatusCodeAttribute).toInt();
        const QByteArray type = m_reply->rawHeader("Content-Type").split(';').first().trimmed();
        if (!m_accepted && status == 200 && type == "text/event-stream") {
            m_accepted = true;
            emit reconnected();
        }
    });
    connect(m_reply, &QNetworkReply::readyRead, this, &A2aStream::consume);
    connect(m_reply, &QNetworkReply::finished, this, [this] {
        consume();
        closeReply();
        if (m_running) {
            m_reconnect.start(m_retrySeconds * 1000);
            m_retrySeconds = qMin(m_retrySeconds * 2, 10);
        }
    });
}

void A2aStream::consume()
{
    if (!m_reply) return;
    const QByteArray bytes = m_reply->readAll();
    if (!m_accepted) return;
    m_buffer.append(bytes);
    for (qsizetype pos = m_buffer.indexOf('\n'); pos >= 0; pos = m_buffer.indexOf('\n')) {
        QByteArray value = m_buffer.left(pos);
        m_buffer.remove(0, pos + 1);
        if (value.endsWith('\r')) value.chop(1);
        line(value);
    }
}

void A2aStream::line(const QByteArray &value)
{
    if (value.isEmpty()) {
        if (m_event == "a2a") {
            const QJsonDocument doc = QJsonDocument::fromJson(m_data);
            const QJsonValue seq = doc.object().value(QStringLiteral("seq"));
            const qint64 number = seq.toInteger(-1);
            if (seq.isDouble() && number >= 0 && seq.toDouble() == double(number)) {
                m_retrySeconds = 1;
                emit changed(number);
            }
        }
        m_event.clear();
        m_data.clear();
        return;
    }
    if (value.startsWith(':')) {
        m_retrySeconds = 1; // The server's keepalive means the stream is healthy.
        return;
    }
    const qsizetype colon = value.indexOf(':');
    const QByteArray field = colon < 0 ? value : value.left(colon);
    QByteArray data = colon < 0 ? QByteArray() : value.mid(colon + 1);
    if (data.startsWith(' ')) data.remove(0, 1);
    if (field == "event") m_event = data;
    else if (field == "data") {
        if (!m_data.isEmpty()) m_data.append('\n');
        m_data.append(data);
    }
}
