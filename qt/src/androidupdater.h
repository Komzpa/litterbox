#pragma once

#include <QByteArray>
#include <QObject>
#include <QString>

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

    Q_INVOKABLE void checkForUpdates();
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
};
