#include "CardStore.h"
#include <QFile>
#include <QIcon>
#include <QQmlContext>
#include <QQmlEngine>
#include <QTemporaryDir>
#include <QtQuickTest/quicktest.h>

class BundleTestSetup : public QObject
{
    Q_OBJECT
public slots:
    void qmlEngineAvailable(QQmlEngine *engine)
    {
        auto *store = new CardStore(engine);
        const QString database = m_directory.filePath(QStringLiteral("cards.sqlite"));
        const QString cache = qEnvironmentVariable("LB_BUNDLE_CACHE");
        if (!m_directory.isValid()) qFatal("Cannot create disposable bundle cache");
        if (!cache.isEmpty() && (!QFile::copy(cache, database)
                || !QFile::setPermissions(database, QFileDevice::ReadOwner | QFileDevice::WriteOwner)))
            qFatal("Cannot copy writable disposable bundle cache");
        if (!store->open(database)) qFatal("Cannot open disposable bundle cache");
        engine->rootContext()->setContextProperty(QStringLiteral("bundleStore"), store);
        QVariantList cards;
        for (int row = 0; row < store->rowCount(); ++row)
            cards.append(store->data(store->index(row), CardStore::CardRole));
        engine->rootContext()->setContextProperty(QStringLiteral("bundleCachedCards"), cards);
        engine->rootContext()->setContextProperty(QStringLiteral("bundleCacheLoaded"), !cache.isEmpty());
        engine->rootContext()->setContextProperty(QStringLiteral("bundleCachePath"), database);
        engine->rootContext()->setContextProperty(QStringLiteral("bundleProofDirectory"), qEnvironmentVariable("LB_BUNDLE_PROOF"));
        engine->rootContext()->setContextProperty(QStringLiteral("bundleProofWidth"), qEnvironmentVariableIntValue("LB_BUNDLE_WIDTH"));
    }
private:
    QTemporaryDir m_directory;
};

int main(int argc, char **argv)
{
    QIcon::setThemeName(QStringLiteral("breeze"));
    CardStore::registerQml("BundleTest", 1, 0);
    BundleTestSetup setup;
    return quick_test_main_with_setup(argc, argv, "bundleexpand", nullptr, &setup);
}

#include "bundle_runner.moc"
