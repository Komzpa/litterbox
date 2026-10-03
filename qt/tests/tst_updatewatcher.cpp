// SPDX-License-Identifier: MIT
// Auto-restart after package upgrade. The watcher must restart into an
// atomically replaced binary (the way dpkg installs one), wait out package
// manager activity, collapse flapping replacements into a single restart and
// stay inert outside an installed-style prefix.
#include "updatewatcher.h"

#include <QCoreApplication>
#include <QDir>
#include <QFile>
#include <QSignalSpy>
#include <QTemporaryDir>
#include <QTest>

#include <fcntl.h>
#include <sys/file.h>
#include <sys/stat.h>
#include <unistd.h>

namespace {
// Installed-style layout: <prefix>/bin/litterbox-qt plus the desktop file the
// package installs under <prefix>/share/applications.
QString makeInstalledBinary(QTemporaryDir &dir, const QString &script)
{
    QDir prefix(dir.path());
    QDir().mkpath(prefix.filePath(QStringLiteral("bin")));
    QDir().mkpath(prefix.filePath(QStringLiteral("share/applications")));
    QFile marker(prefix.filePath(QStringLiteral("share/applications/litterbox-qt.desktop")));
    if (!marker.open(QIODevice::WriteOnly)) return {};
    marker.write("[Desktop Entry]\n");
    marker.close();
    const QString binary = prefix.filePath(QStringLiteral("bin/litterbox-qt"));
    QFile file(binary);
    if (!file.open(QIODevice::WriteOnly)) return {};
    file.write(script.toUtf8());
    file.close();
    ::chmod(binary.toLocal8Bit().constData(), 0755);
    return binary;
}

// dpkg installs by renaming a freshly written file over the target path.
void atomicReplace(const QString &binary, const QString &script)
{
    const QString staged = binary + QStringLiteral(".dpkg-new");
    QFile file(staged);
    QVERIFY(file.open(QIODevice::WriteOnly));
    file.write(script.toUtf8());
    file.close();
    ::chmod(staged.toLocal8Bit().constData(), 0755);
    QVERIFY(::rename(staged.toLocal8Bit().constData(), binary.toLocal8Bit().constData()) == 0);
}
}

class UpdateWatcherTest : public QObject {
    Q_OBJECT
private slots:
    void initTestCase()
    {
        // A clean environment: tests below set the generation explicitly.
        qunsetenv("LB_RESTART_GENERATION");
    }
    void watchesOnlyInstalledStylePrefix()
    {
        QTemporaryDir dir;
        const QString binary = makeInstalledBinary(dir, QStringLiteral("#!/bin/sh\n"));
        QVERIFY(!binary.isEmpty());
        QVERIFY(UpdateWatcher::watchingEnabled(binary));
        // A build directory has no bin/ + share/applications layout.
        QVERIFY(!UpdateWatcher::watchingEnabled(dir.filePath(QStringLiteral("build/litterbox-qt"))));
        QVERIFY(!UpdateWatcher::watchingEnabled(dir.filePath(QStringLiteral("obj-x86_64-linux-gnu/litterbox-qt"))));
        // The running test binary is a build-tree binary: never armed.
        QVERIFY(UpdateWatcher::createForRunningApp(this) == nullptr);
    }
    void restartsIntoAtomicReplacement()
    {
        QTemporaryDir dir;
        const QString marker = dir.filePath(QStringLiteral("spawned"));
        const QString binary = makeInstalledBinary(dir, QStringLiteral("#!/bin/sh\n"));
        QVERIFY(!binary.isEmpty());
        UpdateWatcher watcher(binary);
        watcher.setLockPaths({});
        watcher.setPollInterval(50);
        watcher.setStableInterval(200);
        QSignalSpy spy(&watcher, &UpdateWatcher::replacementSpawned);
        watcher.start();
        // Seeded baseline: no replacement, no restart.
        QTest::qWait(300);
        QCOMPARE(spy.count(), 0);
        atomicReplace(binary, QStringLiteral("#!/bin/sh\necho \"v2 $LB_RESTART_GENERATION\" > \"%1\"\n").arg(marker));
        QTRY_VERIFY_WITH_TIMEOUT(spy.count() == 1, 5000);
        QTRY_VERIFY_WITH_TIMEOUT(QFile::exists(marker), 5000);
        QFile result(marker);
        QVERIFY(result.open(QIODevice::ReadOnly));
        // The spawned replacement is the new bytes, generation 0 -> 1.
        QCOMPARE(QString::fromUtf8(result.readAll()).trimmed(), QStringLiteral("v2 1"));
    }
    void waitsForPackageManagerLock()
    {
        QTemporaryDir dir;
        const QString binary = makeInstalledBinary(dir, QStringLiteral("#!/bin/sh\n"));
        const QString lockPath = dir.filePath(QStringLiteral("dpkg-lock"));
        QFile lockFile(lockPath);
        QVERIFY(lockFile.open(QIODevice::ReadWrite));
        const int lockFd = lockFile.handle();
        QVERIFY(lockFd >= 0);
        QVERIFY(::flock(lockFd, LOCK_EX) == 0);
        UpdateWatcher watcher(binary);
        watcher.setLockPaths({lockPath});
        watcher.setPollInterval(50);
        watcher.setStableInterval(150);
        QSignalSpy spy(&watcher, &UpdateWatcher::replacementSpawned);
        watcher.start();
        atomicReplace(binary, QStringLiteral("#!/bin/sh\n"));
        // Replacement seen, but dpkg still holds the lock: no restart.
        QTest::qWait(700);
        QCOMPARE(spy.count(), 0);
        ::flock(lockFd, LOCK_UN);
        QTRY_VERIFY_WITH_TIMEOUT(spy.count() == 1, 5000);
    }
    void debounceCollapsesFlappingReplacements()
    {
        QTemporaryDir dir;
        const QString marker = dir.filePath(QStringLiteral("spawned"));
        const QString binary = makeInstalledBinary(dir, QStringLiteral("#!/bin/sh\n"));
        UpdateWatcher watcher(binary);
        watcher.setLockPaths({});
        watcher.setPollInterval(50);
        watcher.setStableInterval(400);
        QSignalSpy spy(&watcher, &UpdateWatcher::replacementSpawned);
        watcher.start();
        atomicReplace(binary, QStringLiteral("#!/bin/sh\necho v2 > \"%1\"\n").arg(marker));
        QTest::qWait(150);
        atomicReplace(binary, QStringLiteral("#!/bin/sh\necho \"v3 $LB_RESTART_GENERATION\" > \"%1\"\n").arg(marker));
        QTRY_VERIFY_WITH_TIMEOUT(spy.count() == 1, 5000);
        QTest::qWait(600);
        // Flapping within the debounce window collapses into one restart.
        QCOMPARE(spy.count(), 1);
        QFile result(marker);
        QVERIFY(result.open(QIODevice::ReadOnly));
        QCOMPARE(QString::fromUtf8(result.readAll()).trimmed(), QStringLiteral("v3 1"));
    }
    void detectsReplacementBeforeWatchStarts()
    {
        // The upgrade can land while the app is still starting up: by the time
        // the watcher exists, the path already holds the new file. The
        // baseline then comes from the executed file (/proc/self/exe),
        // simulated here by an identity source captured before the replace.
        QTemporaryDir dir;
        const QString marker = dir.filePath(QStringLiteral("spawned"));
        const QString binary = makeInstalledBinary(dir, QStringLiteral("#!/bin/sh\n"));
        const QString executedFile = dir.filePath(QStringLiteral("executed"));
        QVERIFY(QFile::copy(binary, executedFile));
        atomicReplace(binary, QStringLiteral("#!/bin/sh\necho v2 > \"%1\"\n").arg(marker));
        UpdateWatcher watcher(binary);
        watcher.setBaselinePath(executedFile);
        watcher.setLockPaths({});
        watcher.setPollInterval(50);
        watcher.setStableInterval(150);
        QSignalSpy spy(&watcher, &UpdateWatcher::replacementSpawned);
        watcher.start();
        QTRY_VERIFY_WITH_TIMEOUT(spy.count() == 1, 5000);
    }
    void oneRestartPerReplacementOnly()
    {
        QTemporaryDir dir;
        const QString binary = makeInstalledBinary(dir, QStringLiteral("#!/bin/sh\n"));
        UpdateWatcher watcher(binary);
        watcher.setLockPaths({});
        watcher.setPollInterval(50);
        watcher.setStableInterval(150);
        QSignalSpy spy(&watcher, &UpdateWatcher::replacementSpawned);
        watcher.start();
        atomicReplace(binary, QStringLiteral("#!/bin/sh\n"));
        QTRY_VERIFY_WITH_TIMEOUT(spy.count() == 1, 5000);
        // Further replacements after the restart is decided are the child's
        // business: this process must not restart again (loop guard).
        atomicReplace(binary, QStringLiteral("#!/bin/sh\n"));
        QTest::qWait(800);
        QCOMPARE(spy.count(), 1);
    }
    void generationCapDisablesWatching()
    {
        QTemporaryDir dir;
        const QString binary = makeInstalledBinary(dir, QStringLiteral("#!/bin/sh\n"));
        qputenv("LB_RESTART_GENERATION", QByteArray::number(UpdateWatcher::MaxRestartGeneration));
        QCOMPARE(UpdateWatcher::restartGeneration(), UpdateWatcher::MaxRestartGeneration);
        QVERIFY(!UpdateWatcher::watchingEnabled(binary));
        qputenv("LB_RESTART_GENERATION", "2");
        QVERIFY(UpdateWatcher::watchingEnabled(binary));
        qunsetenv("LB_RESTART_GENERATION");
    }
};

QTEST_MAIN(UpdateWatcherTest)
#include "tst_updatewatcher.moc"
