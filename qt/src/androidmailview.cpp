#include "androidmailview.h"
#include "maildocument.h"

#include <QCoreApplication>
#include <QGuiApplication>
#include <QHash>
#include <QJniEnvironment>
#include <QQmlEngine>
#include <QQuickWindow>
#include <QtCore/qcoreapplication_platform.h>

namespace {
QHash<qint64, AndroidMailView *> views;
qint64 nextId = 0;
constexpr auto javaClass = "org/litterbox/MailView";
void linkClicked(JNIEnv *env, jclass, jlong id, jstring url)
{
    const QString link = QJniObject::fromLocalRef(env->NewLocalRef(url)).toString();
    QMetaObject::invokeMethod(QCoreApplication::instance(), [id, link] {
        if (auto *view = views.value(id)) emit view->linkActivated(link);
    }, Qt::QueuedConnection);
}
}

void AndroidMailView::initialize()
{
    QJniEnvironment env;
    const JNINativeMethod methods[] = {
        {const_cast<char *>("linkClicked"), const_cast<char *>("(JLjava/lang/String;)V"),
         reinterpret_cast<void *>(linkClicked)}
    };
    if (!env.registerNativeMethods(javaClass, methods, 1))
        qFatal("Cannot register Android mail navigation interceptor");
    qmlRegisterType<AndroidMailView>("Litterbox.Mail", 1, 0, "AndroidMailView");
}

AndroidMailView::AndroidMailView(QQuickItem *parent) : QQuickItem(parent), m_id(++nextId)
{
    views.insert(m_id, this);
    connect(this, &QQuickItem::windowChanged, this, [this](QQuickWindow *window) {
        if (window) connect(window, &QQuickWindow::afterAnimating, this, &AndroidMailView::synchronize,
                            Qt::UniqueConnection);
        synchronize();
    });
    connect(this, &QQuickItem::visibleChanged, this, &AndroidMailView::synchronize);
    // R69: hide the native view while the app is in the background and place
    // it again on return; Android may have recreated the Qt surface meanwhile,
    // so the cached rectangle is not trusted and the scene is redrawn.
    connect(qGuiApp, &QGuiApplication::applicationStateChanged, this, [this](Qt::ApplicationState state) {
        if (state == Qt::ApplicationActive) {
            invalidatePlacement();
            if (window()) window()->update();
        }
        synchronize();
    });
}

void AndroidMailView::invalidatePlacement()
{
    m_rect = QRect(-1, -1, -1, -1);
}

void AndroidMailView::adoptView(const QJniObject &view)
{
    m_creating = false;
    if (!view.isValid()) qFatal("Cannot construct Android mail WebView");
    m_view = view;
    const QString document = mailDocument(m_html);
    QNativeInterface::QAndroidApplication::runOnAndroidMainThread([view, document] {
        view.callMethod<void>("loadMail", "(Ljava/lang/String;)V", QJniObject::fromString(document).object());
    });
    invalidatePlacement();
    synchronize();
}

AndroidMailView::~AndroidMailView()
{
    views.remove(m_id);
    const auto view = m_view;
    if (view.isValid()) QNativeInterface::QAndroidApplication::runOnAndroidMainThread([view] {
        view.callMethod<void>("dispose", "()V");
    });
}

void AndroidMailView::setHtml(const QString &html)
{
    if (m_html == html) return;
    m_html = html;
    emit htmlChanged();
    synchronize();
    const auto view = m_view;
    if (view.isValid()) {
        const QString document = mailDocument(html);
        QNativeInterface::QAndroidApplication::runOnAndroidMainThread([view, document] {
            view.callMethod<void>("loadMail", "(Ljava/lang/String;)V", QJniObject::fromString(document).object());
        });
    }
}

void AndroidMailView::synchronize()
{
    const bool visible = window() && window()->isVisible() && isVisible() && width() > 0 && height() > 0
        && QGuiApplication::applicationState() == Qt::ApplicationActive;
    if (!m_view.isValid()) {
        if (!visible || m_creating) return;
        m_creating = true;
        const auto context = QNativeInterface::QAndroidApplication::context();
        const qint64 id = m_id;
        QNativeInterface::QAndroidApplication::runOnAndroidMainThread([context, id] {
            QJniObject created(javaClass, "(Landroid/content/Context;J)V", context.object(), jlong(id));
            QMetaObject::invokeMethod(QCoreApplication::instance(), [id, created] {
                if (auto *view = views.value(id)) {
                    view->adoptView(created);
                } else if (created.isValid()) {
                    QNativeInterface::QAndroidApplication::runOnAndroidMainThread([created] {
                        created.callMethod<void>("dispose", "()V");
                    });
                }
            }, Qt::QueuedConnection);
        });
        return;
    }
    QRect rect;
    if (window()) {
        const qreal scale = window()->devicePixelRatio();
        const auto position = mapToScene(QPointF(0, 0));
        rect = QRect(qRound(position.x() * scale), qRound(position.y() * scale),
                     qRound(width() * scale), qRound(height() * scale));
    }
    if (rect == m_rect && visible == m_visible) return;
    m_rect = rect;
    m_visible = visible;
    const auto view = m_view;
    QNativeInterface::QAndroidApplication::runOnAndroidMainThread([view, rect, visible] {
        view.callMethod<void>("place", "(IIIIZ)V", jint(rect.x()), jint(rect.y()),
                              jint(rect.width()), jint(rect.height()), jboolean(visible));
    });
}
