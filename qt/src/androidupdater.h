#pragma once

#include <QByteArray>
#include <QObject>
#include <QString>
#include <QTimer>

class QCryptographicHash;
class QNetworkAccessManager;
class QNetworkReply;
class QNetworkRequest;
class QSaveFile;

// Downloads and verifies the Android release described by the authenticated,
// same-origin Litterbox update API. The manifest's version is the monotonic
// Android versionCode in decimal form.
class AndroidUpdater : public QObject
{
    Q_OBJECT
    Q_PROPERTY(QString baseUrl READ baseUrl WRITE setBaseUrl NOTIFY baseUrlChanged)
    Q_PROPERTY(QString token READ token WRITE setToken NOTIFY tokenChanged)
    Q_PROPERTY(QString packageId READ packageId WRITE setPackageId NOTIFY packageIdChanged)
    Q_PROPERTY(int installedVersionCode READ installedVersionCode WRITE setInstalledVersionCode NOTIFY installedVersionCodeChanged)
    Q_PROPERTY(bool busy READ busy NOTIFY busyChanged)
    Q_PROPERTY(bool supported READ supported CONSTANT)
    // R64: automatic checks run silently; `automatic` is true while one runs.
    Q_PROPERTY(bool online READ online WRITE setOnline NOTIFY onlineChanged)
    Q_PROPERTY(bool automatic READ automatic NOTIFY busyChanged)
    Q_PROPERTY(QString readyVersion READ readyVersion NOTIFY readyVersionChanged)
public:
    explicit AndroidUpdater(QObject *parent = nullptr);
    ~AndroidUpdater() override;

    QString baseUrl() const;
    void setBaseUrl(const QString &url);
    QString token() const;
    void setToken(const QString &token);
    QString packageId() const;
    void setPackageId(const QString &packageId);
    int installedVersionCode() const;
    void setInstalledVersionCode(int versionCode);
    bool busy() const;
    bool supported() const;
    bool online() const { return m_online; }
    void setOnline(bool online);
    bool automatic() const { return m_busy && m_automaticRun; }
    QString readyVersion() const { return m_readyVersion; }

    // R64 schedule: success waits kAutoSuccessIntervalMs; failures back off
    // from kAutoFailureBaseMs doubling up to the success interval.
    static constexpr qint64 kAutoSuccessIntervalMs = 6LL * 60 * 60 * 1000;
    static constexpr qint64 kAutoFailureBaseMs = 15LL * 60 * 1000;
    // Periodic timer for checks while running; 0 disables it (tests, desktop).
    void setAutoCheckPeriod(int milliseconds);
    // Test hook: a fixed wall clock in ms; negative restores the real clock.
    void setClockForTesting(qint64 nowMs) { m_clockOverrideMs = nowMs; }
    qint64 nextAutomaticCheckMs() const { return m_nextAutoCheckMs; }

    Q_INVOKABLE void checkForUpdates();
    // Start/resume/periodic trigger: silent, rate limited, skipped offline.
    Q_INVOKABLE bool checkAutomatically();
    Q_INVOKABLE bool readInstalledVersionCode();
    Q_INVOKABLE void installDownloadedUpdate();

signals:
    void baseUrlChanged();
    void tokenChanged();
    void packageIdChanged();
    void installedVersionCodeChanged();
    void busyChanged();
    void noUpdateAvailable();
    void updateReady(const QString &version, const QString &sha256);
    void installConsentRequired();
    void errorOccurred(const QString &message);
    void automaticCheckFailed(const QString &message);
    void onlineChanged();
    void readyVersionChanged();

private:
    QNetworkRequest buildRequest(const QString &path, const QByteArray &accept) const;
    void setBusy(bool value);
    void onManifestReadyRead();
    void onManifestFinished();
    void startDownload(const QString &relativePath);
    void onDownloadReadyRead();
    void onDownloadFinished();
    void finishWithError(const QString &message);
    void discardDownload();
    void discardReady();
    void finishNoUpdate();
    void finishReady();
    void scheduleAfter(bool success);
    qint64 nowMs() const;
    bool startCheck(bool automatic);

    QNetworkAccessManager *m_nam;
    QNetworkReply *m_manifestReply = nullptr;
    QNetworkReply *m_downloadReply = nullptr;
    QSaveFile *m_downloadFile = nullptr;
    QCryptographicHash *m_hasher = nullptr;
    QString m_baseUrl;
    QString m_token;
    QString m_packageId;
    int m_installedVersionCode = 0;
    bool m_busy = false;
    QString m_pendingVersion;
    QString m_pendingSha256;
    QString m_apkPath;
    QByteArray m_manifestBody;
    bool m_manifestTooLarge = false;
    qint64 m_downloadedBytes = 0;
    QString m_downloadError;
    // Verified APK kept between checks so one version is downloaded once.
    QString m_readyVersion;
    QString m_readySha256;
    QString m_readyApkPath;
    bool m_online = true;
    bool m_automaticRun = false;
    int m_autoFailures = 0;
    qint64 m_nextAutoCheckMs = 0;
    qint64 m_clockOverrideMs = -1;
    QTimer m_autoTimer;
};
