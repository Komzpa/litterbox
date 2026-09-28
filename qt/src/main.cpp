#include "api.h"
#include "CardStore.h"
#include "timerules.h"

#include <QDir>
#include <QFileInfo>
#include <QGuiApplication>
#include <QHash>
#include <QDebug>
#include <QJsonDocument>
#include <QJsonObject>
#include <QQmlApplicationEngine>
#include <QQmlContext>
#include <QSet>
#include <QSettings>
#include <QStandardPaths>
#include <QTimer>
#include <QDateTime>
#include <QImage>
#include <QQuickWindow>
#include <QSqlDatabase>
#include <QSqlQuery>
#include <QJsonArray>
#include <QUrl>
#include <QKeyEvent>
#include <QWheelEvent>
#include <QQuickItem>

int main(int argc, char *argv[])
{
    QGuiApplication app(argc, argv);
    app.setApplicationName(QStringLiteral("Litterbox"));
    app.setOrganizationName(QStringLiteral("Litterbox"));
    const QStringList arguments = app.arguments();
    const bool captureScenario = arguments.size() == 4 && arguments.at(1) == QStringLiteral("--capture-scenario");
    const bool captureScrollScenario = arguments.size() == 4 && arguments.at(1) == QStringLiteral("--capture-scroll-scenario");
    const bool captureMode = captureScenario || captureScrollScenario;
    if (arguments.size() != 1 && !captureMode) {
        qCritical("Usage: litterbox-qt [--capture-scenario|--capture-scroll-scenario <fixture.sqlite> <output-directory>]");
        return 2;
    }
    QString captureOutput;
    QString captureDatabase;
    if (captureMode) {
        captureDatabase = QFileInfo(arguments.at(2)).canonicalFilePath();
        if (captureDatabase.isEmpty() || !QFileInfo(captureDatabase).isFile()) {
            qCritical("Capture scenario requires an existing isolated fixture database");
            return 2;
        }
        QSqlDatabase fixture = QSqlDatabase::addDatabase(QStringLiteral("QSQLITE"), QStringLiteral("capture-fixture-validation"));
        fixture.setDatabaseName(captureDatabase);
        fixture.setConnectOptions(QStringLiteral("QSQLITE_OPEN_READONLY"));
        bool isFixture = fixture.open();
        if (isFixture) {
            {
                QSqlQuery cards(fixture);
                isFixture = cards.exec(QStringLiteral("SELECT payload FROM cards ORDER BY position"));
                if (captureScenario) {
                    const QStringList expectedIds{
                        QStringLiteral("aaaa1111-1111-4111-8111-111111111111"),
                        QStringLiteral("bbbb2222-2222-4222-8222-222222222222"),
                        QStringLiteral("cccc3333-3333-4333-8333-333333333333")};
                    const QStringList expectedTitles{
                        QStringLiteral("Fixture A — pinned (rank 1)"),
                        QStringLiteral("Fixture B — pinned (rank 2)"),
                        QStringLiteral("Fixture C — snoozable")};
                    for (int index = 0; isFixture && index < expectedIds.size(); ++index) {
                        if (!cards.next()) { isFixture = false; break; }
                        const QJsonObject card = QJsonDocument::fromJson(cards.value(0).toByteArray()).object();
                        isFixture = card.value(QStringLiteral("id")).toString() == expectedIds[index] &&
                            card.value(QStringLiteral("title")).toString() == expectedTitles[index];
                        if (index < 2)
                            isFixture = isFixture && card.value(QStringLiteral("pinned_rank")).toInt() == index + 1;
                        else
                            isFixture = isFixture && card.value(QStringLiteral("source")).toString() == QStringLiteral("reminder");
                    }
                    if (isFixture && cards.next()) isFixture = false;
                } else {
                    int count = 0;
                    while (cards.next()) ++count;
                    isFixture = isFixture && count >= 10;
                }
                QSqlQuery outbox(fixture);
                isFixture = isFixture && outbox.exec(QStringLiteral("SELECT COUNT(*) FROM outbox")) && outbox.next() && outbox.value(0).toInt() == 0;
            }
            fixture.close();
        }
        fixture = QSqlDatabase();
        QSqlDatabase::removeDatabase(QStringLiteral("capture-fixture-validation"));
        if (!isFixture) {
            qCritical("Capture scenario accepts only the untouched three-card R20/R21 fixture");
            return 2;
        }
         captureOutput = QFileInfo(arguments.at(3)).absoluteFilePath();
        if (captureOutput == QFileInfo(captureDatabase).absolutePath() || !QDir().mkpath(captureOutput) ||
            !QDir(captureOutput).entryList(QDir::AllEntries | QDir::NoDotAndDotDot).isEmpty()) {
            qCritical("Capture output directory must be empty and separate from the fixture database");
            return 2;
        }
    }

    QSettings settings;
    Api api;
    api.setBaseUrl(qEnvironmentVariable("LB_SERVER", "http://127.0.0.1:8080"));
    api.setToken(qEnvironmentVariable("LB_TOKEN", settings.value(QStringLiteral("token")).toString()));
    QObject::connect(&api, &Api::tokenChanged, &app, [&] { settings.setValue(QStringLiteral("token"), api.token()); });

    CardStore store;
    CardStore::registerQml("litterbox", 1, 0);
    const QString dataDir = QStandardPaths::writableLocation(QStandardPaths::AppDataLocation);
    if (!captureScenario && !QDir().mkpath(dataDir)) return 1;
    const QString databasePath = captureMode ? captureDatabase : QDir(dataDir).filePath(QStringLiteral("cards.sqlite"));
    if (!store.open(databasePath)) return 1;

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
    if (captureScrollScenario) {
        QTimer::singleShot(0, &app, [&] {
            auto *window = qobject_cast<QQuickWindow *>(engine.rootObjects().constFirst());
            QObject *root = engine.rootObjects().constFirst();
            auto *list = root->findChild<QQuickItem *>(QStringLiteral("inboxList"));
            if (!window || !list) { qCritical("Capture scroll proof could not find rendered inbox window/list"); app.exit(3); return; }
            window->resize(window->width(), 260);
            list->forceActiveFocus();
            QCoreApplication::processEvents();
            auto key = [&](int code) {
                QKeyEvent event(QEvent::KeyPress, code, Qt::NoModifier);
                QCoreApplication::sendEvent(list, &event);
                QCoreApplication::processEvents();
                return list->property("contentY").toReal();
            };
            auto wheel = [&](QPoint pixel, QPoint angle) {
                const QPointF position(list->mapToScene(QPointF(list->width() / 2, list->height() / 2)));
                QWheelEvent event(position, window->mapToGlobal(position.toPoint()), pixel, angle, Qt::NoButton,
                                  Qt::NoModifier, Qt::NoScrollPhase, false, Qt::MouseEventNotSynthesized,
                                  QPointingDevice::primaryPointingDevice());
                const bool sent = QCoreApplication::sendEvent(window, &event);
                QCoreApplication::processEvents();
                return list->property("contentY").toReal();
            };
            key(Qt::Key_Home);
            if (list->property("contentHeight").toReal() <= list->property("height").toReal()) { qCritical("Scroll fixture does not overflow viewport"); app.exit(4); return; }
            const qreal down = key(Qt::Key_Down);
            const qreal pageDown = key(Qt::Key_PageDown);
            const qreal up = key(Qt::Key_Up);
            const qreal end = key(Qt::Key_End);
            const qreal maximum = list->property("contentHeight").toReal() - list->property("height").toReal();
            const qreal home = key(Qt::Key_Home);
            const qreal pageUp = key(Qt::Key_PageUp);
            if (down < 48 || up >= pageDown || pageDown <= down || end < maximum - 1 || home != 0 || pageUp != 0) {
                qCritical("Rendered inbox key scroll assertion failed: down=%.1f up=%.1f pageDown=%.1f end=%.1f max=%.1f home=%.1f pageUp=%.1f", down, up, pageDown, end, maximum, home, pageUp);
                app.exit(4);
                return;
            }
            if (wheel({}, QPoint(0, -120)) <= 0) { qCritical("Angle wheel assertion failed"); app.exit(4); return; }
            key(Qt::Key_Home);
            if (wheel(QPoint(0, -4), {}) < 10) { qCritical("4px precision wheel moved less than 10px"); app.exit(4); return; }
            window->resize(3840, 2160);
            key(Qt::Key_Home);
            QCoreApplication::processEvents();
            QVariant metricsValue;
            if (!QMetaObject::invokeMethod(list, "captureLayoutMetrics", Q_RETURN_ARG(QVariant, metricsValue))) {
                qCritical("Unable to read rendered 4K inbox layout metrics");
                app.exit(4);
                return;
            }
            const QVariantMap metrics = metricsValue.toMap();
            const qreal cardWidth = metrics.value(QStringLiteral("cardWidth")).toReal();
            const qreal cardX = metrics.value(QStringLiteral("cardX")).toReal();
            const qreal actionsX = metrics.value(QStringLiteral("actionsX")).toReal();
            const qreal actionsWidth = metrics.value(QStringLiteral("actionsWidth")).toReal();
            if (!metrics.value(QStringLiteral("hasActions")).toBool() || qAbs(cardWidth - 1200) > 1 || qAbs(cardX - 1320) > 1 ||
                actionsX < cardX || actionsX + actionsWidth > cardX + cardWidth) {
                qCritical("3840px inbox layout assertion failed: cardWidth=%.1f cardX=%.1f actionsX=%.1f actionsWidth=%.1f", cardWidth, cardX, actionsX, actionsWidth);
                app.exit(4);
                return;
            }
            const QImage frame = window->grabWindow();
            if (frame.isNull() || !frame.save(QDir(captureOutput).filePath(QStringLiteral("fullscreen-4k.png")), "PNG")) { app.exit(4); return; }
            qInfo("Capture scroll/layout proof: rendered inbox scroll keys and wheels verified; at 3840x2160 card width %.0fpx, x=%.0f, actions x=%.0f inside card; fullscreen-4k.png", cardWidth, cardX, actionsX);
            app.exit(0);
        });
    } else if (captureScenario) {
        if (store.rowCount() != 3 || store.pendingOps() != 0 ||
            store.pinnedCardIds() != QStringList({
                QStringLiteral("aaaa1111-1111-4111-8111-111111111111"),
                QStringLiteral("bbbb2222-2222-4222-8222-222222222222")})) {
            qCritical("Capture fixture must contain the untouched R20/R21 three-card baseline");
            return 2;
        }
        QTimer::singleShot(0, &app, [&] {
            auto *window = qobject_cast<QQuickWindow *>(engine.rootObjects().constFirst());
            QObject *root = engine.rootObjects().constFirst();
            if (!window) { app.exit(3); return; }
            const auto saveFrame = [&](const QString &name) {
                QCoreApplication::processEvents();
                const QImage frame = window->grabWindow();
                if (frame.isNull() || !frame.save(QDir(captureOutput).filePath(name), "PNG")) {
                    qCritical("Unable to capture rendered window: %s", qPrintable(name));
                    return false;
                }
                return true;
            };
            const QString cardA = QStringLiteral("aaaa1111-1111-4111-8111-111111111111");
            auto *list = root->findChild<QQuickItem *>(QStringLiteral("inboxList"));
            if (!list) { qCritical("Capture scroll proof could not find inboxList"); app.exit(9); return; }
            window->resize(window->width(), 260);
            list->forceActiveFocus();
            QCoreApplication::processEvents();
            auto key = [&](int code) {
                QKeyEvent event(QEvent::KeyPress, code, Qt::NoModifier);
                QCoreApplication::sendEvent(list, &event);
                QCoreApplication::processEvents();
                return list->property("contentY").toReal();
            };
            auto wheel = [&](QPoint pixel, QPoint angle) {
                const QPointF position(list->width() / 2, list->height() / 2);
                QWheelEvent event(position, position, pixel, angle, Qt::NoButton, Qt::NoModifier,
                                  Qt::NoScrollPhase, false);
                QCoreApplication::sendEvent(list, &event);
                QCoreApplication::processEvents();
                return list->property("contentY").toReal();
            };
            key(Qt::Key_Home);
            if (list->property("contentHeight").toReal() <= list->property("height").toReal()) {
                qCritical("Capture scroll proof fixture does not overflow the desktop viewport"); app.exit(9); return;
            }
            const qreal down = key(Qt::Key_Down);
            if (down < 48) { qCritical("Down key did not scroll the rendered inbox"); app.exit(9); return; }
            if (key(Qt::Key_PageDown) <= down) { qCritical("PageDown did not scroll the rendered inbox"); app.exit(9); return; }
            if (key(Qt::Key_End) < list->property("contentHeight").toReal() - list->property("height").toReal() - 1) {
                qCritical("End key did not reach the rendered inbox end"); app.exit(9); return;
            }
            if (key(Qt::Key_Home) != 0 || key(Qt::Key_PageUp) != 0) {
                qCritical("Home/PageUp did not reach the rendered inbox start"); app.exit(9); return;
            }
            if (wheel({}, QPoint(0, -120)) <= 0) { qCritical("Angle wheel did not scroll the rendered inbox"); app.exit(9); return; }
            key(Qt::Key_Home);
            if (wheel(QPoint(0, -4), {}) < 10) { qCritical("High-precision wheel delta moved less than 10 px"); app.exit(9); return; }
            if (!saveFrame(QStringLiteral("scroll.png"))) { app.exit(9); return; }
            qInfo("Capture scroll proof: rendered QML Home/End/PageUp/PageDown/Down, angle wheel and 4px precision wheel verified");
            window->resize(520, 800);
            QCoreApplication::processEvents();
            const QString cardB = QStringLiteral("bbbb2222-2222-4222-8222-222222222222");
            const QString cardC = QStringLiteral("cccc3333-3333-4333-8333-333333333333");
            if (!saveFrame(QStringLiteral("baseline.png"))) { app.exit(4); return; }

            const QString snoozeUntil = QDateTime::currentDateTime().addDays(1).toString(QStringLiteral("yyyy-MM-dd hh:mm"));
            QVariant snoozeResult;
            const bool snoozeInvoked = QMetaObject::invokeMethod(root, "captureSnooze", Q_RETURN_ARG(QVariant, snoozeResult),
                Q_ARG(QVariant, QVariant(cardC)), Q_ARG(QVariant, QVariant(snoozeUntil)));
            if (!snoozeInvoked || snoozeResult.toString() != QStringLiteral("accepted")) {
                qCritical("Capture snooze action failed: invoked=%d result=%s rows=%d",
                    snoozeInvoked, qPrintable(snoozeResult.toString()), store.rowCount());
                app.exit(5);
                return;
            }
            if (store.rowCount() != 2) {
                qCritical("Capture snooze action returned accepted but left %d cards", store.rowCount());
                app.exit(6);
                return;
            }
            if (!saveFrame(QStringLiteral("snoozed.png"))) { app.exit(6); return; }

            QVariant pinResult;
            const bool pinInvoked = QMetaObject::invokeMethod(root, "capturePinMoveUp", Q_RETURN_ARG(QVariant, pinResult),
                Q_ARG(QVariant, QVariant(cardB)));
            if (!pinInvoked || pinResult.toString() != QStringLiteral("moved") ||
                store.pinnedCardIds() != QStringList({cardB, cardA})) {
                qCritical("Capture pin reorder failed: invoked=%d result=%s pins=%s",
                    pinInvoked, qPrintable(pinResult.toString()), qPrintable(store.pinnedCardIds().join(QStringLiteral(","))));
                app.exit(7);
                return;
            }
            if (!saveFrame(QStringLiteral("reordered.png"))) { app.exit(7); return; }

            QSqlDatabase proof = QSqlDatabase::addDatabase(QStringLiteral("QSQLITE"), QStringLiteral("capture-proof"));
            proof.setDatabaseName(captureDatabase);
            proof.setConnectOptions(QStringLiteral("QSQLITE_OPEN_READONLY"));
            if (!proof.open()) { app.exit(8); return; }
            QSqlQuery cards(proof);
            if (!cards.exec(QStringLiteral("SELECT payload FROM cards ORDER BY position"))) { app.exit(8); return; }
            QStringList persistedCards;
            while (cards.next()) {
                const QJsonObject card = QJsonDocument::fromJson(cards.value(0).toByteArray()).object();
                persistedCards.append(card.value(QStringLiteral("id")).toString());
            }
            if (persistedCards != QStringList({cardB, cardA})) { app.exit(8); return; }
            QSqlQuery operations(proof);
            if (!operations.exec(QStringLiteral("SELECT payload FROM outbox ORDER BY seq"))) { app.exit(8); return; }
            int snoozes = 0, reorders = 0, operationCount = 0;
            while (operations.next()) {
                ++operationCount;
                const QJsonObject operation = QJsonDocument::fromJson(operations.value(0).toByteArray()).object();
                const QString type = operation.value(QStringLiteral("type")).toString();
                if (type == QStringLiteral("snooze") && operation.value(QStringLiteral("card_id")).toString() == cardC &&
                    !operation.value(QStringLiteral("args")).toObject().value(QStringLiteral("until")).toString().isEmpty()) ++snoozes;
                if (type == QStringLiteral("reorder_pins") &&
                    operation.value(QStringLiteral("args")).toObject().value(QStringLiteral("cards")).toArray() ==
                        QJsonArray{cardB, cardA}) ++reorders;
            }
            if (operationCount != 2 || snoozes != 1 || reorders != 1) { app.exit(8); return; }
            proof.close();
            qInfo("Capture proof: baseline.png, snoozed.png, reordered.png; persisted snooze and pin order verified");
            app.exit(0);
        });
    } else {
        store.setOnline(true);
        openStream();
    }
    return app.exec();
}
