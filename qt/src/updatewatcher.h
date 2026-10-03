// SPDX-License-Identifier: MIT
#pragma once
// Auto-restart into a freshly package-upgraded binary. dpkg replaces
// /usr/bin/litterbox-qt by renaming a new inode over the path; this watcher
// polls that identity, debounces until the replacement is stable and no
// package manager transaction holds the dpkg locks, then starts the new
// binary detached and lets the application quit normally (so CardStore's
// destructor drains queued commits before the replacement opens the
// database). Watching is armed only for an installed-style prefix; a build
// directory run never restarts itself.

#include <QElapsedTimer>
#include <QObject>
#include <QString>
#include <QStringList>

class QTimer;

class UpdateWatcher : public QObject {
    Q_OBJECT
public:
    explicit UpdateWatcher(const QString &binaryPath, QObject *parent = nullptr);

    // Factory for the running application: nullptr (watching stays off) when
    // the binary does not sit in an installed-style prefix or the chained
    // restart generation cap is reached (loop guard).
    static UpdateWatcher *createForRunningApp(QObject *parent = nullptr);
    // Installed-style means <prefix>/bin/litterbox-qt whose prefix carries
    // share/applications/litterbox-qt.desktop. A build tree has neither.
    static bool watchingEnabled(const QString &binaryPath);
    // Chained auto-restarts so far (LB_RESTART_GENERATION set by the spawner).
    static int restartGeneration();
    static constexpr int MaxRestartGeneration = 5;

    // Test seams; must be set before start().
    void setArguments(const QStringList &args);
    void setLockPaths(const QStringList &paths);
    void setPollInterval(int ms);
    void setStableInterval(int ms);
    // Identity source for the startup baseline. Defaults to the watched path;
    // the running app uses /proc/self/exe so a replacement that lands during
    // application startup (before the watcher is created) is still detected.
    void setBaselinePath(const QString &path);

    void start();

signals:
    // Emitted once, after the replacement binary was spawned detached.
    void replacementSpawned();

private:
    struct Identity {
        quint64 dev = 0, ino = 0, size = 0;
        qint64 mtimeSec = 0, mtimeNsec = 0;
        bool operator==(const Identity &other) const {
            return dev == other.dev && ino == other.ino && size == other.size &&
                mtimeSec == other.mtimeSec && mtimeNsec == other.mtimeNsec;
        }
        bool operator!=(const Identity &other) const { return !(*this == other); }
    };
    Identity identify(bool *exists, const QString &path) const;
    bool packageManagerIdle() const;
    bool spawnReplacement();
    void poll();

    QString m_binaryPath;
    QString m_baselinePath;
    QStringList m_arguments;
    QStringList m_lockPaths;
    QTimer *m_timer = nullptr;
    Identity m_baseline;
    bool m_baselineValid = false;
    Identity m_candidate;
    bool m_candidateValid = false;
    QElapsedTimer m_stable;
    int m_pollIntervalMs = 1000;
    int m_stableIntervalMs = 3000;
    bool m_handled = false;
    bool m_spawned = false;
};
