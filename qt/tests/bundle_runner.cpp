#include "CardStore.h"
#include <QFile>
#include <QIcon>
#include <QPointer>
#include <QQmlContext>
#include <QQmlEngine>
#include <QTemporaryDir>
#include <QtQuickTest/quicktest.h>

class BundleTestSetup : public QObject
{
    Q_OBJECT
public slots:
    // Each QML test/data row owns a separate real database and draft journal.
    // Never clear pending owner writes to make the next fixture fit.
    Q_INVOKABLE QObject *freshStore()
    {
        if (m_previous) m_previous->deleteLater();
        auto *store = new CardStore(m_engine);
        const QString database = m_directory.filePath(QStringLiteral("cards-%1.sqlite").arg(++m_sequence));
        const QString cache = qEnvironmentVariable("LB_BUNDLE_CACHE");
        if (!cache.isEmpty() && (!QFile::copy(cache, database)
                || !QFile::setPermissions(database, QFileDevice::ReadOwner | QFileDevice::WriteOwner)))
            qFatal("Cannot copy writable disposable bundle cache");
        if (!store->open(database)) qFatal("Cannot open isolated bundle cache");
        m_previous = store;
        return store;
    }
    void qmlEngineAvailable(QQmlEngine *engine)
    {
        m_engine = engine;
        engine->rootContext()->setContextProperty(QStringLiteral("bundleFixture"), this);
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
#ifdef LB_PHOTO_FIXTURE
        // R67 sender-photo consumer regression fixture: the valid 64x64
        // red-left/green-right PNG served by the staged smoke. Fed to
        // CardStore::applyRemoteSenderPhoto exactly as main.cpp's
        // requestFinishedBytes leg feeds Api reply bytes.
        {
            QFile fixture(QStringLiteral(LB_PHOTO_FIXTURE));
            if (!fixture.open(QIODevice::ReadOnly)) qFatal("Cannot open sender-photo fixture");
            engine->rootContext()->setContextProperty(QStringLiteral("senderPhotoFixtureBytes"), fixture.readAll());
        }
        engine->rootContext()->setContextProperty(QStringLiteral("senderPhotoGarbageBytes"),
            QByteArrayLiteral("not-a-real-image-payload"));
#endif
    }
private:
    QTemporaryDir m_directory;
    QQmlEngine *m_engine = nullptr;
    int m_sequence = 0;
    QPointer<CardStore> m_previous;
};

int main(int argc, char **argv)
{
    QIcon::setThemeName(QStringLiteral("breeze"));
    CardStore::registerQml("BundleTest", 1, 0);
    BundleTestSetup setup;
    return quick_test_main_with_setup(argc, argv, "bundleexpand", nullptr, &setup);
}

#include "bundle_runner.moc"
