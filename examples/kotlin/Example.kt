import io.ddreport.DdReport
import java.io.File

// Run from the repository root, see examples/README.md.
// Needs the ddreport binary: set DDREPORT_BIN or put it on PATH.
fun main() {
    val template = File("examples/sales.json").readText()
    val data = File("examples/data.json").readText()

    File("examples/out").mkdirs()
    for (format in listOf("pdf", "html", "xlsx", "csv")) {
        val name = File("examples/out/kotlin-sales.$format")
        name.writeBytes(DdReport.render(template, data, format))
        println("wrote $name")
    }
}
