#include "androidupdater.h"
#include "api.h"
#include "CardStore.h"
#include "timerules.h"
#ifndef Q_OS_ANDROID
#include "maildocumentprofile.h"
#include "updatewatcher.h"
#endif

#include <QDir>
#include <QCryptographicHash>
#include <QFile>
#include <QFileInfo>
#include <QGuiApplication>
#include <QIcon>
#include <QHash>
#include <QDebug>
#include <QJsonDocument>
#include <QJsonObject>
#include <QQmlApplicationEngine>
#include <QQmlContext>
#include <QSet>
#include <QSettings>
#include <QStandardPaths>
#include <QElapsedTimer>
#include <QTimer>
#include <QThread>
#include <QDateTime>
#include <QImage>
#include <QQuickWindow>
#include <QSize>
#include <QSqlDatabase>
#include <QSqlQuery>
#include <QJsonArray>
#include <QHoverEvent>
#include <QEventLoop>
#include <QQmlExpression>
 #include <QUrl>
 #include <QKeyEvent>
 #include <QMouseEvent>
#include <QQmlExpression>
 #include <QWheelEvent>
 #include <QQuickItem>

#ifdef Q_OS_ANDROID
#include <QJniObject>
#include <QtCore/qcoreapplication_platform.h>
#endif
#include <memory>
// QObject::findChild cannot see ListView delegates in Qt 6: they only exist as
// visual children of the list's content item (probe: declared item found,
// delegate/handle NULL). The capture therefore walks the visual tree, the same
// walk InboxView.captureLayoutMetrics already does from the list content item.
static QQuickItem *findVisualItem(QQuickItem *root, const QString &objectName)
{
    if (!root) return nullptr;
    if (root->objectName() == objectName) return root;
    const QList<QQuickItem *> children = root->childItems();
    for (QQuickItem *child : children) {
        if (QQuickItem *found = findVisualItem(child, objectName)) return found;
    }
    return nullptr;
}
static void findAllVisualItems(QQuickItem *root, const QString &objectName, QList<QQuickItem *> &out)
{
    if (!root) return;
    if (root->objectName() == objectName) out.append(root);
    const QList<QQuickItem *> children = root->childItems();
    for (QQuickItem *child : children) findAllVisualItems(child, objectName, out);
}
static QQuickItem *pickVisibleScriptItem(QQuickItem *root, const QString &objectName, int index, QQuickWindow *window)
{
    QList<QQuickItem *> all;
    findAllVisualItems(root, objectName, all);
    if (all.isEmpty()) {
        QObject *declared = window ? window->contentItem()->findChild<QObject *>(objectName) : nullptr;
        if (QQuickItem *item = qobject_cast<QQuickItem *>(declared)) all.append(item);
    }
    if (all.isEmpty()) return nullptr;
    QList<QQuickItem *> visible;
    const QRectF frame(0, 0, window ? window->width() : 0, window ? window->height() : 0);
    for (QQuickItem *item : all) {
        if (!item || !item->isVisible() || item->width() <= 0 || item->height() <= 0 || item->opacity() <= 0.01) continue;
        const QPointF topLeft = item->mapToScene(QPointF(0, 0));
        if (window && !QRectF(topLeft, QSizeF(item->width(), item->height())).intersects(frame)) continue;
        visible.append(item);
    }
    if (visible.isEmpty()) return nullptr;
    if (index < 0 || index >= visible.size()) index = 0;
    return visible.at(index);
}
static void settleScriptWindow(QQuickWindow *window)
{
    QCoreApplication::processEvents();
    if (window) {
        QEventLoop frameWait;
        QObject::connect(window, &QQuickWindow::frameSwapped, &frameWait, &QEventLoop::quit);
        QTimer::singleShot(800, &frameWait, &QEventLoop::quit);
        frameWait.exec();
        QThread::msleep(150);
    } else {
        QThread::msleep(150);
    }
    QCoreApplication::processEvents();
}
static int scriptStackDepth(QObject *root)
{
    if (!root) return -1;
    QObject *stack = root->findChild<QObject *>(QStringLiteral("pageStack"));
    return stack ? stack->property("depth").toInt() : -1;
}

#ifdef Q_OS_ANDROID
static int enrollFromFile(QGuiApplication &app, Api &api, QSettings &settings,
                          const QJniObject &activity)
{
    // No CardStore or QML engine exists in this process. Keep all request state
    // alive until the asynchronous enrollment and authenticated probe finish.
    int enrollmentRequest = 0;
    int cardsRequest = 0;
    QString enrollmentPath;
    QString enrolledDeviceId;
    const auto fail = [&app](int status) {
        qWarning() << "ENROLL_FILE status=" << status;
        app.exit(1);
    };
    QObject::connect(&api, &Api::requestFailed, &app,
        [&](int id, int status, const QString &) {
            if (id == enrollmentRequest || id == cardsRequest) fail(status);
        });
    QObject::connect(&api, &Api::requestFinished, &app,
        [&](int id, const QJsonValue &data, int status) {
            if (id == cardsRequest) {
                const QJsonObject sections = data.toObject();
                qsizetype count = 0;
                for (const QString &section : {QStringLiteral("now"), QStringLiteral("later"), QStringLiteral("missed")}) {
                    if (status != 200 || !sections.value(section).isArray()) {
                        fail(status);
                        return;
                    }
                    count += sections.value(section).toArray().size();
                }
                qInfo() << "ENROLL_FILE cards_status=" << status << "card_count=" << count
                        << "server=" << api.baseUrl() << "device_id=" << enrolledDeviceId
                        << "token_sha256=" << QCryptographicHash::hash(
                            api.token().toUtf8(), QCryptographicHash::Sha256).toHex();
                app.exit(0);
                return;
            }
            if (id != enrollmentRequest) return;
            const QJsonObject reply = data.toObject();
            const QString token = reply.value(QStringLiteral("token")).toString();
            const QString deviceId = reply.value(QStringLiteral("device_id")).toString();
            const QString tenantId = reply.value(QStringLiteral("tenant_id")).toString();
            if (status != 201 || token.trimmed().isEmpty() || deviceId.trimmed().isEmpty() || tenantId.trimmed().isEmpty()) {
                fail(status);
                return;
            }
            // Use the normal settings owner, but do not persist the temporary
            // network endpoint or token until the complete reply is validated.
            const bool hadToken = settings.contains(QStringLiteral("token"));
            const bool hadServer = settings.contains(QStringLiteral("server_url"));
            const QVariant oldToken = settings.value(QStringLiteral("token"));
            const QVariant oldServer = settings.value(QStringLiteral("server_url"));
            settings.setValue(QStringLiteral("token"), token);
            settings.setValue(QStringLiteral("server_url"), api.baseUrl());
            settings.sync();
            if (settings.status() != QSettings::NoError) {
                if (hadToken) settings.setValue(QStringLiteral("token"), oldToken);
                else settings.remove(QStringLiteral("token"));
                if (hadServer) settings.setValue(QStringLiteral("server_url"), oldServer);
                else settings.remove(QStringLiteral("server_url"));
                settings.sync();
                fail(0);
                return;
            }
            api.setToken(token);
            enrolledDeviceId = deviceId;
            const bool fileRemoved = QFile::remove(enrollmentPath);
            qInfo() << "ENROLL_FILE server=" << api.baseUrl() << "device_id=" << deviceId
                    << "token_saved=" << true << "file_removed=" << fileRemoved;
            cardsRequest = api.get(QStringLiteral("/v1/cards"));
        });
    QTimer::singleShot(30000, &app, [&] { fail(0); });
    QTimer::singleShot(0, &app, [&] {
        if (!api.token().isEmpty() || !settings.value(QStringLiteral("token")).toString().isEmpty()) {
            fail(0);
            return;
        }
        const QJniObject directory = activity.callObjectMethod(
            "getExternalFilesDir", "(Ljava/lang/String;)Ljava/io/File;", jobject(nullptr));
        if (!directory.isValid()) {
            fail(0);
            return;
        }
        const QJniObject absolutePath = directory.callObjectMethod(
            "getAbsolutePath", "()Ljava/lang/String;");
        if (!absolutePath.isValid() || absolutePath.toString().isEmpty()) {
            fail(0);
            return;
        }
        enrollmentPath = QDir(absolutePath.toString()).filePath(QStringLiteral("enroll.json"));
        QFile file(enrollmentPath);
        if (!file.open(QIODevice::ReadOnly)) {
            fail(0);
            return;
        }
        QJsonParseError parseError;
        const QJsonDocument document = QJsonDocument::fromJson(file.readAll(), &parseError);
        file.close();
        const QJsonObject enrollment = document.object();
        const QUrl server(enrollment.value(QStringLiteral("server")).toString());
        const QString invite = enrollment.value(QStringLiteral("invite_code")).toString();
        const QString name = enrollment.value(QStringLiteral("device_name")).toString();
        const QString platform = enrollment.value(QStringLiteral("platform")).toString();
        if (parseError.error != QJsonParseError::NoError || !document.isObject() ||
            !server.isValid() || server.scheme() != QStringLiteral("https") || server.host().isEmpty() ||
            !server.userInfo().isEmpty() || server.hasQuery() || server.hasFragment() ||
            invite.trimmed().isEmpty() || name.trimmed().isEmpty() || platform.trimmed().isEmpty()) {
            fail(0);
            return;
        }
        QString baseUrl = server.toString();
        while (baseUrl.endsWith(QLatin1Char('/'))) baseUrl.chop(1);
        api.setBaseUrl(baseUrl);
        enrollmentRequest = api.post(QStringLiteral("/v1/devices/enroll"), {
            {QStringLiteral("invite_code"), invite},
            {QStringLiteral("device_name"), name},
            {QStringLiteral("platform"), platform}});
    });
    const int result = app.exec();
    QNativeInterface::QAndroidApplication::runOnAndroidMainThread([activity] {
        activity.callMethod<void>("finish");
    });
    return result;
}
#endif

#ifndef LB_DEFAULT_SERVER_URL
// Compile-time fallback only; the real value comes from the CMake cache
// variable LB_DEFAULT_SERVER_URL. Never localhost: Android has no LB_SERVER
// and would otherwise talk to itself.
#define LB_DEFAULT_SERVER_URL ""
#endif
int main(int argc, char *argv[])
{
#ifndef Q_OS_ANDROID
    MailDocumentProfile::initialize();
#endif
    QGuiApplication app(argc, argv);
    app.setApplicationName(QStringLiteral("Litterbox"));
    app.setOrganizationName(QStringLiteral("Litterbox"));
    // UI stall meter (opt-in via LITTERBOX_STALL_LOG=1, off by default):
    // a precise 16 ms heartbeat on the GUI thread; any gap >100 ms means the
    // event loop was blocked and is logged with a wall-clock timestamp.
    QElapsedTimer uiHeartbeat;
    QTimer uiHeartbeatTimer;
    if (qEnvironmentVariableIsSet("LITTERBOX_STALL_LOG")) {
        uiHeartbeat.start();
        uiHeartbeatTimer.setTimerType(Qt::PreciseTimer);
        uiHeartbeatTimer.setInterval(16);
        QObject::connect(&uiHeartbeatTimer, &QTimer::timeout, &app, [&, last = qint64(0)]() mutable {
            const qint64 now = uiHeartbeat.elapsed();
            if (last != 0 && now - last > 100)
                qInfo("UI_STALL %lld ms at %s", now - last,
                      qPrintable(QDateTime::currentDateTime().toString(Qt::ISODateWithMs)));
            last = now;
        });
        uiHeartbeatTimer.start();
    }
    // Bundled Breeze subset for named icons. Android ships no system Breeze
    // theme, so the APK carries the SVGs it needs as a qrc icon theme named
    // "litterbox" (qt/icons/litterbox, LGPL-3.0-or-later, see LICENSE.breeze).
    // Desktop keeps its system theme and falls back to the same bundle, so
    // both render the same glyphs.
    QIcon::setThemeSearchPaths(QIcon::themeSearchPaths() << QStringLiteral(":/icons"));
    QIcon::setFallbackSearchPaths(QIcon::fallbackSearchPaths() << QStringLiteral(":/icons"));
    QIcon::setFallbackThemeName(QStringLiteral("litterbox"));
#ifdef Q_OS_ANDROID
    QIcon::setThemeName(QStringLiteral("litterbox"));
#endif
    const QStringList arguments = app.arguments();
    const bool captureScenario = arguments.size() == 4 && arguments.at(1) == QStringLiteral("--capture-scenario");
    const bool captureScrollScenario = arguments.size() == 4 && arguments.at(1) == QStringLiteral("--capture-scroll-scenario");
    const bool captureCardControls = (arguments.size() == 4 ||
        (arguments.size() == 7 && arguments.at(4) == QStringLiteral("--capture-viewport"))) &&
        arguments.at(1) == QStringLiteral("--capture-card-controls");
    const bool captureMode = captureScenario || captureScrollScenario || captureCardControls;
    const bool testProfileMode = arguments.size() == 3 && arguments.at(1) == QStringLiteral("--test-profile");
    const bool captureScriptMode = arguments.size() == 5 && arguments.at(1) == QStringLiteral("--capture-script");
    const bool profileMode = testProfileMode || captureScriptMode;
    if (arguments.size() != 1 && !captureMode && !testProfileMode && !captureScriptMode) {
        qCritical("Usage: litterbox-qt [--capture-scenario|--capture-scroll-scenario|--capture-card-controls <fixture.sqlite> <output-directory> [--capture-viewport <width> <height>]|--test-profile <isolated-profile-directory>|--capture-script <test-profile-dir> <script.json> <output-dir>]\nCapture-script steps (JSON array, run in order, scene settles after each): {\"resize\":[w,h]} {\"shot\":\"name\"} {\"click\":\"objectName\"[,\"index\":n]} {\"rightClick\":\"objectName\"} {\"hover\":\"objectName\"} {\"key\":\"Escape|Tab|Return|Delete|End|Home|PageDown\"} {\"mouseButton\":\"Back\"} {\"openCard\":\"card-id\"} {\"scroll\":\"end|top\"|pixels} {\"wait\":ms} {\"eval\":\"qml expression\"}. Shots write <output-dir>/name.png plus name.json with window size and stack depth. Unknown objectName fails the step with exit 3. Prefer QT_QPA_PLATFORM=offscreen; QtWebEngine mail bodies render blank there, so rerun mail-body shots under an owned Xvfb with QT_QPA_PLATFORM=xcb (never the live desktop).");
        return 2;
    }
    QSize captureViewport;
    if (captureCardControls && arguments.size() == 7) {
        bool widthOk = false, heightOk = false;
        const int width = arguments.at(5).toInt(&widthOk);
        const int height = arguments.at(6).toInt(&heightOk);
        if (!widthOk || !heightOk || width <= 0 || height <= 0) {
            qCritical("Capture viewport requires positive integer width and height");
            return 2;
        }
        captureViewport = QSize(width, height);
    }
    QString captureOutput;
    QString captureDatabase;
    QString captureScriptPath;
    QString captureScriptOutput;
    QString testProfileDirectory;
    QString testServerUrl;
    QString testToken;
    if (profileMode) {
        testProfileDirectory = QFileInfo(arguments.at(2)).canonicalFilePath();
        const QString profileConfigPath = QDir(testProfileDirectory).filePath(QStringLiteral("profile.json"));
        QFile profileConfig(profileConfigPath);
        if (testProfileDirectory.isEmpty() || !QFileInfo(testProfileDirectory).isDir() || !profileConfig.open(QIODevice::ReadOnly)) {
            qCritical("Test profile requires an existing directory containing profile.json");
            return 2;
        }
        const QJsonObject profile = QJsonDocument::fromJson(profileConfig.readAll()).object();
        const QUrl serverUrl(profile.value(QStringLiteral("server_url")).toString());
        testServerUrl = serverUrl.toString();
        testToken = profile.value(QStringLiteral("token")).toString();
        if ((serverUrl.scheme() != QStringLiteral("http") && serverUrl.scheme() != QStringLiteral("https")) ||
            serverUrl.host().isEmpty() || testToken.isEmpty()) {
            qCritical("Test profile profile.json requires an HTTP(S) server_url and non-empty token");
            return 2;
        }
        const QString testDatabasePath = QDir(testProfileDirectory).filePath(QStringLiteral("cards.sqlite"));
        const QString testSettingsPath = QDir(testProfileDirectory).filePath(QStringLiteral("settings.ini"));
        QSettings normalSettings;
        const QString normalDatabasePath = QDir(QStandardPaths::writableLocation(QStandardPaths::AppDataLocation)).filePath(QStringLiteral("cards.sqlite"));
        const auto canonicalOrAbsolute = [](const QString &path) {
            const QFileInfo info(path);
            const QString canonical = info.canonicalFilePath();
            return canonical.isEmpty() ? info.absoluteFilePath() : canonical;
        };
        if (canonicalOrAbsolute(testDatabasePath) == canonicalOrAbsolute(normalDatabasePath) ||
            canonicalOrAbsolute(testSettingsPath) == canonicalOrAbsolute(normalSettings.fileName())) {
            qCritical("Test profile storage must not overlap normal application storage");
            return 2;
        }
    }
    if (captureScriptMode) {
        captureScriptPath = QFileInfo(arguments.at(3)).canonicalFilePath();
        captureScriptOutput = QFileInfo(arguments.at(4)).absoluteFilePath();
        if (captureScriptPath.isEmpty() || !QFileInfo(captureScriptPath).isFile()) {
            qCritical("Capture script requires an existing JSON script file");
            return 2;
        }
        if (captureScriptOutput == testProfileDirectory ||
            !QDir().mkpath(captureScriptOutput) ||
            !QDir(captureScriptOutput).entryList(QDir::AllEntries | QDir::NoDotAndDotDot).isEmpty()) {
            qCritical("Capture script output directory must be empty and separate from the test profile");
            return 2;
        }
    }
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
                if (captureCardControls) {
                    // R38 fixture: two pinned manual cards, two untimed manual
                    // cards, one time-anchored card, and an empty outbox.
                    QList<QJsonObject> fixtures;
                    while (cards.next()) fixtures.append(QJsonDocument::fromJson(cards.value(0).toByteArray()).object());
                    isFixture = isFixture && fixtures.size() == 5 &&
                        fixtures.at(0).value(QStringLiteral("pinned_rank")).toInt() == 1 &&
                        fixtures.at(1).value(QStringLiteral("pinned_rank")).toInt() == 2 &&
                        fixtures.at(2).value(QStringLiteral("source")).toString() == QStringLiteral("manual") &&
                        fixtures.at(3).value(QStringLiteral("source")).toString() == QStringLiteral("manual") &&
                        fixtures.at(4).value(QStringLiteral("timed")).toBool();
                } else if (captureScenario) {
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
                        if (index < 2) isFixture = isFixture && card.value(QStringLiteral("pinned_rank")).toInt() == index + 1;
                        else isFixture = isFixture && card.value(QStringLiteral("source")).toString() == QStringLiteral("reminder");
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

    std::unique_ptr<QSettings> settings;
    if (profileMode) {
        settings = std::make_unique<QSettings>(QDir(testProfileDirectory).filePath(QStringLiteral("settings.ini")), QSettings::IniFormat);
        settings->setValue(QStringLiteral("token"), testToken);
        settings->sync();
        if (settings->status() != QSettings::NoError) return 1;
    } else {
        settings = std::make_unique<QSettings>();
    }
    Api api;
    // Server URL precedence: test profile > LB_SERVER env (when set and
    // non-empty) > QSettings server_url saved at enrollment > compiled
    // LB_DEFAULT_SERVER_URL default. Android has no LB_SERVER, so the
    // compiled default is what the phone uses until enrollment saves one.
    const QString compiledDefaultServerUrl = QStringLiteral(LB_DEFAULT_SERVER_URL);
    api.setBaseUrl(Api::resolveBaseUrl(profileMode, testServerUrl,
        qEnvironmentVariable("LB_SERVER"),
        settings->value(QStringLiteral("server_url")).toString(), compiledDefaultServerUrl));
    api.setToken(profileMode ? testToken : qEnvironmentVariable("LB_TOKEN", settings->value(QStringLiteral("token")).toString()));
#ifdef Q_OS_ANDROID
    const QJniObject activity = QNativeInterface::QAndroidApplication::context();
    const QJniObject activityClass = activity.isValid() ? activity.callObjectMethod(
        "getClass", "()Ljava/lang/Class;") : QJniObject();
    const QJniObject activityName = activityClass.isValid() ? activityClass.callObjectMethod(
        "getName", "()Ljava/lang/String;") : QJniObject();
    if (activityName.toString() == QStringLiteral("net.komzpa.litterbox.EnrollmentActivity"))
        return enrollFromFile(app, api, *settings, activity);
#endif
    QObject::connect(&api, &Api::tokenChanged, &app, [&] { settings->setValue(QStringLiteral("token"), api.token()); });
    QObject::connect(&api, &Api::baseUrlChanged, &app, [&] { settings->setValue(QStringLiteral("server_url"), api.baseUrl()); });
    AndroidUpdater updater;
    updater.setBaseUrl(api.baseUrl());
    updater.setToken(api.token());
    updater.setPackageId(QStringLiteral("org.qtproject.example.litterbox_qt"));
    QObject::connect(&api, &Api::baseUrlChanged, &updater, [&] { updater.setBaseUrl(api.baseUrl()); });
    QObject::connect(&api, &Api::tokenChanged, &updater, [&] { updater.setToken(api.token()); });
    updater.readInstalledVersionCode();


    CardStore store;
    CardStore::registerQml("litterbox", 1, 0);
    const QString dataDir = QStandardPaths::writableLocation(QStandardPaths::AppDataLocation);
    if (!captureMode && !QDir().mkpath(profileMode ? testProfileDirectory : dataDir)) return 1;
    const QString databasePath = captureMode ? captureDatabase :
        QDir(profileMode ? testProfileDirectory : dataDir).filePath(QStringLiteral("cards.sqlite"));
    if (!store.open(databasePath)) return 1;
    // Enrollment delivers its token through tokenChanged (the QML page sets
    // api.token on success and the connection above persists it). Use the
    // fresh token at once so the app goes online without a restart.
    QObject::connect(&api, &Api::tokenChanged, &app, [&] {
        if (!api.token().isEmpty()) { store.setOnline(true); store.refresh(); }
    });

    TimeRules timeRules;
    QQmlApplicationEngine engine;
    api.setEngine(&engine);
    engine.rootContext()->setContextProperty(QStringLiteral("api"), &api);
    engine.rootContext()->setContextProperty(QStringLiteral("updater"), &updater);
    engine.rootContext()->setContextProperty(QStringLiteral("store"), &store);
    engine.rootContext()->setContextProperty(QStringLiteral("timeRules"), &timeRules);

    // Preserve the existing v1 note/done routes. Other queued actions use
    // the server operation API, which acknowledges durable outbox entries.
    QSet<int> cardsRequests;
    QHash<int, QString> operationRequests;
    QHash<int, QString> mailBodyRequests;
    QObject::connect(&store, &CardStore::requestMailBodyGet, &app,
        [&](const QString &cardId, const QString &path) {
            mailBodyRequests.insert(api.get(path), cardId);
        });
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
            if (mailBodyRequests.contains(id)) {
                store.applyRemoteMailBody(mailBodyRequests.take(id), data.toObject().toVariantMap());
                return;
            }
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
        [&](int id, int status, const QString &error) {
            cardsRequests.remove(id);
            if (mailBodyRequests.contains(id)) store.reportMailBodyFailed(mailBodyRequests.take(id));
            if (operationRequests.contains(id)) {
                qWarning().noquote() << "Operation request failed:" << status << error;
                store.reportPostResult(operationRequests.take(id), status, {});
            }
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
#ifdef Q_OS_ANDROID
    // Android pages must never resolve to a build-host file URL.
    const QString pages = QStringLiteral("qrc:/qt/qml/litterbox/qml/pages");
#else
    const QString pages = qEnvironmentVariable("LB_PAGES", QStringLiteral(LB_SOURCE_PAGES_DIR));
#endif
    const bool qrcPages = pages.startsWith(QStringLiteral("qrc:/"));
    engine.rootContext()->setContextProperty(QStringLiteral("pagesDir"),
        qrcPages ? QUrl(pages + QStringLiteral("/")) : QUrl::fromLocalFile(pages + QStringLiteral("/")));
    QUrl pageUrl(QStringLiteral("qrc:/qt/qml/litterbox/qml/InboxView.qml"));
    if (!qrcPages) for (const QString &name : {QStringLiteral("Main.qml"), QStringLiteral("main.qml"),
                                QStringLiteral("Inbox.qml"), QStringLiteral("inbox.qml")}) {
        const QString path = QDir(pages).filePath(name);
        if (QFileInfo::exists(path)) { pageUrl = QUrl::fromLocalFile(path); break; }
    }
    engine.setInitialProperties({
        {QStringLiteral("api"), QVariant::fromValue(static_cast<QObject *>(&api))},
        {QStringLiteral("updater"), QVariant::fromValue(static_cast<QObject *>(&updater))},
        {QStringLiteral("store"), QVariant::fromValue(static_cast<QObject *>(&store))},
        {QStringLiteral("timeRules"), QVariant::fromValue(static_cast<QObject *>(&timeRules))}
    });
    engine.load(pageUrl);
    if (engine.rootObjects().isEmpty()) return 1;
#ifndef Q_OS_ANDROID
    // Window placement is persisted so an upgrade restart (UpdateWatcher)
    // comes back where the user left it. Capture harnesses resize their own
    // frames and must neither inherit nor record normal placement.
    if (!captureMode && !captureScriptMode) {
        auto *restoredWindow = qobject_cast<QQuickWindow *>(engine.rootObjects().constFirst());
        if (restoredWindow) {
            const QVariantMap geometry = settings->value(QStringLiteral("windowGeometry")).toMap();
            const int width = geometry.value(QStringLiteral("width")).toInt();
            const int height = geometry.value(QStringLiteral("height")).toInt();
            if (width > 0 && height > 0) {
                restoredWindow->setPosition(geometry.value(QStringLiteral("x")).toInt(),
                                            geometry.value(QStringLiteral("y")).toInt());
                restoredWindow->resize(width, height);
                if (geometry.value(QStringLiteral("maximized")).toBool()) restoredWindow->showMaximized();
            }
            QObject::connect(&app, &QCoreApplication::aboutToQuit, &app,
                [settings = settings.get(), restoredWindow] {
                    settings->setValue(QStringLiteral("windowGeometry"), QVariantMap{
                        {QStringLiteral("x"), restoredWindow->x()},
                        {QStringLiteral("y"), restoredWindow->y()},
                        {QStringLiteral("width"), restoredWindow->width()},
                        {QStringLiteral("height"), restoredWindow->height()},
                        {QStringLiteral("maximized"),
                         (restoredWindow->windowState() & Qt::WindowMaximized) != 0}});
                    settings->sync();
                });
        }
    }
#endif
    if (captureScriptMode) {
        QTimer::singleShot(0, &app, [&] {
            auto *window = qobject_cast<QQuickWindow *>(engine.rootObjects().constFirst());
            QObject *root = engine.rootObjects().constFirst();
            QQuickItem *list = findVisualItem(window ? window->contentItem() : nullptr, QStringLiteral("inboxList"));
            QFile scriptFile(captureScriptPath);
            if (!window || !list || !scriptFile.open(QIODevice::ReadOnly)) {
                qCritical("Capture script could not load window, inboxList, or script file"); app.exit(3); return;
            }
            QJsonParseError parseError;
            const QJsonDocument scriptDoc = QJsonDocument::fromJson(scriptFile.readAll(), &parseError);
            if (!scriptDoc.isArray()) {
                qCritical("Capture script must be a JSON array: %s", qPrintable(parseError.errorString())); app.exit(2); return;
            }
            const QHash<QString, int> keys{
                {QStringLiteral("Escape"), Qt::Key_Escape}, {QStringLiteral("Tab"), Qt::Key_Tab},
                {QStringLiteral("Return"), Qt::Key_Return}, {QStringLiteral("Delete"), Qt::Key_Delete},
                {QStringLiteral("End"), Qt::Key_End}, {QStringLiteral("Home"), Qt::Key_Home},
                {QStringLiteral("PageDown"), Qt::Key_PageDown}};
            const auto fail = [&](int index, const QString &why, int code = 4) {
                qCritical("Capture script step %d failed: %s", index + 1, qPrintable(why)); app.exit(code);
            };
            int stepNumber = 0;
            for (const QJsonValue &stepValue : scriptDoc.array()) {
                const int step = stepNumber++;
                const QJsonObject action = stepValue.toObject();
                bool ok = !action.isEmpty();
                if (action.contains(QStringLiteral("resize"))) {
                    const QJsonArray size = action.value(QStringLiteral("resize")).toArray();
                    ok = size.size() == 2 && size.at(0).toInt() > 0 && size.at(1).toInt() > 0;
                    if (ok) window->resize(size.at(0).toInt(), size.at(1).toInt());
                } else if (action.contains(QStringLiteral("shot"))) {
                    const QString name = QFileInfo(action.value(QStringLiteral("shot")).toString()).fileName();
                    const QString base = name.isEmpty() ? QString() : QDir(captureScriptOutput).filePath(name);
                    const QImage frame = window->grabWindow();
                    ok = !base.isEmpty() && !frame.isNull() && frame.save(base + QStringLiteral(".png"), "PNG");
                    if (ok) {
                        QFile metadata(base + QStringLiteral(".json"));
                        ok = metadata.open(QIODevice::WriteOnly | QIODevice::Truncate);
                        if (ok) {
                            const QJsonObject info{{QStringLiteral("width"), window->width()},
                                {QStringLiteral("height"), window->height()},
                                {QStringLiteral("stack_depth"), scriptStackDepth(root)}};
                            ok = metadata.write(QJsonDocument(info).toJson(QJsonDocument::Indented)) > 0;
                        }
                    }
                } else if (action.contains(QStringLiteral("setText"))) {
                    const QString objectName = action.value(QStringLiteral("setText")).toString();
                    QQuickItem *item = findVisualItem(window->contentItem(), objectName);
                    const QString text = action.value(QStringLiteral("text")).toString();
                    ok = item && item->metaObject()->indexOfProperty("text") >= 0;
                    if (ok) {
                        item->setProperty("text", text);
                        QCoreApplication::processEvents();
                    }
                } else if (action.contains(QStringLiteral("click")) || action.contains(QStringLiteral("rightClick")) || action.contains(QStringLiteral("hover"))) {
                    const QString verb = action.contains(QStringLiteral("click")) ? QStringLiteral("click") : action.contains(QStringLiteral("rightClick")) ? QStringLiteral("rightClick") : QStringLiteral("hover");
                    const QString objectName = action.value(verb).toString();
                    QQuickItem *item = pickVisibleScriptItem(window->contentItem(), objectName, action.value(QStringLiteral("index")).toInt(), window);
                    if (!item) { fail(step, QStringLiteral("unknown or non-visible objectName '%1'").arg(objectName), 3); return; }
                    const QPointF point = item->mapToScene(QPointF(item->width() / 2, item->height() / 2));
                    const bool right = verb == QStringLiteral("rightClick");
                    if (verb == QStringLiteral("hover")) {
                        QMouseEvent event(QEvent::MouseMove, point, window->mapToGlobal(point.toPoint()), Qt::NoButton, Qt::NoButton, Qt::NoModifier);
                        QCoreApplication::sendEvent(window, &event);
                    } else {
                        const Qt::MouseButton button = right ? Qt::RightButton : Qt::LeftButton;
                        QMouseEvent press(QEvent::MouseButtonPress, point, window->mapToGlobal(point.toPoint()), button, button, Qt::NoModifier);
                        QCoreApplication::sendEvent(window, &press);
                        QMouseEvent release(QEvent::MouseButtonRelease, point, window->mapToGlobal(point.toPoint()), button, Qt::NoButton, Qt::NoModifier);
                        QCoreApplication::sendEvent(window, &release);
                    }
                } else if (action.contains(QStringLiteral("key"))) {
                    const QString key = action.value(QStringLiteral("key")).toString();
                    if (!keys.contains(key)) { fail(step, QStringLiteral("unsupported key '%1'").arg(key)); return; }
                    QKeyEvent press(QEvent::KeyPress, keys.value(key), Qt::NoModifier);
                    QCoreApplication::sendEvent(window, &press);
                    QKeyEvent release(QEvent::KeyRelease, keys.value(key), Qt::NoModifier);
                    QCoreApplication::sendEvent(window, &release);
                } else if (action.contains(QStringLiteral("mouseButton"))) {
                    if (action.value(QStringLiteral("mouseButton")).toString() != QStringLiteral("Back")) { fail(step, QStringLiteral("unsupported mouseButton")); return; }
                    const QPointF point(window->width() / 2.0, window->height() / 2.0);
                    QMouseEvent press(QEvent::MouseButtonPress, point, window->mapToGlobal(point.toPoint()), Qt::BackButton, Qt::BackButton, Qt::NoModifier);
                    QCoreApplication::sendEvent(window, &press);
                    QMouseEvent release(QEvent::MouseButtonRelease, point, window->mapToGlobal(point.toPoint()), Qt::BackButton, Qt::NoButton, Qt::NoModifier);
                    QCoreApplication::sendEvent(window, &release);
                } else if (action.contains(QStringLiteral("openCard"))) {
                    const QString objectName = QStringLiteral("openCard-") + action.value(QStringLiteral("openCard")).toString();
                    QQuickItem *item = pickVisibleScriptItem(window->contentItem(), objectName, 0, window);
                    if (!item) { fail(step, QStringLiteral("unknown or non-visible objectName '%1'").arg(objectName), 3); return; }
                    const QPointF point = item->mapToScene(QPointF(item->width() / 2, item->height() / 2));
                    QMouseEvent press(QEvent::MouseButtonPress, point, window->mapToGlobal(point.toPoint()), Qt::LeftButton, Qt::LeftButton, Qt::NoModifier);
                    QCoreApplication::sendEvent(window, &press);
                    QMouseEvent release(QEvent::MouseButtonRelease, point, window->mapToGlobal(point.toPoint()), Qt::LeftButton, Qt::NoButton, Qt::NoModifier);
                    QCoreApplication::sendEvent(window, &release);
                } else if (action.contains(QStringLiteral("scroll"))) {
                    const QJsonValue value = action.value(QStringLiteral("scroll"));
                    if (value.isString() && value.toString() == QStringLiteral("top")) list->setProperty("contentY", list->property("originY"));
                    else if (value.isString() && value.toString() == QStringLiteral("end")) QMetaObject::invokeMethod(list, "positionViewAtEnd");
                    else if (value.isDouble()) list->setProperty("contentY", list->property("originY").toReal() + value.toDouble());
                    else ok = false;
                } else if (action.contains(QStringLiteral("wait"))) {
                    const int ms = action.value(QStringLiteral("wait")).toInt(-1);
                    ok = ms >= 0 && ms <= 60000;
                    if (ok) { QEventLoop wait; QTimer::singleShot(ms, &wait, &QEventLoop::quit); wait.exec(); }
                } else if (action.contains(QStringLiteral("eval"))) {
                    QQmlExpression expression(QQmlEngine::contextForObject(root), root, action.value(QStringLiteral("eval")).toString());
                    const QVariant result = expression.evaluate();
                    if (expression.hasError()) { fail(step, expression.error().toString()); return; }
                    qInfo("Capture script eval step %d: %s", step + 1, qPrintable(result.toString()));
                } else {
                    ok = false;
                }
                if (!ok) { fail(step, QStringLiteral("invalid step or operation failed")); return; }
                settleScriptWindow(window);
            }
            app.exit(0);
        });
    }
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
            // Home/End navigate via positionViewAtBeginning/End, which settle
            // over layout passes; numeric contentY alone cannot distinguish a
            // rendered card from a blank region, so the bounds assertions read
            // which card index is actually visible at the viewport edges.
            auto visibleRange = [&]() {
                QVariant value;
                if (!QMetaObject::invokeMethod(list, "captureVisibleRange", Q_RETURN_ARG(QVariant, value))) return QVariantMap{};
                return value.toMap();
            };
            auto waitVisible = [&](auto predicate) {
                QElapsedTimer timer;
                timer.start();
                QVariantMap range;
                while (timer.elapsed() < 3000) {
                    QCoreApplication::processEvents();
                    range = visibleRange();
                    if (predicate(range)) break;
                    QThread::msleep(50);
                }
                return range;
            };
            const qreal down = key(Qt::Key_Down);
            const qreal pageDown = key(Qt::Key_PageDown);
            const qreal up = key(Qt::Key_Up);
            key(Qt::Key_End);
            const int lastIndex = list->property("count").toInt() - 1;
            const QVariantMap endRange = waitVisible([&](const QVariantMap &range) { return range.value(QStringLiteral("lastVisible")).toInt() == lastIndex; });
            const qreal end = endRange.value(QStringLiteral("contentY")).toReal();
            const qreal maximum = list->property("contentHeight").toReal() - list->property("height").toReal();
            // contentHeight with variable-height delegates is an estimate until
            // every delegate has been created once, so the reachable end can sit
            // below the numeric maximum; the authoritative check is that the
            // last card is really rendered at the viewport's bottom edge.
            if (endRange.value(QStringLiteral("lastVisible")).toInt() != lastIndex || endRange.value(QStringLiteral("firstVisible")).toInt() < 0) {
                qCritical("End key did not render the last card: lastVisible=%d firstVisible=%d end=%.1f max=%.1f",
                          endRange.value(QStringLiteral("lastVisible")).toInt(), endRange.value(QStringLiteral("firstVisible")).toInt(), end, maximum);
                app.exit(4);
                return;
            }
            key(Qt::Key_Home);
            const QVariantMap homeRange = waitVisible([&](const QVariantMap &range) { return range.value(QStringLiteral("firstVisible")).toInt() == 0; });
            const qreal home = homeRange.value(QStringLiteral("contentY")).toReal();
            const qreal pageUp = key(Qt::Key_PageUp);
            if (down < 48 || up >= pageDown || pageDown <= down || homeRange.value(QStringLiteral("firstVisible")).toInt() != 0 || home > 1 || pageUp > 1) {
                qCritical("Rendered inbox key scroll assertion failed: down=%.1f up=%.1f pageDown=%.1f home=%.1f firstVisible=%d pageUp=%.1f",
                          down, up, pageDown, home, homeRange.value(QStringLiteral("firstVisible")).toInt(), pageUp);
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
    } else if (captureCardControls) {
        if (store.pendingOps() != 0 || store.rowCount() != 5 ||
            store.pinnedCardIds() != QStringList({QStringLiteral("11111111-1111-4111-8111-111111111111"), QStringLiteral("22222222-2222-4222-8222-222222222222")}) ||
            !store.data(store.index(4), CardStore::CardRole).toMap().value(QStringLiteral("timed")).toBool()) {
            qCritical("Card controls fixture must be the R38 fixture: two pinned manual cards, two untimed manual cards and one timed card, empty outbox");
            return 2;
        }
        QTimer::singleShot(0, &app, [&] {
            auto *window = qobject_cast<QQuickWindow *>(engine.rootObjects().constFirst());
            QObject *root = engine.rootObjects().constFirst();
            const auto saveFrame = [&](const QString &name) { QCoreApplication::processEvents(); const QImage image = window->grabWindow(); return !image.isNull() && image.save(QDir(captureOutput).filePath(name), "PNG"); };
            const QStringList initial = store.cardIds();
            if (!window) { app.exit(3); return; }
            if (captureViewport.isValid()) window->resize(captureViewport);
            if (!saveFrame(QStringLiteral("baseline.png"))) { app.exit(3); return; }
            QVariant dialogOpened;
            if (!QMetaObject::invokeMethod(root, "captureOpenCreateDialog", Q_RETURN_ARG(QVariant, dialogOpened), Q_ARG(QVariant, QVariant(QStringLiteral("Capture manual card"))), Q_ARG(QVariant, QVariant(QStringLiteral("Created in Qt capture")))) || !dialogOpened.toBool()) {
                qCritical("Add-card dialog did not open");
                app.exit(4);
                return;
            }
            {
                QEventLoop openWait;
                QTimer::singleShot(400, &openWait, &QEventLoop::quit);
                openWait.exec();
            }
            if (!saveFrame(QStringLiteral("create-dialog.png"))) { app.exit(4); return; }
            QVariant created;
            if (!QMetaObject::invokeMethod(root, "captureAcceptCreateDialog", Q_RETURN_ARG(QVariant, created)) || !created.toBool()) { qCritical("Add-card dialog failed"); app.exit(4); return; }
            if (!QMetaObject::invokeMethod(root, "captureCloseCreateDialog")) { app.exit(4); return; }
            QEventLoop closeWait;
            QTimer::singleShot(400, &closeWait, &QEventLoop::quit);
            closeWait.exec();
            if (!saveFrame(QStringLiteral("created-card.png"))) { app.exit(4); return; }
            const QStringList afterCreate = store.cardIds();
            if (afterCreate.size() != initial.size() + 1) { app.exit(4); return; }
            QString createdId;
            for (const QString &id : afterCreate) if (!initial.contains(id)) createdId = id;
            if (createdId.isEmpty()) { qCritical("Created card not found in the cache"); app.exit(4); return; }

            const QString pin1 = QStringLiteral("11111111-1111-4111-8111-111111111111");
            const QString pin2 = QStringLiteral("22222222-2222-4222-8222-222222222222");
            const QString manualA = QStringLiteral("33333333-3333-4333-8333-333333333333");
            const QString manualB = QStringLiteral("44444444-4444-4444-8444-444444444444");
            const QString timedCard = QStringLiteral("55555555-5555-4555-8555-555555555555");

            window->resize(560, 1200);
            QCoreApplication::processEvents();

            // Grab the rendered `=` handle with real pointer events and release
            // it over the target card's handle.
            auto dragHandleTo = [&](const QString &cardId, const QString &targetCardId) -> bool {
                QQuickItem *from = findVisualItem(window->contentItem(), QStringLiteral("reorderHandle-") + cardId);
                QQuickItem *to = findVisualItem(window->contentItem(), QStringLiteral("reorderHandle-") + targetCardId);
                if (!from || !to) {
                    qCritical("Rendered `=` handle missing for %s or %s", qPrintable(cardId), qPrintable(targetCardId));
                    return false;
                }
                const QPointF start = from->mapToScene(QPointF(from->width() / 2, from->height() / 2));
                const QPointF end = to->mapToScene(QPointF(to->width() / 2, to->height() / 2));
                auto send = [&](QEvent::Type type, const QPointF &local, Qt::MouseButtons buttons) {
                    QMouseEvent event(type, local, window->mapToGlobal(local.toPoint()), Qt::LeftButton, buttons, Qt::NoModifier);
                    QCoreApplication::sendEvent(window, &event);
                    QCoreApplication::processEvents();
                };
                send(QEvent::MouseButtonPress, start, Qt::LeftButton);
                for (int step = 1; step <= 8; ++step)
                    send(QEvent::MouseMove, start + (end - start) * (qreal(step) / 8.0), Qt::LeftButton);
                send(QEvent::MouseButtonRelease, end, Qt::NoButton);
                QCoreApplication::processEvents();
                return true;
            };
            // One real pointer press and release on a rendered control: used for the
            // no-motion `=` press and for the neighbouring Note and Done controls.
            const auto pressAndRelease = [&](const QString &itemName) -> bool {
                QQuickItem *item = findVisualItem(window->contentItem(), itemName);
                if (!item) {
                    qCritical("Rendered control missing: %s", qPrintable(itemName));
                    return false;
                }
                const QPointF point = item->mapToScene(QPointF(item->width() / 2, item->height() / 2));
                auto send = [&](QEvent::Type type, Qt::MouseButtons buttons) {
                    QMouseEvent event(type, point, window->mapToGlobal(point.toPoint()), Qt::LeftButton, buttons, Qt::NoModifier);
                    QCoreApplication::sendEvent(window, &event);
                    QCoreApplication::processEvents();
                };
                send(QEvent::MouseButtonPress, Qt::LeftButton);
                send(QEvent::MouseButtonRelease, Qt::NoButton);
                QCoreApplication::processEvents();
                return true;
            };
            const auto requireOrder = [&](const QStringList &want, const char *stage) {
                if (store.cardIds() == want) return true;
                qCritical("R38 %s order mismatch: got=%s want=%s", stage,
                    qPrintable(store.cardIds().join(QStringLiteral(","))), qPrintable(want.join(QStringLiteral(","))));
                return false;
            };

            if (!requireOrder(QStringList{pin1, pin2, createdId, manualA, manualB, timedCard}, "post-create")) { app.exit(5); return; }
            if (!dragHandleTo(manualA, createdId)) { app.exit(5); return; }
            if (!requireOrder(QStringList{pin1, pin2, manualA, createdId, manualB, timedCard}, "drag-up")) { app.exit(5); return; }
            if (!saveFrame(QStringLiteral("drag-up.png"))) { app.exit(5); return; }
            if (!dragHandleTo(manualA, manualB)) { app.exit(5); return; }
            if (!requireOrder(QStringList{pin1, pin2, createdId, manualB, manualA, timedCard}, "drag-down")) { app.exit(5); return; }
            if (!saveFrame(QStringLiteral("drag-down.png"))) { app.exit(5); return; }
            // A pinned card dragged past the block stays inside the pinned block.
            if (!dragHandleTo(pin1, timedCard)) { app.exit(6); return; }
            if (!requireOrder(QStringList{pin2, pin1, createdId, manualB, manualA, timedCard}, "pinned-clamp")) { app.exit(6); return; }
            if (!saveFrame(QStringLiteral("pinned-clamp.png"))) { app.exit(6); return; }
            // A time-anchored card refuses the drag and nothing moves.
            const QStringList beforeTimed = store.cardIds();
            if (!dragHandleTo(timedCard, pin2)) { app.exit(7); return; }
            if (!requireOrder(beforeTimed, "timed-refusal")) { app.exit(7); return; }

            QSqlDatabase proof = QSqlDatabase::addDatabase(QStringLiteral("QSQLITE"), QStringLiteral("control-capture-proof"));
            proof.setDatabaseName(captureDatabase);
            proof.setConnectOptions(QStringLiteral("QSQLITE_OPEN_READONLY"));
            if (!proof.open()) { app.exit(8); return; }
            QSqlQuery cards(proof);
            if (!cards.exec(QStringLiteral("SELECT payload FROM cards ORDER BY position"))) { app.exit(8); return; }
            QStringList persisted;
            while (cards.next()) persisted.append(QJsonDocument::fromJson(cards.value(0).toByteArray()).object().value(QStringLiteral("id")).toString());
            if (persisted != beforeTimed) {
                qCritical("Persisted card order mismatch: got=%s want=%s", qPrintable(persisted.join(QStringLiteral(","))), qPrintable(beforeTimed.join(QStringLiteral(","))));
                app.exit(8);
                return;
            }
            const auto outboxCount = [&]() -> int {
                QSqlQuery count(proof);
                if (!count.exec(QStringLiteral("SELECT COUNT(*) FROM outbox")) || !count.next()) return -1;
                return count.value(0).toInt();
            };
            struct OpInventory {
                int creates = 0, reorders = 0, pinReorders = 0, done = 0, other = 0;
                bool valid = true;
                QJsonArray pinOrder;
            };
            const auto opInventory = [&]() -> OpInventory {
                OpInventory inventory;
                QSqlQuery ops(proof);
                if (!ops.exec(QStringLiteral("SELECT payload FROM outbox ORDER BY seq"))) { inventory.valid = false; return inventory; }
                while (ops.next()) {
                    const QJsonObject op = QJsonDocument::fromJson(ops.value(0).toByteArray()).object();
                    const QString type = op.value(QStringLiteral("type")).toString();
                    if (type == QStringLiteral("create_card")) ++inventory.creates;
                    else if (type == QStringLiteral("reorder_cards")) ++inventory.reorders;
                    else if (type == QStringLiteral("reorder_pins")) { ++inventory.pinReorders; inventory.pinOrder = op.value(QStringLiteral("args")).toObject().value(QStringLiteral("cards")).toArray(); }
                    else if (type == QStringLiteral("done")) ++inventory.done;
                    else ++inventory.other;
                }
                return inventory;
            };
            const OpInventory afterDrags = opInventory();
            if (!afterDrags.valid) { app.exit(8); return; }
            if (afterDrags.creates != 1 || afterDrags.reorders != 2 || afterDrags.pinReorders != 1 || afterDrags.done != 0 || afterDrags.other != 0 || afterDrags.pinOrder != QJsonArray{pin2, pin1}) {
                qCritical("Outbox mismatch: create=%d reorder_cards=%d reorder_pins=%d done=%d other=%d", afterDrags.creates, afterDrags.reorders, afterDrags.pinReorders, afterDrags.done, afterDrags.other);
                app.exit(8);
                return;
            }

            // A press on `=` released on the same row changes nothing: the order and
            // the outbox must be exactly as they were before the press.
            const int opsBeforeNoMotion = outboxCount();
            if (opsBeforeNoMotion < 0) { app.exit(8); return; }
            if (!pressAndRelease(QStringLiteral("reorderHandle-") + manualA)) { app.exit(8); return; }
            if (!requireOrder(beforeTimed, "no-motion")) { app.exit(8); return; }
            const int opsAfterNoMotion = outboxCount();
            if (opsAfterNoMotion != opsBeforeNoMotion) {
                qCritical("No-motion press changed the outbox: before=%d after=%d", opsBeforeNoMotion, opsAfterNoMotion);
                app.exit(8);
                return;
            }
            if (!saveFrame(QStringLiteral("no-motion.png"))) { app.exit(8); return; }

            // The rendered controls beside the drag surface must stay usable under the
            // same real pointer events: Note opens its dialog, Done dismisses the card.
            // Keep the tall drag-proof viewport above, then restore the requested
            // dialog viewport before opening the lazy action sheet and Note.
            if (captureViewport.isValid()) window->resize(captureViewport);
            QCoreApplication::processEvents();
            QVariant actionsOpened;
            if (!QMetaObject::invokeMethod(root, "captureOpenActions", Q_RETURN_ARG(QVariant, actionsOpened),
                    Q_ARG(QVariant, QVariant(manualB))) || !actionsOpened.toBool()) {
                qCritical("Card action sheet did not open for %s", qPrintable(manualB));
                app.exit(9);
                return;
            }
            {
                QEventLoop actionsOpenWait;
                QTimer::singleShot(400, &actionsOpenWait, &QEventLoop::quit);
                actionsOpenWait.exec();
            }
            if (!pressAndRelease(QStringLiteral("noteButton-") + manualB)) { app.exit(9); return; }
            QVariant noteCardId;
            if (!QMetaObject::invokeMethod(root, "captureNoteDialogCardId", Q_RETURN_ARG(QVariant, noteCardId)) || noteCardId.toString() != manualB) {
                qCritical("Note control did not open its dialog for %s", qPrintable(manualB));
                app.exit(9);
                return;
            }
            {
                QEventLoop noteOpenWait;
                QTimer::singleShot(400, &noteOpenWait, &QEventLoop::quit);
                noteOpenWait.exec();
            }
            if (!saveFrame(QStringLiteral("note-dialog.png"))) { app.exit(9); return; }
            const int opsBeforeNoteClose = outboxCount();
            if (!QMetaObject::invokeMethod(root, "captureCloseNoteDialog")) { app.exit(9); return; }
            QEventLoop noteCloseWait;
            QTimer::singleShot(400, &noteCloseWait, &QEventLoop::quit);
            noteCloseWait.exec();
            if (outboxCount() != opsBeforeNoteClose) {
                qCritical("Closing the note dialog without saving enqueued an operation");
                app.exit(9);
                return;
            }
            // Done remains a pointer proof in the original tall scenario viewport.
            if (captureViewport.isValid()) window->resize(560, 1200);
            QCoreApplication::processEvents();
            if (!pressAndRelease(QStringLiteral("doneButton-") + manualB)) { app.exit(9); return; }
            QStringList afterDone = beforeTimed;
            afterDone.removeAll(manualB);
            if (store.cardIds() != afterDone) {
                qCritical("Done control did not dismiss %s: got=%s want=%s", qPrintable(manualB), qPrintable(store.cardIds().join(QStringLiteral(","))), qPrintable(afterDone.join(QStringLiteral(","))));
                app.exit(9);
                return;
            }
            if (!saveFrame(QStringLiteral("neighbors.png"))) { app.exit(9); return; }

            // Final inventory: the drags and the no-motion press queue nothing new, so
            // only the dismissal the Done control really performed is added.
            const OpInventory finalInventory = opInventory();
            if (!finalInventory.valid) { app.exit(9); return; }
            if (finalInventory.creates != 1 || finalInventory.reorders != 2 || finalInventory.pinReorders != 1 || finalInventory.done != 1 || finalInventory.other != 0 || finalInventory.pinOrder != QJsonArray{pin2, pin1}) {
                qCritical("Final outbox mismatch: create=%d reorder_cards=%d reorder_pins=%d done=%d other=%d", finalInventory.creates, finalInventory.reorders, finalInventory.pinReorders, finalInventory.done, finalInventory.other);
                app.exit(9);
                return;
            }
            proof.close();
            qInfo("Card controls capture: rendered add-card dialog, dragged `=` up and down, pinned clamp, timed refusal, no-motion press, and the Note and Done controls; rendered captures, cache and outbox verified");
            app.exit(0);
        });
    } else if (captureScenario) {
        if (store.rowCount() != 3 || store.pendingOps() != 0 ||
            store.pinnedCardIds() != QStringList({QStringLiteral("aaaa1111-1111-4111-8111-111111111111"), QStringLiteral("bbbb2222-2222-4222-8222-222222222222")})) {
            qCritical("Capture fixture must contain the untouched three-card pin fixture");
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
    } else if (!captureScriptMode) {
        store.setOnline(true);
        openStream();
    }
#ifndef Q_OS_ANDROID
    // Restart into a package-upgraded binary: the watcher spawns the
    // replacement detached first, then quits here — a normal quit, so
    // ~CardStore drains queued commits (the outbox) before the replacement
    // instance opens the same database. Capture runs are harnesses and never
    // self-restart; a build-directory binary fails createForRunningApp.
    if (!captureMode && !captureScriptMode) {
        if (UpdateWatcher *updateWatcher = UpdateWatcher::createForRunningApp(&app))
            QObject::connect(updateWatcher, &UpdateWatcher::replacementSpawned,
                             &app, &QCoreApplication::quit);
    }
#endif
    return app.exec();
}
