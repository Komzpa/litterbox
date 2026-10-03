#pragma once

#include <QQuickWebEngineProfile>

// One ephemeral document per view; neither mail nor its resources use the network.
class MailDocumentProfile : public QQuickWebEngineProfile
{
    Q_OBJECT
public:
    explicit MailDocumentProfile(QObject *parent = nullptr);
    Q_INVOKABLE QUrl documentUrl(const QString &html);
    // Initialize before constructing QGuiApplication, in the app and its runners.
    static void initialize();

private:
    class DocumentHandler;
    DocumentHandler *m_handler;
    quint64 m_revision = 0;
};
