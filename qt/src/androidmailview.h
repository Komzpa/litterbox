#pragma once
#include <QQuickItem>
#include <QJniObject>

// Android's native Chromium WebView occupies this item's screen rectangle.
class AndroidMailView : public QQuickItem
{
    Q_OBJECT
    Q_PROPERTY(QString html READ html WRITE setHtml NOTIFY htmlChanged)
public:
    explicit AndroidMailView(QQuickItem *parent = nullptr);
    ~AndroidMailView() override;
    QString html() const { return m_html; }
    void setHtml(const QString &html);
    static void initialize();
signals:
    void htmlChanged();
    void linkActivated(const QString &link);
private:
    void synchronize();
    void adoptView(const QJniObject &view);
    void invalidatePlacement();
    QString m_html;
    QJniObject m_view;
    QRect m_rect;
    bool m_visible = false;
    // R69: the WebView is constructed asynchronously; the Qt GUI thread never
    // waits on the Android main thread (that wait froze the UI on pause/resume).
    bool m_creating = false;
    qint64 m_id;
};
