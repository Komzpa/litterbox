#pragma once

#include <QByteArray>
#include <QObject>
#include <QPointer>
#include <QTimer>

class Api;
class QNetworkAccessManager;
class QNetworkReply;

// Invalidations only: pages keep their own message cursor and refetch after
// changed(), and also after reconnected() to recover events missed offline.
// Owns the reply so a token/server change cannot leave an old tenant streaming.
class A2aStream : public QObject
{
    Q_OBJECT
public:
    explicit A2aStream(Api *api, QObject *parent = nullptr);
    ~A2aStream() override;

    Q_INVOKABLE void start();
    Q_INVOKABLE void stop();

signals:
    void changed(qint64 seq);
    void reconnected();

private:
    void reset();
    void open();
    void closeReply();
    void consume();
    void line(const QByteArray &line);

    Api *m_api;
    QNetworkAccessManager *m_nam;
    QPointer<QNetworkReply> m_reply;
    QTimer m_reconnect;
    QByteArray m_buffer;
    QByteArray m_event;
    QByteArray m_data;
    int m_retrySeconds = 1;
    bool m_running = true;
    bool m_accepted = false;
};
