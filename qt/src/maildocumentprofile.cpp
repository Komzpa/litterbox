#include "maildocumentprofile.h"
#include "maildocument.h"

#include <QBuffer>
#include <QCoreApplication>
#include <QQmlEngine>
#include <QtWebEngineQuick>
#include <QQuickWebEngineDownloadRequest>
#include <QWebEngineUrlRequestInfo>
#include <QWebEngineUrlRequestInterceptor>
#include <QWebEngineUrlRequestJob>
#include <QWebEngineUrlScheme>
#include <QWebEngineUrlSchemeHandler>

namespace {
class EmbeddedImagesOnly : public QWebEngineUrlRequestInterceptor
{
public:
    using QWebEngineUrlRequestInterceptor::QWebEngineUrlRequestInterceptor;
    void interceptRequest(QWebEngineUrlRequestInfo &request) override
    {
        const QUrl url = request.requestUrl();
        const bool document = request.resourceType() == QWebEngineUrlRequestInfo::ResourceTypeMainFrame &&
            url.scheme() == QStringLiteral("litterbox-mail") && url.host() == QStringLiteral("message") &&
            url.path() == QStringLiteral("/body");
        const bool image = request.resourceType() == QWebEngineUrlRequestInfo::ResourceTypeImage &&
            url.toString().startsWith(QStringLiteral("data:image/"), Qt::CaseInsensitive);
        request.block(!document && !image);
    }
};
}

class MailDocumentProfile::DocumentHandler : public QWebEngineUrlSchemeHandler
{
public:
    using QWebEngineUrlSchemeHandler::QWebEngineUrlSchemeHandler;
    QByteArray document;
    void requestStarted(QWebEngineUrlRequestJob *job) override
    {
        if (job->requestMethod() != QByteArrayLiteral("GET") || job->requestUrl().host() != QStringLiteral("message") ||
            job->requestUrl().path() != QStringLiteral("/body")) {
            job->fail(QWebEngineUrlRequestJob::RequestDenied);
            return;
        }
        auto *buffer = new QBuffer(job);
        buffer->setData(document);
        buffer->open(QIODevice::ReadOnly);
        job->reply(QByteArrayLiteral("text/html"), buffer);
    }
};

void MailDocumentProfile::initialize()
{
#ifdef Q_OS_LINUX
    // Qt's NVIDIA Vulkan-to-OpenGL texture import can paint mail solid black.
    // Rasterize the HTML only; Qt Quick still presents through the hardware GPU.
    qputenv("QTWEBENGINE_CHROMIUM_FLAGS",
            qgetenv("QTWEBENGINE_CHROMIUM_FLAGS") + QByteArrayLiteral(" --disable-gpu"));
#endif
    QCoreApplication::setAttribute(Qt::AA_ShareOpenGLContexts);
    QWebEngineUrlScheme scheme(QByteArrayLiteral("litterbox-mail"));
    scheme.setSyntax(QWebEngineUrlScheme::Syntax::Host);
    scheme.setFlags(QWebEngineUrlScheme::SecureScheme | QWebEngineUrlScheme::LocalScheme);
    QWebEngineUrlScheme::registerScheme(scheme);
    QtWebEngineQuick::initialize();
    qmlRegisterType<MailDocumentProfile>("Litterbox.Mail", 1, 0, "MailDocumentProfile");
}

MailDocumentProfile::MailDocumentProfile(QObject *parent)
    : QQuickWebEngineProfile(parent), m_handler(new DocumentHandler(this))
{
    setOffTheRecord(true);
    setHttpCacheType(QQuickWebEngineProfile::NoCache);
    setPersistentCookiesPolicy(QQuickWebEngineProfile::NoPersistentCookies);
    setUrlRequestInterceptor(new EmbeddedImagesOnly(this));
    installUrlSchemeHandler(QByteArrayLiteral("litterbox-mail"), m_handler);
    connect(this, &QQuickWebEngineProfile::downloadRequested, this,
            [](QQuickWebEngineDownloadRequest *download) { download->cancel(); });
}

QUrl MailDocumentProfile::documentUrl(const QString &html)
{
    m_handler->document = mailDocument(html).toUtf8();
    return QUrl(QStringLiteral("litterbox-mail://message/body?revision=%1").arg(++m_revision));
}
