package org.litterbox;

import java.io.File;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;

/** Standalone JVM oracle using the exact production spool/scrubber, no Android mocks. */
public final class CrashStoreTest {
    private static void check(boolean condition, String reason) {
        if (!condition) throw new AssertionError(reason);
    }
    public static void main(String[] args) throws Exception {
        File directory = Files.createTempDirectory("crash-store-oracle-").toFile();
        try {
            CrashStore store = new CrashStore(directory);
            RuntimeException failure = new RuntimeException("mail secret-token https://private.invalid");
            failure.setStackTrace(new StackTraceElement[] {
                new StackTraceElement("org.litterbox.Safe", "run", "private-mail.txt", 42),
                new StackTraceElement("mail\nsecret", "token", "credential", 4)
            });
            String scrubbed = CrashStore.exceptionStack(failure);
            check(scrubbed.contains("java.lang.RuntimeException\n"), "class retained");
            check(scrubbed.contains("  at org.litterbox.Safe.run:42\n"), "useful frame retained");
            check(!scrubbed.contains("mail") && !scrubbed.contains("token") && !scrubbed.contains("private"), "private text excluded");
            store.save("java-fixed", "kind=java\n" + scrubbed);
            byte[] original = Files.readAllBytes(new File(directory, "java-fixed.txt").toPath());
            new CrashStore(directory).save("java-fixed", "different\n");
            check(java.util.Arrays.equals(original, Files.readAllBytes(new File(directory, "java-fixed.txt").toPath())), "restart retains exact bytes and identity");
            Files.write(new File(directory, "exit-acked.sent").toPath(), new byte[0]);
            store.save("exit-acked", "kind=exit\n");
            check(!new File(directory, "exit-acked.txt").exists(), "ack tombstone suppresses history replay");
            for (int i = 1; i < CrashStore.MAX_REPORTS; i++) store.save("exit-" + i, "kind=exit\n");
            boolean full = false;
            try { store.save("java-overflow", "kind=java\n"); } catch (IOException expected) { full = true; }
            check(full, "capacity rejects new reports rather than deleting pending evidence");
            check(java.util.Arrays.equals(original, Files.readAllBytes(new File(directory, "java-fixed.txt").toPath())), "capacity preserves earliest pending crash");
            check(CrashStore.atom("unsafe\nmail").equals("unknown"), "metadata injection excluded");
            check(CrashStore.atom("0.1-53.35").equals("0.1-53.35"), "safe build preserved");
        } finally {
            File[] files = directory.listFiles();
            if (files != null) for (File file : files) Files.deleteIfExists(file.toPath());
            Files.deleteIfExists(directory.toPath());
        }
    }
}
