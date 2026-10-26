#include "crashuploader.h"
#include "api.h"

#include <QDir>
#include <QFile>
#include <QJsonObject>
#include <QRegularExpression>
#include <QSaveFile>
#include <QTimer>
#ifdef Q_OS_ANDROID
#include <QCoreApplication>
#include <QJniObject>
#endif

CrashUploader::CrashUploader(Api &api, QObject *parent) : QObject(parent), m_api(api)
{
    connect(&api, &Api::tokenChanged, this, &CrashUploader::start);
    connect(&api, &Api::baseUrlChanged, this, &CrashUploader::start);
    connect(&api, &Api::requestFinished, this, [this](int id, const QJsonValue &data, int status) {
        complete(id, status, data.toObject().value(QStringLiteral("ack")).toString());
    });
    connect(&api, &Api::requestFailed, this, [this](int id, int, const QString &, const QJsonValue &) {
        complete(id, 0, {});
    });
}

QString CrashUploader::androidSpoolDirectory()
{
#ifdef Q_OS_ANDROID
    const QJniObject context = QNativeInterface::QAndroidApplication::context();
    if (!context.isValid()) return {};
    const QJniObject directory = context.callObjectMethod("getNoBackupFilesDir", "()Ljava/io/File;");
    if (!directory.isValid()) return {};
    return directory.callObjectMethod("getAbsolutePath", "()Ljava/lang/String;").toString() + QStringLiteral("/crashes");
#else
    return {};
#endif
}

void CrashUploader::setOnline(bool online)
{
    m_online = online;
    if (online) start();
}

void CrashUploader::start()
{
    if (m_running || !m_online || m_directory.isEmpty() || m_api.token().isEmpty() || m_api.baseUrl().isEmpty()) return;
    m_queue = QDir(m_directory).entryList({QStringLiteral("*.txt")}, QDir::Files, QDir::Name);
    if (m_queue.size() > 64) m_queue = m_queue.mid(0, 64);
    m_acknowledged = m_failed = 0;
    m_running = true;
    QTimer::singleShot(0, this, &CrashUploader::next);
}

void CrashUploader::next()
{
    if (!m_online || m_api.token().isEmpty() || m_api.baseUrl().isEmpty()) m_queue.clear();
    if (m_queue.isEmpty()) {
        m_running = false;
        emit finished(m_acknowledged, m_failed);
        return;
    }
    m_name = m_queue.takeFirst();
    const QString id = m_name.chopped(4);
    static const QRegularExpression validId(QStringLiteral("^[a-z0-9-]{1,96}$"));
    QFile file(QDir(m_directory).filePath(m_name));
    if (!validId.match(id).hasMatch() || !file.open(QIODevice::ReadOnly) || file.size() <= 0 || file.size() > 65536) {
        ++m_failed;
        QTimer::singleShot(0, this, &CrashUploader::next);
        return;
    }
    // A committed tombstone is authoritative even if an earlier unlink failed.
    if (QFile::exists(QDir(m_directory).filePath(id + QStringLiteral(".sent")))) {
        file.close();
        QFile::remove(file.fileName());
        QTimer::singleShot(0, this, &CrashUploader::next);
        return;
    }
    const QByteArray body = file.readAll();
    if (file.error() != QFile::NoError) {
        ++m_failed;
        QTimer::singleShot(0, this, &CrashUploader::next);
        return;
    }
    m_token = m_api.token();
    m_baseUrl = m_api.baseUrl();
    m_requestId = m_api.post(QStringLiteral("/v1/diagnostics/crashes"),
        {{QStringLiteral("report_id"), id}, {QStringLiteral("body"), QString::fromUtf8(body)},
         {QStringLiteral("uploader_build"), m_build}});
}

void CrashUploader::complete(int id, int status, const QString &ack)
{
    if (id != m_requestId) return;
    m_completedId = id; // Other Api consumers can ignore this diagnostic response in this signal turn.
    m_requestId = -1;
    if (status == 200 && ack == m_name.chopped(4) && m_token == m_api.token() && m_baseUrl == m_api.baseUrl()) {
        QDir directory(m_directory);
        QSaveFile tombstone(directory.filePath(ack + QStringLiteral(".sent")));
        if (tombstone.open(QIODevice::WriteOnly) && tombstone.commit()) {
            directory.remove(m_name);
            ++m_acknowledged;
            const QStringList sent = directory.entryList({QStringLiteral("*.sent")}, QDir::Files, QDir::Time | QDir::Reversed);
            for (int index = 0; index < sent.size() - 256; ++index) directory.remove(sent.at(index));
        } else ++m_failed;
    } else ++m_failed; // Includes 422, wrong ACK, auth rotation and all transport failures.
    // Retain the batch after any rejected receipt; retry via the existing
    // startup/reconnect/periodic entry points, not an immediate failure burst.
    if (m_failed > 0) m_queue.clear();
    QTimer::singleShot(0, this, &CrashUploader::next);
}
