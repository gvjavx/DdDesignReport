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
        check("csv keeps UTF-8", text("csv").equals("Kopi\nTéh\n"));
        check("pdf", text("pdf").startsWith("%PDF-"));
        check("html", text("html").contains(">Téh<"));
        check("xlsx is a zip", text("xlsx").startsWith("PK"));
        check("xls", text("xls").contains("<Data ss:Type=\"String\">Kopi</Data>"));
        check("null data", new String(DdReport.render(TEMPLATE, null, "csv")).isEmpty());

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

        if (failed > 0) {
            System.out.println(failed + " failed");
            System.exit(1);
        }
        System.out.println("all passed");
    }
}
