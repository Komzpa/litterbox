#include "api.h"
#include "CardStore.h"
#include "timerules.h"

#include <QDir>
#include <QFileInfo>
#include <QGuiApplication>
#include <QHash>
#include <QJsonObject>
#include <QQmlApplicationEngine>
#include <QQmlContext>
#include <QSet>
#include <QSettings>
#include <QStandardPaths>
#include <QTimer>
#include <QUrl>

int main(int argc, char *argv[])
{
    QGuiApplication app(argc, argv);
    app.setApplicationName(QStringLiteral("Litterbox"));
    app.setOrganizationName(QStringLiteral("Litterbox"));

    QSettings settings;
    Api api;
    api.setBaseUrl(qEnvironmentVariable("LB_SERVER", "http://127.0.0.1:8080"));
    api.setToken(qEnvironmentVariable("LB_TOKEN", settings.value(QStringLiteral("token")).toString()));
    QObject::connect(&api, &Api::tokenChanged, &app, [&] { settings.setValue(QStringLiteral("token"), api.token()); });

    CardStore store;
    CardStore::registerQml("litterbox", 1, 0);
    const QString dataDir = QStandardPaths::writableLocation(QStandardPaths::AppDataLocation);
    QDir().mkpath(dataDir);
    if (!store.open(QDir(dataDir).filePath(QStringLiteral("cards.sqlite"))))
        return 1;

    QQmlApplicationEngine engine;
    api.setEngine(&engine);
    engine.rootContext()->setContextProperty(QStringLiteral("api"), &api);
    engine.rootContext()->setContextProperty(QStringLiteral("store"), &store);
    TimeRules timeRules;
    engine.rootContext()->setContextProperty(QStringLiteral("timeRules"), &timeRules);

    // Preserve the existing v1 note/done routes. Other queued actions use
    // the server operation API, which acknowledges durable outbox entries.
    QSet<int> cardsRequests;
    QHash<int, QString> operationRequests;
    QObject::connect(&store, &CardStore::requestCards, &app, [&](const QString &path) {
        cardsRequests.insert(api.get(path));
    });
    QObject::connect(&store, &CardStore::requestPost, &app,
        [&](const QString &opId, const QString &requestPath, const QVariantMap &payload) {
            const QString cardId = payload.value(QStringLiteral("card_id")).toString();
            const QString type = payload.value(QStringLiteral("type")).toString();
            const QVariantMap args = payload.value(QStringLiteral("args")).toMap();
            QString path = QStringLiteral("/v1/cards/") + cardId;
            QJsonObject body;
            if (type == QStringLiteral("done")) {
                path += QStringLiteral("/dismiss");
                if (args.contains(QStringLiteral("note"))) body.insert(QStringLiteral("note"), args.value(QStringLiteral("note")).toString());
            } else if (type == QStringLiteral("note")) {
                path += QStringLiteral("/note");
                body.insert(QStringLiteral("text"), args.value(QStringLiteral("note")).toString());
            } else if (requestPath == QStringLiteral("/v1/ops")) {
                operationRequests.insert(api.post(requestPath, QJsonObject::fromVariantMap(payload)), opId);
                return;
            } else {
                store.reportPostResult(opId, 0, {});
                return;
            }
            operationRequests.insert(api.post(path, body), opId);
        });
    QObject::connect(&api, &Api::requestFinished, &app,
        [&](int id, const QJsonValue &data, int status) {
            if (cardsRequests.remove(id)) {
                store.applyRemoteCards(data.toObject().toVariantMap());
                return;
            }
            if (operationRequests.contains(id)) {
                QVariantMap response = data.toObject().toVariantMap();
                if (status == 204) response.insert(QStringLiteral("ok"), true);
                store.reportPostResult(operationRequests.take(id), status, response);
            }
        });
    QObject::connect(&api, &Api::requestFailed, &app,
        [&](int id, int status, const QString &) {
            cardsRequests.remove(id);
            if (operationRequests.contains(id)) store.reportPostResult(operationRequests.take(id), status, {});
            if (status == 0) store.setOnline(false);
        });

    QTimer reconnect;
    reconnect.setSingleShot(true);
    int retrySeconds = 1, streamId = -1;
    bool cardEvent = false;
    auto openStream = [&] { streamId = api.stream(QStringLiteral("/v1/cards/events")); };
    QObject::connect(&reconnect, &QTimer::timeout, &app, openStream);
    QObject::connect(&api, &Api::streamLine, &app, [&](int id, const QString &line) {
        if (id != streamId) return;
        if (line.startsWith(QStringLiteral("event:"))) cardEvent = line.mid(6).trimmed() == QStringLiteral("cards");
        if (line.isEmpty() && cardEvent) {
            cardEvent = false;
            retrySeconds = 1;
            store.setOnline(true);
            store.refresh();
        }
    });
    QObject::connect(&api, &Api::streamFinished, &app, [&](int id, const QString &) {
        if (id != streamId) return;
        streamId = -1;
        cardEvent = false;
        store.setOnline(false);
        reconnect.start(retrySeconds * 1000);
        retrySeconds = qMin(retrySeconds * 2, 10);
    });

    // Pages are owned by the QML slice; load their entrypoint if present.
    const QString pages = qEnvironmentVariable("LB_PAGES", QStringLiteral(LB_SOURCE_PAGES_DIR));
    engine.rootContext()->setContextProperty(QStringLiteral("pagesDir"), QUrl::fromLocalFile(pages + QStringLiteral("/")));
    QUrl pageUrl(QStringLiteral("qrc:/qt/qml/litterbox/qml/InboxView.qml"));
    for (const QString &name : {QStringLiteral("Main.qml"), QStringLiteral("main.qml"),
                                QStringLiteral("Inbox.qml"), QStringLiteral("inbox.qml")}) {
        const QString path = QDir(pages).filePath(name);
        if (QFileInfo::exists(path)) { pageUrl = QUrl::fromLocalFile(path); break; }
    }
    engine.load(pageUrl);
    if (engine.rootObjects().isEmpty()) return 1;
    store.setOnline(true);
    openStream();
    return app.exec();
}
