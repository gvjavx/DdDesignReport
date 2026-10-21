import io.ddreport.DdReport;

import java.io.IOException;
import java.nio.charset.StandardCharsets;

// Plain-Java test, no test framework needed:
//   javac -d examples/out/classes java/io/ddreport/DdReport.java java/DdReportTest.java
//   java -cp examples/out/classes DdReportTest
// Needs the ddreport binary: set DDREPORT_BIN or put it on PATH.
public class DdReportTest {
    static final String TEMPLATE = "{\"bands\":{\"detail\":{\"height\":20,\"elements\":["
            + "{\"type\":\"text\",\"x\":0,\"y\":0,\"w\":100,\"h\":14,\"text\":\"{{name}}\"}]}}}";
    static final String DATA = "{\"rows\":[{\"name\":\"Kopi\"},{\"name\":\"Téh\"}]}";
    static int failed;

    static void check(String name, boolean ok) {
        System.out.println((ok ? "ok    " : "FAIL  ") + name);
        if (!ok) failed++;
    }

    static String text(String format) throws Exception {
        return new String(DdReport.render(TEMPLATE, DATA, format), StandardCharsets.UTF_8);
    }

    public static void main(String[] args) throws Exception {
        String bom = String.valueOf((char) 0xFEFF); // CSV starts with the byte order mark Excel needs
        check("csv keeps UTF-8", text("csv").equals(bom + "Kopi\nTéh\n"));
        check("pdf", text("pdf").startsWith("%PDF-"));
        check("html", text("html").contains(">Téh<"));
        check("xlsx is a zip", text("xlsx").startsWith("PK"));
        check("xls", text("xls").contains("<Data ss:Type=\"String\">Kopi</Data>"));
        check("null data", new String(DdReport.render(TEMPLATE, null, "csv"), StandardCharsets.UTF_8).equals(bom));
        // fifty thousand rows take seconds; a limit of 100 ms stops them, on a fast machine too
        String many = "[" + "{\"name\":\"x\"},".repeat(50_000) + "{\"name\":\"y\"}]";
        try {
            DdReport.render(TEMPLATE, many, "pdf", java.time.Duration.ofMillis(100));
            check("a render that takes too long is stopped", false);
        } catch (java.io.IOException e) {
            boolean ok = e.getMessage().contains("longer than 100 ms");
            if (!ok) System.out.println("      got: " + e);
            check("a render that takes too long is stopped", ok);
        }

        try {
            DdReport.render(TEMPLATE, DATA, "docx");
            check("unknown format throws", false);
        } catch (IOException e) {
            check("unknown format throws with the binary's message", e.getMessage().contains("unknown format \"docx\""));
        }
        try {
            DdReport.render("{", DATA, "pdf");
            check("broken template throws", false);
        } catch (IOException e) {
            check("broken template throws", true);
        }

        // a renderer keeps one process for report after report
        try (DdReport.Renderer renderer = new DdReport.Renderer(java.time.Duration.ofMillis(300))) {
            String csv = bom + "Kopi\nTéh\n";
            check("renderer: same output", new String(renderer.render(TEMPLATE, DATA, "csv"), StandardCharsets.UTF_8).equals(csv));
            check("renderer: pdf", new String(renderer.render(TEMPLATE, DATA, "pdf"), StandardCharsets.UTF_8).startsWith("%PDF-"));
            try {
                renderer.render(TEMPLATE, DATA, "docx");
                check("renderer: unknown format throws", false);
            } catch (IOException e) {
                check("renderer: unknown format throws with the binary's message", e.getMessage().contains("unknown format \"docx\""));
            }
            try {
                renderer.render(TEMPLATE, DATA, "csv 2\n{}");
                check("renderer: a format cannot pass for a request line", false);
            } catch (IOException e) {
                check("renderer: a format cannot pass for a request line", e.getMessage().contains("unknown format"));
            }
            // data that hides a second request in it is one broken request, not two
            try {
                renderer.render(TEMPLATE, "null}{\"template\":{},\"data\":null", "csv");
                check("renderer: smuggled request refused", false);
            } catch (IOException e) {
                check("renderer: smuggled request refused", true);
            }
            check("renderer: still in step afterwards", new String(renderer.render(TEMPLATE, DATA, "csv"), StandardCharsets.UTF_8).equals(csv));
            try {
                renderer.render(TEMPLATE, many, "pdf");
                check("renderer: a report that takes too long is stopped", false);
            } catch (IOException e) {
                check("renderer: a report that takes too long is stopped", e.getMessage().contains("longer than 300 ms"));
            }
            check("renderer: works again after that", new String(renderer.render(TEMPLATE, DATA, "csv"), StandardCharsets.UTF_8).equals(csv));
        }

        if (failed > 0) {
            System.out.println(failed + " failed");
            System.exit(1);
        }
        System.out.println("all passed");
    }
}
