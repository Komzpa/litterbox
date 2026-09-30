#include "androidupdater.h"

#include <QCryptographicHash>
#include <QDir>
#include <QFile>
#include <QJsonDocument>
#include <QJsonObject>
#include <QNetworkAccessManager>
#include <QNetworkReply>
#include <QNetworkRequest>
#include <QRegularExpression>
#include <QSaveFile>
#include <QStandardPaths>
#include <QUrl>

#ifdef Q_OS_ANDROID
#include <QJniObject>
#include <QtCore/qcoreapplication_platform.h>
#endif

namespace {
constexpr auto kManifestPath = "/v1/android/update";
constexpr auto kDownloadPath = "/v1/android/update.apk";
constexpr auto kClientApiHeader = "X-Litterbox-Api";
constexpr qint64 kMaxManifestBytes = 64 * 1024;
constexpr qint64 kMaxApkBytes = 256LL * 1024 * 1024;

QString normalizeUrl(const QString &baseUrl)
{
    QString url = baseUrl;
    while (url.endsWith(QLatin1Char('/')))
        url.chop(1);
    return url;
}

bool validSha256(const QString &value)
{
    static const QRegularExpression pattern(QStringLiteral("\\A[0-9a-fA-F]{64}\\z"));
    return pattern.match(value).hasMatch();
}
} // namespace

AndroidUpdater::AndroidUpdater(QObject *parent)
    : QObject(parent)
    , m_nam(new QNetworkAccessManager(this))
{
}

AndroidUpdater::~AndroidUpdater()
{
    if (m_manifestReply) {
        disconnect(m_manifestReply, nullptr, this, nullptr);
        m_manifestReply->abort();
    }
    if (m_downloadReply) {
        disconnect(m_downloadReply, nullptr, this, nullptr);
        m_downloadReply->abort();
    }
    if (m_downloadFile) {
        m_downloadFile->cancelWriting();
        delete m_downloadFile;
    }
    delete m_hasher;
}

QString AndroidUpdater::baseUrl() const { return m_baseUrl; }
void AndroidUpdater::setBaseUrl(const QString &url)
{
    const QString normalized = normalizeUrl(url);
    if (m_baseUrl == normalized)
        return;
    m_baseUrl = normalized;
    emit baseUrlChanged();
}

QString AndroidUpdater::token() const { return m_token; }
void AndroidUpdater::setToken(const QString &token)
{
    if (m_token == token)
        return;
    m_token = token;
    emit tokenChanged();
}

QString AndroidUpdater::packageId() const { return m_packageId; }
void AndroidUpdater::setPackageId(const QString &packageId)
{
    if (m_packageId == packageId)
        return;
    m_packageId = packageId;
    emit packageIdChanged();
}

int AndroidUpdater::installedVersionCode() const { return m_installedVersionCode; }
void AndroidUpdater::setInstalledVersionCode(int versionCode)
{
    if (m_installedVersionCode == versionCode)
        return;
    m_installedVersionCode = versionCode;
    emit installedVersionCodeChanged();
}

bool AndroidUpdater::busy() const { return m_busy; }
bool AndroidUpdater::supported() const
{
#ifdef Q_OS_ANDROID
    return true;
#else
    return false;
#endif
}

bool AndroidUpdater::readInstalledVersionCode()
{
#ifdef Q_OS_ANDROID
    const QJniObject context = QNativeInterface::QAndroidApplication::context();
    if (!context.isValid())
        return false;
    const QJniObject packageManager = context.callObjectMethod(
        "getPackageManager", "()Landroid/content/pm/PackageManager;");
    if (!packageManager.isValid())
        return false;
    const QJniObject packageName = context.callObjectMethod(
        "getPackageName", "()Ljava/lang/String;");
    if (!packageName.isValid())
        return false;
    const QJniObject packageInfo = packageManager.callObjectMethod(
        "getPackageInfo", "(Ljava/lang/String;I)Landroid/content/pm/PackageInfo;",
        packageName.object(), jint(0));
    if (!packageInfo.isValid())
        return false;
    const int versionCode = packageInfo.getField<jint>("versionCode");
    if (versionCode <= 0)
        return false;
    setInstalledVersionCode(versionCode);
    return true;
#else
    return false;
#endif
}

QNetworkRequest AndroidUpdater::buildRequest(const QString &path, const QByteArray &accept) const
{
    QNetworkRequest request(QUrl(m_baseUrl + path));
    request.setRawHeader("Accept", accept);
    request.setRawHeader(kClientApiHeader, "1");
    request.setAttribute(QNetworkRequest::RedirectPolicyAttribute, QNetworkRequest::ManualRedirectPolicy);
    if (!m_token.isEmpty())
        request.setRawHeader("Authorization", "Bearer " + m_token.toUtf8());
    return request;
}

void AndroidUpdater::setBusy(bool value)
{
    if (m_busy == value)
        return;
    m_busy = value;
    emit busyChanged();
}

void AndroidUpdater::checkForUpdates()
{
    if (m_busy)
        return;
    if (m_baseUrl.isEmpty() || m_packageId.isEmpty() || m_token.isEmpty()) {
        emit errorOccurred(QStringLiteral("update server, package, and device token are required"));
        return;
    }
    const QUrl origin(m_baseUrl);
    if (!origin.isValid() || (origin.scheme() != QStringLiteral("https")
                              && origin.scheme() != QStringLiteral("http"))
        || origin.host().isEmpty() || !origin.userInfo().isEmpty() || !origin.query().isEmpty()
        || !origin.fragment().isEmpty()) {
        emit errorOccurred(QStringLiteral("update server URL is invalid"));
        return;
    }
    if (m_installedVersionCode <= 0) {
        emit errorOccurred(QStringLiteral("installed Android version is unavailable"));
        return;
    }

    if (!m_apkPath.isEmpty())
        QFile::remove(m_apkPath);
    m_apkPath.clear();
    m_pendingVersion.clear();
    m_pendingSha256.clear();
    m_manifestBody.clear();
    m_manifestTooLarge = false;
    setBusy(true);
    m_manifestReply = m_nam->get(buildRequest(QLatin1String(kManifestPath), "application/json"));
    connect(m_manifestReply, &QNetworkReply::readyRead, this, &AndroidUpdater::onManifestReadyRead);
    connect(m_manifestReply, &QNetworkReply::finished, this, &AndroidUpdater::onManifestFinished);
}

void AndroidUpdater::onManifestReadyRead()
{
    if (!m_manifestReply)
        return;
    const QByteArray chunk = m_manifestReply->readAll();
    if (m_manifestBody.size() + chunk.size() > kMaxManifestBytes) {
        m_manifestTooLarge = true;
        m_manifestReply->abort();
        return;
    }
    m_manifestBody.append(chunk);
}

void AndroidUpdater::onManifestFinished()
{
    if (!m_manifestReply)
        return;
    const int status = m_manifestReply->attribute(QNetworkRequest::HttpStatusCodeAttribute).toInt();
    const bool transportError = m_manifestReply->error() != QNetworkReply::NoError;
    const QByteArray lastBytes = m_manifestReply->readAll();
    if (m_manifestBody.size() + lastBytes.size() > kMaxManifestBytes)
        m_manifestTooLarge = true;
    else
        m_manifestBody.append(lastBytes);
    m_manifestReply->deleteLater();
    m_manifestReply = nullptr;

    if (m_manifestTooLarge) {
        finishWithError(QStringLiteral("update manifest exceeds size limit"));
        return;
    }
    if (transportError || status != 200) {
        if (!transportError && status == 503) {
            setBusy(false);
            emit noUpdateAvailable();
            return;
        }
        finishWithError(QStringLiteral("update manifest unavailable (HTTP %1)").arg(status));
        return;
    }

    QJsonParseError parseError{};
    const QJsonDocument document = QJsonDocument::fromJson(m_manifestBody, &parseError);
    if (parseError.error != QJsonParseError::NoError || !document.isObject()) {
        finishWithError(QStringLiteral("update manifest is not valid JSON"));
        return;
    }
    const QJsonObject manifest = document.object();
    const QString version = manifest.value(QStringLiteral("version")).toString();
    const QString package = manifest.value(QStringLiteral("package")).toString();
    const QString sha256 = manifest.value(QStringLiteral("sha256")).toString();
    const QString downloadPath = manifest.value(QStringLiteral("download_path")).toString();

    if (version.isEmpty() || package.isEmpty() || sha256.isEmpty() || downloadPath.isEmpty()) {
        finishWithError(QStringLiteral("update manifest is missing required fields"));
        return;
    }
    if (package != m_packageId) {
        finishWithError(QStringLiteral("update package identity does not match this app"));
        return;
    }
    if (!validSha256(sha256)) {
        finishWithError(QStringLiteral("update manifest SHA-256 is invalid"));
        return;
    }
    if (downloadPath != QLatin1String(kDownloadPath)) {
        finishWithError(QStringLiteral("update download path is not the fixed same-origin route"));
        return;
    }

    bool validCode = false;
    const int remoteCode = version.toInt(&validCode);
    if (!validCode || remoteCode <= 0) {
        finishWithError(QStringLiteral("update version is not a valid versionCode"));
        return;
    }
    if (remoteCode <= m_installedVersionCode) {
        setBusy(false);
        emit noUpdateAvailable();
        return;
    }

    m_pendingVersion = version;
    m_pendingSha256 = sha256.toLower();
    startDownload(downloadPath);
}

void AndroidUpdater::startDownload(const QString &relativePath)
{
    const QString dir = QStandardPaths::writableLocation(QStandardPaths::AppDataLocation);
    if (dir.isEmpty() || !QDir().mkpath(dir)) {
        finishWithError(QStringLiteral("could not create download directory"));
        return;
    }
    m_apkPath = dir + QStringLiteral("/update-%1.apk").arg(m_pendingVersion);
    m_downloadFile = new QSaveFile(m_apkPath);
    if (!m_downloadFile->open(QIODevice::WriteOnly)) {
        discardDownload();
        finishWithError(QStringLiteral("could not write update file"));
        return;
    }
    m_hasher = new QCryptographicHash(QCryptographicHash::Sha256);
    m_downloadedBytes = 0;
    m_downloadError.clear();
    m_downloadReply = m_nam->get(buildRequest(relativePath, "application/vnd.android.package-archive"));
    connect(m_downloadReply, &QNetworkReply::readyRead, this, &AndroidUpdater::onDownloadReadyRead);
    connect(m_downloadReply, &QNetworkReply::finished, this, &AndroidUpdater::onDownloadFinished);
}

void AndroidUpdater::onDownloadReadyRead()
{
    if (!m_downloadReply || !m_downloadFile || !m_hasher || !m_downloadError.isEmpty())
        return;
    const QByteArray chunk = m_downloadReply->readAll();
    if (chunk.isEmpty())
        return;
    if (m_downloadedBytes + chunk.size() > kMaxApkBytes) {
        m_downloadError = QStringLiteral("update APK exceeds size limit");
        m_downloadReply->abort();
        return;
    }
    const qint64 written = m_downloadFile->write(chunk);
    if (written != chunk.size()) {
        m_downloadError = QStringLiteral("could not write complete update APK");
        m_downloadReply->abort();
        return;
    }
    m_downloadedBytes += written;
    m_hasher->addData(chunk);
}

void AndroidUpdater::onDownloadFinished()
{
    if (!m_downloadReply)
        return;
    const int status = m_downloadReply->attribute(QNetworkRequest::HttpStatusCodeAttribute).toInt();
    const bool transportError = m_downloadReply->error() != QNetworkReply::NoError;
    if (m_downloadError.isEmpty() && m_downloadFile && m_hasher) {
        const QByteArray remainder = m_downloadReply->readAll();
        if (!remainder.isEmpty()) {
            if (m_downloadedBytes + remainder.size() > kMaxApkBytes) {
                m_downloadError = QStringLiteral("update APK exceeds size limit");
            } else if (m_downloadFile->write(remainder) != remainder.size()) {
                m_downloadError = QStringLiteral("could not write complete update APK");
            } else {
                m_downloadedBytes += remainder.size();
                m_hasher->addData(remainder);
            }
        }
    }
    m_downloadReply->deleteLater();
    m_downloadReply = nullptr;

    const QString downloadError = m_downloadError;
    const QByteArray digest = m_hasher ? m_hasher->result().toHex() : QByteArray();
    if (m_hasher) {
        delete m_hasher;
        m_hasher = nullptr;
    }
    if (!downloadError.isEmpty()) {
        discardDownload();
        finishWithError(downloadError);
        return;
    }
    if (transportError || status != 200 || m_downloadedBytes == 0) {
        discardDownload();
        finishWithError(QStringLiteral("update download failed (HTTP %1)").arg(status));
        return;
    }
    if (QString::fromLatin1(digest) != m_pendingSha256) {
        discardDownload();
        finishWithError(QStringLiteral("update checksum verification failed"));
        return;
    }
    if (!m_downloadFile || !m_downloadFile->commit()) {
        discardDownload();
        finishWithError(QStringLiteral("could not finalize update file"));
        return;
    }
    delete m_downloadFile;
    m_downloadFile = nullptr;
    setBusy(false);
    emit updateReady(m_pendingVersion, m_pendingSha256);
}

void AndroidUpdater::discardDownload()
{
    if (m_downloadFile) {
        m_downloadFile->cancelWriting();
        delete m_downloadFile;
        m_downloadFile = nullptr;
    }
    if (!m_apkPath.isEmpty())
        QFile::remove(m_apkPath);
    m_apkPath.clear();
}

void AndroidUpdater::installDownloadedUpdate()
{
    if (m_apkPath.isEmpty()) {
        emit errorOccurred(QStringLiteral("no verified update is ready to install"));
        return;
    }
#ifdef Q_OS_ANDROID
    const QJniObject context = QNativeInterface::QAndroidApplication::context();
    if (!context.isValid()) {
        emit errorOccurred(QStringLiteral("Android context unavailable"));
        return;
    }
    const QString authority = m_packageId + QStringLiteral(".qtprovider");
    QJniObject file("java/io/File", "(Ljava/lang/String;)V",
                    QJniObject::fromString(m_apkPath).object());
    QJniObject uri = QJniObject::callStaticObjectMethod(
        "androidx/core/content/FileProvider", "getUriForFile",
        "(Landroid/content/Context;Ljava/lang/String;Ljava/io/File;)Landroid/net/Uri;",
        context.object(), QJniObject::fromString(authority).object(), file.object());
    if (!uri.isValid()) {
        emit errorOccurred(QStringLiteral("could not build installer content URI"));
        return;
    }

    QJniObject packageManager = context.callObjectMethod(
        "getPackageManager", "()Landroid/content/pm/PackageManager;");
    if (QNativeInterface::QAndroidApplication::sdkVersion() >= 26 && packageManager.isValid()
        && !packageManager.callMethod<jboolean>("canRequestPackageInstalls", "()Z")) {
        QJniObject intent("android/content/Intent", "(Ljava/lang/String;)V",
                          QJniObject::fromString("android.settings.MANAGE_UNKNOWN_APP_SOURCES").object());
        QJniObject settingsUri = QJniObject::callStaticObjectMethod(
            "android/net/Uri", "parse", "(Ljava/lang/String;)Landroid/net/Uri;",
            QJniObject::fromString(QStringLiteral("package:%1").arg(m_packageId)).object());
        intent.callObjectMethod("setData", "(Landroid/net/Uri;)Landroid/content/Intent;", settingsUri.object());
        context.callMethod<void>("startActivity", "(Landroid/content/Intent;)V", intent.object());
        emit installConsentRequired();
        return;
    }

    QJniObject intent("android/content/Intent", "(Ljava/lang/String;)V",
                      QJniObject::fromString("android.intent.action.VIEW").object());
    intent.callObjectMethod("setDataAndType",
                            "(Landroid/net/Uri;Ljava/lang/String;)Landroid/content/Intent;",
                            uri.object(),
                            QJniObject::fromString("application/vnd.android.package-archive").object());
    intent.callMethod<void>("addFlags", "(I)V", jint(0x00000001)); // FLAG_GRANT_READ_URI_PERMISSION
    intent.callMethod<void>("addFlags", "(I)V", jint(0x10000000)); // FLAG_ACTIVITY_NEW_TASK
    context.callMethod<void>("startActivity", "(Landroid/content/Intent;)V", intent.object());
#else
    emit errorOccurred(QStringLiteral("in-app install is only supported on Android"));
#endif
}

void AndroidUpdater::finishWithError(const QString &message)
{
    setBusy(false);
    emit errorOccurred(message);
}
