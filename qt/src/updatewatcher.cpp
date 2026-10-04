// SPDX-License-Identifier: MIT
#include "updatewatcher.h"

#include <QCoreApplication>
#include <QDir>
#include <QElapsedTimer>
#include <QFileInfo>
#include <QProcess>
#include <QProcessEnvironment>
#include <QStandardPaths>
#include <QTimer>
#include <QtGlobal>

#include <cerrno>
#include <fcntl.h>
#include <sys/file.h>
#include <sys/stat.h>
#include <unistd.h>

UpdateWatcher::UpdateWatcher(const QString &binaryPath, QObject *parent)
    : QObject(parent), m_binaryPath(binaryPath),
      // dpkg and apt hold these for the whole unpack+configure transaction;
      // waiting on them keeps the restart clear of a half-installed package.
      m_lockPaths({QStringLiteral("/var/lib/dpkg/lock-frontend"),
                   QStringLiteral("/var/lib/dpkg/lock")}) {
}

int UpdateWatcher::restartGeneration() {
    bool ok = false;
    const int generation = qEnvironmentVariableIntValue("LB_RESTART_GENERATION", &ok);
    return ok && generation > 0 ? generation : 0;
}

bool UpdateWatcher::watchingEnabled(const QString &binaryPath) {
    if (restartGeneration() >= MaxRestartGeneration) return false;
    const QFileInfo info(binaryPath);
    if (info.fileName() != QStringLiteral("litterbox-qt") || info.dir().dirName() != QStringLiteral("bin"))
        return false;
    QDir prefix = info.dir();
    if (!prefix.cdUp()) return false;
    return QFileInfo::exists(prefix.filePath(QStringLiteral("share/applications/litterbox-qt.desktop")));
}

UpdateWatcher *UpdateWatcher::createForRunningApp(QObject *parent) {
    const QString binaryPath = QCoreApplication::applicationFilePath();
    if (!watchingEnabled(binaryPath)) return nullptr;
    auto *watcher = new UpdateWatcher(binaryPath, parent);
    watcher->setArguments(QCoreApplication::arguments().mid(1));
    // The kernel keeps the executed file's inode alive for the process
    // lifetime, so /proc/self/exe is the identity this process actually runs
    // even if dpkg replaced the path before the watcher was created.
    watcher->setBaselinePath(QStringLiteral("/proc/self/exe"));
    watcher->start();
    return watcher;
}

void UpdateWatcher::setArguments(const QStringList &args) { m_arguments = args; }
void UpdateWatcher::setLockPaths(const QStringList &paths) { m_lockPaths = paths; }
void UpdateWatcher::setPollInterval(int ms) { m_pollIntervalMs = ms; }
void UpdateWatcher::setStableInterval(int ms) { m_stableIntervalMs = ms; }
void UpdateWatcher::setBaselinePath(const QString &path) { m_baselinePath = path; }

void UpdateWatcher::start() {
    if (m_timer) return;
    m_timer = new QTimer(this);
    connect(m_timer, &QTimer::timeout, this, &UpdateWatcher::poll);
    poll(); // Seed the baseline identity before arming the poll.
    m_timer->start(m_pollIntervalMs);
}

UpdateWatcher::Identity UpdateWatcher::identify(bool *exists, const QString &path) const {
    struct stat st;
    if (::stat(path.toLocal8Bit().constData(), &st) != 0) {
        *exists = false;
        return {};
    }
    *exists = true;
    Identity identity;
    identity.dev = st.st_dev;
    identity.ino = st.st_ino;
    identity.size = st.st_size;
    identity.mtimeSec = st.st_mtim.tv_sec;
    identity.mtimeNsec = st.st_mtim.tv_nsec;
    return identity;
}

bool UpdateWatcher::packageManagerIdle() const {
    for (const QString &path : m_lockPaths) {
        const int fd = ::open(path.toLocal8Bit().constData(), O_RDONLY | O_CLOEXEC);
        if (fd < 0) continue; // No such lock here: nothing to wait for.
        // flock is not tied to the open mode, so a read-only probe works. A
        // contended lock means dpkg/apt is mid-transaction.
        const bool free = ::flock(fd, LOCK_EX | LOCK_NB) == 0;
        if (free) ::flock(fd, LOCK_UN);
        ::close(fd);
        if (!free) return false;
    }
    return true;
}

bool UpdateWatcher::spawnReplacement() {
    const QString systemdRun = QStandardPaths::findExecutable(QStringLiteral("systemd-run"));
    if (systemdRun.isEmpty()) {
        qWarning("UpdateWatcher: systemd-run is unavailable; keeping the current app running");
        return false;
    }

    QProcessEnvironment environment = QProcessEnvironment::systemEnvironment();
    environment.insert(QStringLiteral("LB_RESTART_GENERATION"),
                       QString::number(restartGeneration() + 1));

    const QString unit = QStringLiteral("litterbox-qt-restart-%1-%2")
                             .arg(QCoreApplication::applicationPid())
                             .arg(restartGeneration() + 1);
    QStringList arguments{QStringLiteral("--user"),
                          QStringLiteral("--unit=%1").arg(unit),
                          QStringLiteral("--collect")};
    for (const QString &key : environment.keys()) {
        if (key == QStringLiteral("DISPLAY") || key == QStringLiteral("WAYLAND_DISPLAY") ||
            key == QStringLiteral("XAUTHORITY") || key == QStringLiteral("DBUS_SESSION_BUS_ADDRESS") ||
            key == QStringLiteral("HOME") || key == QStringLiteral("USER") ||
            key == QStringLiteral("LOGNAME") || key == QStringLiteral("PATH") ||
            key == QStringLiteral("LANG") || key == QStringLiteral("SHELL") ||
            key.startsWith(QStringLiteral("XDG_")) || key.startsWith(QStringLiteral("QT_")) ||
            key.startsWith(QStringLiteral("LC_")) || key == QStringLiteral("LB_RESTART_GENERATION")) {
            arguments.append(QStringLiteral("--setenv=%1=%2").arg(key, environment.value(key)));
        }
    }
    arguments.append(QStringLiteral("--"));
    arguments.append(m_binaryPath);
    arguments.append(m_arguments);

    QProcess process;
    process.setProgram(systemdRun);
    process.setArguments(arguments);
    process.start();
    if (!process.waitForStarted(3000)) {
        qWarning("UpdateWatcher: could not start systemd-run: %s",
                 qPrintable(process.errorString()));
        return false;
    }
    if (!process.waitForFinished(10000)) {
        process.kill();
        process.waitForFinished();
        qWarning("UpdateWatcher: systemd-run timed out while starting replacement %s",
                 qPrintable(m_binaryPath));
        return false;
    }

    QByteArray output = process.readAllStandardOutput();
    output += process.readAllStandardError();
    if (process.exitStatus() != QProcess::NormalExit || process.exitCode() != 0) {
        qWarning("UpdateWatcher: systemd-run failed to start replacement %s: %s",
                 qPrintable(m_binaryPath), output.constData());
        return false;
    }

    qInfo("UpdateWatcher: binary replaced, starting %s in independent service %s and quitting",
          qPrintable(m_binaryPath), qPrintable(unit));
    return true;
}

void UpdateWatcher::poll() {
    if (m_spawned) return;
    bool exists = false;
    const Identity current = identify(&exists, m_binaryPath);
    if (!exists) {
        // Mid-replacement gap (unlink before rename): restart the debounce.
        m_candidateValid = false;
        return;
    }
    if (!m_baselineValid) {
        // First observation: the identity to compare replacements against.
        // The baseline may come from a different path than the watched one
        // (/proc/self/exe for the running app), so stat it separately.
        bool baselineExists = false;
        const QString baselinePath = m_baselinePath.isEmpty() ? m_binaryPath : m_baselinePath;
        const Identity startedAs = identify(&baselineExists, baselinePath);
        if (baselineExists) {
            m_baseline = startedAs;
            m_baselineValid = true;
        }
    }
    if (!m_candidateValid || current != m_candidate) {
        m_candidate = current;
        m_candidateValid = true;
        m_handled = false;
        m_stable.restart();
        return;
    }
    if (m_stable.elapsed() < m_stableIntervalMs) return;
    // Stability alone is not an upgrade: act only once the watched file has
    // actually been replaced relative to the startup baseline, otherwise the
    // app would restart itself on a timer forever.
    if (m_candidate == m_baseline) return;
    if (!packageManagerIdle()) {
        // dpkg still owns the locks: require a fresh stable window once the
        // transaction finishes instead of restarting into a half-installed
        // package.
        m_stable.restart();
        return;
    }
    if (m_handled) return; // This replacement was already acted upon.
    // One spawn attempt per replacement: a failed spawn must not retry every
    // poll (that would be a respawn loop), and a successful spawn lets the
    // caller quit the app. Later genuine replacements are the child's
    // business: the child re-arms against its own baseline.
    m_handled = true;
    if (spawnReplacement()) {
        m_spawned = true;
        emit replacementSpawned();
    }
}
