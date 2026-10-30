package io.ddreport;

import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardCopyOption;
import java.util.Locale;
import java.time.Duration;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;

/**
 * Renders report templates through the {@code ddreport} binary. Usable from Java and Kotlin.
 *
 * <p>The binary is DDREPORT_BIN if it is set; else the one in a jar for this system on the
 * class path (the dependency {@code io.github.gvjavx:ddreport} with the classifier
 * {@code linux-x64}, {@code windows-x64}, {@code darwin-arm64}...); else {@code ddreport} on the PATH.
 */
public final class DdReport {
    private DdReport() {}

    private static String bundled; // the binary from the class path, once it is unpacked

    static synchronized String binary() throws IOException {
        String env = System.getenv("DDREPORT_BIN");
        if (env != null && !env.isBlank()) {
            return env;
        }
        if (bundled != null) {
            return bundled;
        }
        String os = System.getProperty("os.name").toLowerCase(Locale.ROOT), arch = System.getProperty("os.arch").toLowerCase(Locale.ROOT);
        String system = (os.contains("win") ? "windows" : os.contains("mac") ? "darwin" : "linux")
                + "-" + (arch.contains("aarch64") || arch.contains("arm64") ? "arm64" : "x64");
        String exe = system.startsWith("windows") ? ".exe" : "";
        try (InputStream in = DdReport.class.getResourceAsStream("/io/ddreport/native/" + system + "/ddreport" + exe)) {
            if (in == null) {
                return "ddreport";
            }
            // a program cannot run from inside a jar: it is copied out once, for as long as this JVM runs
            Path file = Files.createTempFile("ddreport-", exe);
            file.toFile().deleteOnExit();
            Files.copy(in, file, StandardCopyOption.REPLACE_EXISTING);
            file.toFile().setExecutable(true);
            bundled = file.toString();
            return bundled;
        }
    }

    /** Like {@link #render(String, String, String, Duration)} with a one-minute time limit. */
    public static byte[] render(String templateJson, String dataJson, String format)
            throws IOException, InterruptedException {
        return render(templateJson, dataJson, format, Duration.ofMinutes(1));
    }

    /**
     * @param templateJson template exported by the designer
     * @param dataJson     JSON object (fields plus the row array) or JSON array of rows; null or empty for none
     * @param format       pdf, html, xlsx, xls, csv, jrxml, escpos (receipt printers) or txt
     * @param timeout      how long the render may take before it is stopped
     */
    public static byte[] render(String templateJson, String dataJson, String format, Duration timeout)
            throws IOException, InterruptedException {
        // ponytail: one process per report. Embed the core (WASM/JNI) if spawn cost shows up.
        String bin = binary();
        String data = dataJson == null || dataJson.isBlank() ? "null" : dataJson;
        Process p;
        try {
            p = new ProcessBuilder(bin, "-format", format).start();
        } catch (IOException e) {
            throw new IOException(e.getMessage() + ". Add the dependency io.github.gvjavx:ddreport with the classifier of this system"
                    + " (linux-x64, windows-x64, darwin-arm64...), or set DDREPORT_BIN to a ddreport binary", e);
        }
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

    /**
     * Keeps one ddreport process running and sends it one report after another, so a
     * busy server does not start a process per report. Calls from several threads
     * take turns; for reports in parallel, use one Renderer per thread or a pool.
     * A process that stops or takes too long fails only the report it was on, and
     * the next report starts a new one. Close it when the application stops.
     * <pre>
     * try (DdReport.Renderer renderer = new DdReport.Renderer()) {
     *     byte[] pdf = renderer.render(templateJson, dataJson, "pdf");
     * }
     * </pre>
     */
    public static final class Renderer implements AutoCloseable {
        private final Duration timeout;
        private Process p;
        private java.io.BufferedInputStream out;

        /** A renderer whose reports may take up to a minute each. */
        public Renderer() {
            this(Duration.ofMinutes(1));
        }

        /** @param timeout how long one report may take before its process is stopped */
        public Renderer(Duration timeout) {
            this.timeout = timeout;
        }

        /** Like {@link DdReport#render(String, String, String)}, in the process this renderer keeps. */
        public synchronized byte[] render(String templateJson, String dataJson, String format)
                throws IOException, InterruptedException {
            // the format goes on the request line: a word, so it can never change where a request ends
            if (format == null || !format.matches("[A-Za-z0-9_]+")) {
                throw new IOException("ddreport: unknown format \"" + format + "\"");
            }
            if (p == null || !p.isAlive()) {
                start();
            }
            String data = dataJson == null || dataJson.isBlank() ? "null" : dataJson;
            byte[] body = ("{\"template\":" + templateJson + ",\"data\":" + data + "}").getBytes(StandardCharsets.UTF_8);
            Process running = p;
            AtomicBoolean timedOut = new AtomicBoolean();
            CompletableFuture<Void> watchdog = CompletableFuture.runAsync(() -> {
                timedOut.set(true);
                running.destroyForcibly();
            }, CompletableFuture.delayedExecutor(timeout.toMillis(), TimeUnit.MILLISECONDS));
            String status;
            byte[] answer;
            try {
                OutputStream in = running.getOutputStream();
                in.write((format + " " + body.length + "\n").getBytes(StandardCharsets.UTF_8));
                in.write(body);
                in.flush();
                String[] head = line().split(" ");
                int length = Integer.parseInt(head[1]);
                status = head[0];
                answer = out.readNBytes(length);
                if (answer.length < length) {
                    throw new IOException("ddreport stopped");
                }
            } catch (IOException | RuntimeException e) {
                stop(); // the stream cannot be followed any more: the next report starts a new process
                if (timedOut.get()) {
                    throw new IOException("ddreport took longer than " + timeout.toMillis() + " ms");
                }
                throw e instanceof IOException ? (IOException) e : new IOException("ddreport answered something unreadable", e);
            } finally {
                watchdog.cancel(false);
            }
            if (status.equals("ok")) {
                return answer;
            }
            throw new IOException(new String(answer, StandardCharsets.UTF_8)); // the report's own error; the process goes on
        }

        private void start() throws IOException {
            try {
                p = new ProcessBuilder(binary(), "-stdio").redirectError(ProcessBuilder.Redirect.DISCARD).start();
            } catch (IOException e) {
                throw new IOException(e.getMessage() + ". Add the dependency io.github.gvjavx:ddreport with the classifier of this system"
                        + " (linux-x64, windows-x64, darwin-arm64...), or set DDREPORT_BIN to a ddreport binary", e);
            }
            out = new java.io.BufferedInputStream(p.getInputStream());
        }

        // line reads the answer's first line, "ok <length>" or "error <length>"
        private String line() throws IOException {
            StringBuilder b = new StringBuilder();
            for (int c; (c = out.read()) != '\n'; ) {
                if (c < 0) {
                    throw new IOException("ddreport stopped");
                }
                b.append((char) c);
            }
            return b.toString();
        }

        private void stop() {
            if (p != null) {
                p.destroyForcibly();
                p = null;
            }
        }

        /** Stops the process. A renderer can be used again after this: the next report starts a new one. */
        @Override
        public synchronized void close() {
            if (p != null) {
                try {
                    p.getOutputStream().close(); // it finishes and stops
                    if (!p.waitFor(5, TimeUnit.SECONDS)) {
                        p.destroyForcibly();
                    }
                } catch (IOException | InterruptedException e) {
                    p.destroyForcibly();
                    if (e instanceof InterruptedException) {
                        Thread.currentThread().interrupt();
                    }
                }
                p = null;
            }
        }
    }
}
