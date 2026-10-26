package org.litterbox;

import android.app.ActivityManager;
import android.app.ApplicationExitInfo;
import android.content.pm.PackageInfo;
import android.os.Build;
import java.io.File;
import java.util.List;
import java.util.UUID;
import org.qtproject.qt.android.bindings.QtApplication;

/** Installed before the Qt activity; diagnostics must never prevent normal startup. */
public final class CrashApplication extends QtApplication {
    @Override public void onCreate() {
        super.onCreate();
        try {
            final CrashStore store = new CrashStore(new File(getNoBackupFilesDir(), "crashes"));
            String version = "unknown", build = "unknown";
            try {
                PackageInfo info = getPackageManager().getPackageInfo(getPackageName(), 0);
                build = Long.toString(Build.VERSION.SDK_INT >= 28 ? info.getLongVersionCode() : info.versionCode);
                version = CrashStore.atom(info.versionName + "." + build);
            } catch (Throwable ignored) { }
            final String header = "schema=1\npackage=" + CrashStore.atom(getPackageName())
                + "\nandroid=" + Build.VERSION.SDK_INT + "\n";
            final String current = header + "version=" + version + "\nbuild=" + build + "\nversion_scope=current-process\n";
            final Thread.UncaughtExceptionHandler previous = Thread.getDefaultUncaughtExceptionHandler();
            Thread.setDefaultUncaughtExceptionHandler((thread, error) -> {
                try {
                    store.save("java-" + UUID.randomUUID(), current + "timestamp=" + System.currentTimeMillis()
                        + "\nkind=java\nreason=uncaught\nstatus=unknown\nstack=frames\n" + CrashStore.exceptionStack(error));
                } catch (Throwable ignored) {
                    // Includes OOM and disk-full: preserve Android's original fatal behavior.
                } finally {
                    if (previous != null) previous.uncaughtException(thread, error);
                    else {
                        android.os.Process.killProcess(android.os.Process.myPid());
                        System.exit(10);
                    }
                }
            });
            new Thread(() -> recoverExits(store, header), "crash-recovery").start();
        } catch (Throwable ignored) { }
    }

    private void recoverExits(CrashStore store, String header) {
        String history = "unsupported";
        if (Build.VERSION.SDK_INT >= 30) {
            try {
                ActivityManager manager = (ActivityManager) getSystemService(ACTIVITY_SERVICE);
                List<ApplicationExitInfo> exits = manager.getHistoricalProcessExitReasons(getPackageName(), 0, 64);
                history = exits.isEmpty() ? "empty" : "available";
                for (ApplicationExitInfo exit : exits) {
                    // All retained exit reasons matter when the incident cause is unknown.
                    // Descriptions, logcat, ANR traces and native tombstone protobufs are NOT safe text.
                    store.save("exit-" + exit.getTimestamp() + "-" + exit.getPid(),
                        header + "version=unknown\nbuild=unknown\nversion_scope=unknown-historical-process\n"
                        + "timestamp=" + exit.getTimestamp() + "\nkind=exit\nreason=" + exit.getReason()
                        + "\nstatus=" + exit.getStatus() + "\nstack=unavailable\n");
                }
            } catch (Throwable ignored) { history = "unavailable"; }
        }
        try {
            store.save("exit-history-" + history, header
                + "version=unknown\nbuild=unknown\nversion_scope=unknown-historical-process\n"
                + "timestamp=" + System.currentTimeMillis() + "\nkind=exit\nreason=unavailable\nstatus=unknown\nstack=unavailable\nhistory=" + history + "\n");
        } catch (Throwable ignored) { }
    }
}
