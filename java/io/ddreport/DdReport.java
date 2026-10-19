package io.ddreport;

import java.io.IOException;
import java.io.OutputStream;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;

/** Renders report templates through the {@code ddreport} binary. Usable from Java and Kotlin. */
public final class DdReport {
    private DdReport() {}

    /** Like {@link #render(String, String, String, Duration)} with a one-minute time limit. */
    public static byte[] render(String templateJson, String dataJson, String format)
            throws IOException, InterruptedException {
        return render(templateJson, dataJson, format, Duration.ofMinutes(1));
    }

    /**
     * @param templateJson template exported by the designer
     * @param dataJson     JSON object (fields plus the row array) or JSON array of rows; null or empty for none
     * @param format       pdf, html, xlsx, xls or csv
     * @param timeout      how long the render may take before it is stopped
     */
    public static byte[] render(String templateJson, String dataJson, String format, Duration timeout)
            throws IOException, InterruptedException {
        // ponytail: one process per report. Embed the core (WASM/JNI) if spawn cost shows up.
        String bin = System.getenv().getOrDefault("DDREPORT_BIN", "ddreport");
        String data = dataJson == null || dataJson.isBlank() ? "null" : dataJson;
        Process p = new ProcessBuilder(bin, "-format", format).start();
        // a render that hangs is stopped, so it cannot hold the caller's thread for good
        AtomicBoolean timedOut = new AtomicBoolean();
        CompletableFuture<Void> watchdog = CompletableFuture.runAsync(() -> {
            if (p.isAlive()) {
                timedOut.set(true);
                p.destroyForcibly();
            }
        }, CompletableFuture.delayedExecutor(timeout.toMillis(), TimeUnit.MILLISECONDS));
        IOException tooLong = new IOException("ddreport took longer than " + timeout.toMillis() + " ms");
        try {
            // the binary reads all of stdin before writing, so sequential I/O cannot deadlock
            try (OutputStream in = p.getOutputStream()) {
                in.write(("{\"template\":" + templateJson + ",\"data\":" + data + "}").getBytes(StandardCharsets.UTF_8));
            } catch (IOException e) {
                // the binary stopped reading early; its exit code and message below say why
            }
            byte[] out, errBytes;
            try {
                out = p.getInputStream().readAllBytes();
                errBytes = p.getErrorStream().readAllBytes();
            } catch (IOException e) {
                // stopping the process closes its streams (on Linux): that is the time limit, not a read error
                throw timedOut.get() ? tooLong : e;
            }
            String err = new String(errBytes, StandardCharsets.UTF_8).trim();
            int status = p.waitFor();
            if (timedOut.get()) {
                throw tooLong;
            }
            if (status != 0) {
                throw new IOException(err.isEmpty() ? "ddreport exited with " + status : err);
            }
            return out;
        } finally {
            watchdog.cancel(false);
            if (p.isAlive()) {
                p.destroyForcibly(); // also when this thread is interrupted
            }
        }
    }
}
