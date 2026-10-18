package io.ddreport;

import java.io.IOException;
import java.io.OutputStream;
import java.nio.charset.StandardCharsets;

/** Renders report templates through the {@code ddreport} binary. Usable from Java and Kotlin. */
public final class DdReport {
    private DdReport() {}

    /**
     * @param templateJson template exported by the designer
     * @param dataJson     JSON object (fields plus the row array) or JSON array of rows
     * @param format       pdf, html, xlsx, xls or csv
     */
    public static byte[] render(String templateJson, String dataJson, String format)
            throws IOException, InterruptedException {
        // ponytail: one process per report. Embed the core (WASM/JNI) if spawn cost shows up.
        String bin = System.getenv().getOrDefault("DDREPORT_BIN", "ddreport");
        Process p = new ProcessBuilder(bin, "-format", format).start();
        // the binary reads all of stdin before writing, so sequential I/O cannot deadlock
        try (OutputStream in = p.getOutputStream()) {
            in.write(("{\"template\":" + templateJson + ",\"data\":" + (dataJson == null ? "null" : dataJson) + "}")
                    .getBytes(StandardCharsets.UTF_8));
        }
        byte[] out = p.getInputStream().readAllBytes();
        String err = new String(p.getErrorStream().readAllBytes(), StandardCharsets.UTF_8).trim();
        if (p.waitFor() != 0) {
            throw new IOException(err.isEmpty() ? "ddreport exited with " + p.exitValue() : err);
        }
        return out;
    }
}
