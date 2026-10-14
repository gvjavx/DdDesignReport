import io.ddreport.DdReport;

import java.nio.file.Files;
import java.nio.file.Path;

// Run from the repository root, see examples/README.md.
// Needs the ddreport binary: set DDREPORT_BIN or put it on PATH.
public class Example {
    public static void main(String[] args) throws Exception {
        String template = Files.readString(Path.of("examples/sales.json"));
        String data = Files.readString(Path.of("examples/data.json"));

        Files.createDirectories(Path.of("examples/out"));
        for (String format : new String[] {"pdf", "html", "xlsx", "csv"}) {
            Path name = Path.of("examples/out/java-sales." + format);
            Files.write(name, DdReport.render(template, data, format));
            System.out.println("wrote " + name);
        }
    }
}
