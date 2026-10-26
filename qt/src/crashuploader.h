#pragma once

#include <QObject>
#include <QStringList>

class Api;

// Uses the enrolled Api session, never a second network/auth stack.
class CrashUploader : public QObject
{
    Q_OBJECT
public:
    explicit CrashUploader(Api &api, QObject *parent = nullptr);
    void setDirectory(const QString &directory) { m_directory = directory; }
    void setUploaderBuild(const QString &build) { m_build = build; }
    void setOnline(bool online);
    void start();
    bool ownsRequest(int id) const { return id == m_requestId || id == m_completedId; }
    static QString androidSpoolDirectory();
signals:
    void finished(int acknowledged, int failed);
private:
    void next();
    void complete(int id, int status, const QString &ack);
    Api &m_api;
    QString m_directory, m_build, m_name, m_token, m_baseUrl;
    QStringList m_queue;
    bool m_online = false, m_running = false;
    int m_requestId = -1, m_completedId = -1, m_acknowledged = 0, m_failed = 0;
};
