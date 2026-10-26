package org.litterbox;

import java.io.File;
import java.io.FileOutputStream;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.Arrays;
import java.util.Comparator;

/** App-private, bounded, atomic spool. Never evict an unacknowledged report. */
final class CrashStore {
    static final int MAX_REPORTS = 64;
    static final int MAX_BYTES = 65536;
    private final File directory;

    CrashStore(File directory) { this.directory = directory; }

    static String atom(String value) {
        return value != null && value.matches("[A-Za-z0-9._+-]{1,96}") ? value : "unknown";
    }

    static String exceptionStack(Throwable error) {
        StringBuilder result = new StringBuilder();
        for (int cause = 0; error != null && cause < 8; cause++, error = error.getCause()) {
            String type = error.getClass().getName();
            if (type.matches("[A-Za-z_$][A-Za-z0-9_$]*(\\.[A-Za-z_$][A-Za-z0-9_$]*)+"))
                result.append(type).append('\n');
            StackTraceElement[] frames = error.getStackTrace();
            for (int i = 0; i < Math.min(frames.length, 128); i++) {
                StackTraceElement frame = frames[i];
                String symbol = frame.getClassName() + "." + frame.getMethodName();
                // Throwable text, file names and arbitrary VM-supplied symbols never enter the spool.
                if (symbol.matches("[A-Za-z0-9_$.<>-]{1,512}") && frame.getLineNumber() >= -99999999 && frame.getLineNumber() <= 999999999)
                    result.append("  at ").append(symbol).append(':').append(frame.getLineNumber()).append('\n');
            }
        }
        return result.toString();
    }

    synchronized void save(String id, String report) throws IOException {
        if (!id.matches("[a-z0-9-]{1,96}")) throw new IOException("Invalid report id");
        if (!directory.isDirectory() && !directory.mkdirs()) throw new IOException("No crash directory");
        File target = new File(directory, id + ".txt");
        if (target.exists() || new File(directory, id + ".sent").exists()) return;
        // Tombstones only suppress retained Android history; pruning them can at worst
        // re-send an already acknowledged id to the server's idempotent store.
        File[] sent = directory.listFiles((dir, name) -> name.endsWith(".sent"));
        if (sent != null && sent.length > 256) {
            Arrays.sort(sent, Comparator.comparingLong(File::lastModified).thenComparing(File::getName));
            for (int i = 0; i < sent.length - 256; i++) sent[i].delete();
        }
        File[] files = directory.listFiles((dir, name) -> name.endsWith(".txt"));
        if (files == null || files.length >= MAX_REPORTS) throw new IOException("Crash spool full");
        byte[] bytes = report.getBytes(StandardCharsets.UTF_8);
        int length = Math.min(bytes.length, MAX_BYTES);
        while (length > 0 && bytes[length - 1] != '\n') length--;
        File pending = new File(directory, "pending");
        try (FileOutputStream output = new FileOutputStream(pending)) {
            output.write(bytes, 0, length);
            output.getFD().sync();
        }
        if (!pending.renameTo(target)) throw new IOException("Cannot commit crash report");
    }
}
