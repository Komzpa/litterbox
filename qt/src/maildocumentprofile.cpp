#include "maildocumentprofile.h"

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
    // CSP precedes the sender's markup. The final stylesheet constrains only
    // overflow; the sender's table structure, spacing and responsive CSS survive.
    m_handler->document = QByteArrayLiteral(
        "<!doctype html><html><head><meta charset='utf-8'>"
        "<meta http-equiv='Content-Security-Policy' content=\"default-src 'none'; "
        "img-src data:; style-src 'unsafe-inline'; script-src 'none'; "
        "frame-src 'none'; object-src 'none'; connect-src 'none'; base-uri 'none'; form-action 'none'\">"
        "</head><body>") + html.toUtf8() + QByteArrayLiteral(
        "<style>html{color-scheme:light;background:white}"
        "body{margin:0;overflow-wrap:anywhere;font-family:sans-serif;font-size:medium;line-height:1.45;color:#263b3a;background-color:white}"
        // max-width is forced so no image leaves the column; height stays a
        // normal-priority rule so the sender's inline height (Google's 40px
        // logo) still wins while width/height attributes keep their aspect ratio.
        "img{max-width:100%!important;height:auto}"
        "table{max-width:100%!important;min-width:0!important}"
        // Newsletter columns carry fixed min-widths sized for a 600px canvas and
        // their own media queries stop at 480px, so a ~500px card would overflow.
        "*{min-width:0!important}"
        "td,th{overflow-wrap:anywhere}"
        "pre{white-space:pre-wrap!important;overflow-wrap:anywhere}"
        "</style></body></html>");
    return QUrl(QStringLiteral("litterbox-mail://message/body?revision=%1").arg(++m_revision));
}
