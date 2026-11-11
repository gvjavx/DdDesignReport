package ddreport

import (
	"math"
	"strings"
	"testing"
)

func TestExpressions(t *testing.T) {
	sc := scope{
		t: &Template{Thousands: ".", Decimal: ","},
		rows: []any{
			map[string]any{"qty": 2.0, "price": 1500.0},
			map[string]any{"qty": 3.0, "price": 1000.0, "disc": 500.0},
			map[string]any{"qty": "n/a", "price": 99.0}, // not a number: skipped by aggregates
		},
		ctx: []map[string]any{{"qty": 4.0, "price": 2500.0, "name": "Kopi", "zero": 0.0, "tax": map[string]any{"rate": 0.11}, "inf": 7.0}},
	}
	for text, want := range map[string]string{
		"{{qty * price}}":                            "10000",
		"{{qty * price:2}}":                          "10.000,00",
		"{{ qty*price : 0 }}":                        "10.000",
		"{{price - 500}}":                            "2000",
		"{{price + qty * 10}}":                       "2540", // * before +
		"{{(price + qty) * 10}}":                     "25040",
		"{{price / qty}}":                            "625",
		"{{10 - 4 - 3}}":                             "3", // left to right
		"{{100 / 10 / 5}}":                           "2",
		"{{-qty}}":                                   "-4",
		"{{-(qty + 1) * 2}}":                         "-10",
		"{{price * tax.rate:0}}":                     "275",
		"{{1.5 * 2}}":                                "3",
		"{{inf + 1}}":                                "8", // a field called inf is a field, not infinity
		"{{sum(qty * price)}}":                       "6000",
		"{{sum(qty * price):2}}":                     "6.000,00",
		"{{sum(qty * price) / count()}}":             "2000",
		"{{sum(qty) + sum(price)}}":                  "2604",
		"{{avg(qty * 2)}}":                           "5",
		"{{max(price - 1000)}}/{{min(price)}}":       "500/99",
		"{{count(disc)}} of {{count()}}":             "1 of 3", // count(x) counts the rows that have x
		"{{qty * price / sum(qty * price) * 100:1}}": "166,7",
		// no value rather than a wrong one
		"[{{price / zero}}]": "[]",
		"[{{name * 2}}]":     "[]",
		"[{{missing + 1}}]":  "[]",
		"[{{sum()}}]":        "[]",
		"[{{avg(missing)}}]": "[]",
		// not expressions: left alone
		"{{qty price}}":  "{{qty price}}",
		"{{qty *}}":      "{{qty *}}",
		"{{(qty}}":       "{{(qty}}",
		"{{qty)}}":       "{{qty)}}",
		"{{round(qty)}}": "4",
		"{{sum(qty}}":    "{{sum(qty}}",
		"{{ }}":          "{{ }}",
		"{{a:b}}":        "{{a:b}}",
		"{{qty % 2}}":    "{{qty % 2}}",
	} {
		if got := sc.resolve(text); got != want {
			t.Errorf("resolve(%q) = %q, want %q", text, got, want)
		}
	}

	// a computed number stays a number in spreadsheets
	if c := sc.cell(Element{Text: "{{qty * price:2}}"}); c != (cell{10000.0, false, 2, "", "", "", 0}) {
		t.Errorf("cell = %+v", c)
	}
	if c := sc.cell(Element{Text: "{{price / zero}}"}); c != (cell{"", false, -1, "", "", "", 0}) {
		t.Errorf("cell without a value = %+v", c)
	}
}

func TestExpressionsInAReport(t *testing.T) {
	tpl := []byte(`{"groupBy":"cat","bands":{
	  "detail":{"height":20,"elements":[{"type":"text","x":0,"w":100,"h":12,"text":"{{name}}"},{"type":"text","x":200,"w":100,"h":12,"text":"{{qty * price:0}}","align":"right"}]},
	  "groupFooter":{"height":20,"elements":[{"type":"text","x":200,"w":100,"h":12,"text":"{{sum(qty * price):0}}","align":"right"}]},
	  "summary":{"height":140,"elements":[{"type":"chart","x":0,"y":0,"w":300,"h":120,"chart":"bar","label":"cat","value":"qty * price"}]}}}`)
	data := []map[string]any{
		{"cat": "A", "name": "x", "qty": 2, "price": 1500},
		{"cat": "B", "name": "y", "qty": 1, "price": 700},
		{"cat": "A", "name": "z", "qty": 3, "price": 1000},
	}
	if csv := string(render(t, tpl, data, "csv")); csv != "x,3000\nz,3000\n,6000\ny,700\n,700\n" {
		t.Errorf("csv = %q", csv)
	}
	html := string(render(t, tpl, data, "html"))
	for _, want := range []string{">3,000<", ">6,000<", ">700<"} { // rows, subtotal, and the chart's value labels
		if !strings.Contains(html, want) {
			t.Errorf("html: missing %q", want)
		}
	}
	if n := strings.Count(html, ">6,000<"); n != 3 { // the subtotal, above the chart's bar, and the top of its scale
		t.Errorf("6,000 appears %d times, want 3", n)
	}
}

func TestTerbilang(t *testing.T) {
	for n, want := range map[float64]string{
		0: "nol", 1: "satu", 10: "sepuluh", 11: "sebelas", 12: "dua belas", 19: "sembilan belas", 20: "dua puluh",
		21: "dua puluh satu", 100: "seratus", 111: "seratus sebelas", 250: "dua ratus lima puluh", 1000: "seribu",
		1001: "seribu satu", 1999: "seribu sembilan ratus sembilan puluh sembilan", 2000: "dua ribu",
		100000: "seratus ribu", 2750000: "dua juta tujuh ratus lima puluh ribu", 1000000: "satu juta",
		1e9 + 5: "satu miliar lima", 3e12: "tiga triliun", -15: "minus lima belas", 12.6: "tiga belas", 1e18: "1000000000000000000",
	} {
		if got := terbilang(n); got != want {
			t.Errorf("terbilang(%v) = %q, want %q", n, got, want)
		}
	}
	// in a report, with upper() as receipts write it
	out := string(render(t, []byte(`{"bands":{"summary":{"height":20,"elements":[{"type":"text","w":400,"h":20,"text":"{{upper(terbilang(amount))}} RUPIAH"}]}}}`),
		map[string]any{"amount": "2750000"}, "html"))
	if !strings.Contains(out, "DUA JUTA TUJUH RATUS LIMA PULUH RIBU RUPIAH") {
		t.Errorf("in a report: %s", out)
	}
}

func TestQRIS(t *testing.T) {
	if got := crc16("123456789"); got != "29B1" { // the published check value of CRC-16/CCITT-FALSE
		t.Fatalf("crc16 = %s, want 29B1", got)
	}
	signed := func(s string) string { return s + "6304" + crc16(s+"6304") }
	// a static merchant code: tag 01 is 11, no amount
	body := "000201" + "010211" +
		"26570011ID.DANA.WWW011893600915302259148102090225914810303UMI" +
		"51440014ID.CO.QRIS.WWW0215ID10200176114730303UMI" +
		"52045812" + "5303360" + "5802ID" + "5911Warung Sari" + "6007BANDUNG" + "610540111"
	static := signed(body)
	dynamic := signed(strings.Replace(strings.Replace(body, "010211", "010212", 1), "5802ID", "540515000"+"5802ID", 1))

	for _, c := range []struct {
		payload string
		amount  float64
		want    string
	}{
		{static, 15000, dynamic},
		{" " + static + "\n", 15000, dynamic}, // pasted with spaces around it
		{static[:len(static)-4] + strings.ToLower(crc16(static[:len(static)-4])), 15000, dynamic}, // CRC in lower-case hex
		{dynamic, 15000, dynamic}, // already dynamic: the amount is replaced, not added twice
		{signed(strings.Replace(body, "5802ID", "54039995802ID", 1)), 15000, dynamic},
		{static, 1500.5, signed(strings.Replace(strings.Replace(body, "010211", "010212", 1), "5802ID", "54071500.50"+"5802ID", 1))},
		{static, 0, ""},  // nothing to pay
		{static, -5, ""}, // a refund is not a payment code
		{static[:len(static)-1] + "0", 15000, ""},    // CRC does not match: a mistyped code
		{static[:len(static)-8], 15000, ""},          // no CRC at all
		{signed("0002020102115802ID"), 15000, ""},    // not payload format 01
		{signed("000201010211599Warung"), 15000, ""}, // a length that runs past the end
		{"", 15000, ""},
		{static, 1e14, ""},       // more than the 13 characters tag 54 holds
		{static, math.NaN(), ""}, // not a number (Inf - Inf in the template)
		{static, 0.001, ""},      // rounds to nothing: not a code to pay 0
		{signed(strings.Replace(body, "010211", "", 1)), 15000, dynamic}, // tag 01 is optional: added
	} {
		if got := qris(c.payload, c.amount); got != c.want {
			t.Errorf("qris(%q, %v)\n got %q\nwant %q", c.payload, c.amount, got, c.want)
		}
	}
	if !strings.Contains(dynamic, "5405150005802ID") || !strings.HasSuffix(dynamic, crc16(dynamic[:len(dynamic)-4])) {
		t.Errorf("amount before country code, CRC over the code: %s", dynamic)
	}

	// in a report: from the data, with an aggregate as the amount
	tpl := []byte(`{"bands":{"summary":{"height":20,"elements":[{"type":"text","w":500,"h":20,"text":"{{qris(shop.qris, sum(qty * price))}}"}]}}}`)
	data := map[string]any{"shop": map[string]any{"qris": static}, "rows": []any{
		map[string]any{"qty": 2.0, "price": 5000.0}, map[string]any{"qty": 1.0, "price": "5000"},
	}}
	if out := string(render(t, tpl, data, "csv")); !strings.Contains(out, dynamic) {
		t.Errorf("in a report: %s", out)
	}
	data["shop"] = map[string]any{"qris": "not a qris code"}
	if out := string(render(t, tpl, data, "csv")); strings.Contains(out, "5303360") || strings.Contains(out, "not a qris") {
		t.Errorf("a bad code prints nothing: %s", out)
	}
}
