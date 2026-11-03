package domain

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Cited es un valor extraído junto con el fragmento exacto del que salió.
type Cited struct {
	Value string `json:"value"`
	Cite  string `json:"cite"`
}

// citeRadius es cuántos caracteres de contexto se guardan a cada lado de la coincidencia.
const citeRadius = 30

func cite(text string, loc []int) string {
	start := max(0, loc[0]-citeRadius)
	end := min(len(text), loc[1]+citeRadius)
	return strings.TrimSpace(strings.ToValidUTF8(text[start:end], ""))
}

// ── RUT ─────────────────────────────────────────────────────────────────────

// El guion es obligatorio: evita confundir teléfonos o montos con RUT.
// Las facturas electrónicas suelen escribirlo con un espacio tras el guion: "12.345.678- 5" (RUT ficticio).
var rutPattern = regexp.MustCompile(`\b(\d{1,2}\.?\d{3}\.?\d{3})-\s?([\dkK])\b`)

// ExtractRUTs devuelve los RUT con dígito verificador válido, formateados 12.345.678-9, sin repetir.
func ExtractRUTs(text string) []Cited {
	var out []Cited
	seen := map[string]bool{}
	for _, loc := range rutPattern.FindAllStringSubmatchIndex(text, -1) {
		rut, ok := formatRUT(text[loc[2]:loc[3]], text[loc[4]:loc[5]])
		if !ok || seen[rut] {
			continue
		}
		seen[rut] = true
		out = append(out, Cited{Value: rut, Cite: cite(text, loc)})
	}
	return out
}

func formatRUT(body, dv string) (string, bool) {
	digits := onlyDigits(body)
	if !ValidRUT(digits, dv) {
		return "", false
	}
	n, _ := strconv.Atoi(digits)
	return groupThousands(n) + "-" + strings.ToUpper(dv), true
}

// ValidRUT verifica el dígito verificador (módulo 11).
func ValidRUT(digits, dv string) bool {
	return digits != "" && rutDV(digits) == strings.ToUpper(dv)
}

func rutDV(digits string) string {
	sum, factor := 0, 2
	for i := len(digits) - 1; i >= 0; i-- {
		sum += int(digits[i]-'0') * factor
		factor = (factor-1)%6 + 2 // 2,3,4,5,6,7,2,…
	}
	return dvSymbol(11 - sum%11)
}

func dvSymbol(v int) string {
	symbols := map[int]string{11: "0", 10: "K"}
	if s, ok := symbols[v]; ok {
		return s
	}
	return strconv.Itoa(v)
}

func groupThousands(n int) string {
	s := strconv.Itoa(n)
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	return strings.Join(append([]string{s}, parts...), ".")
}

// ── Montos ──────────────────────────────────────────────────────────────────

// Money es un monto en centésimas para no usar punto flotante (UF 35,5 → 3550).
type Money struct {
	Cents    int64  `json:"cents"`
	Currency string `json:"currency"`
	Cite     string `json:"cite"`
}

type amountRule struct {
	pattern  *regexp.Regexp
	currency string
}

// En CLP el punto separa miles; en UF/USD la coma (o el punto) es decimal.
var amountRules = []amountRule{
	{regexp.MustCompile(`(?i)\bUF\s?(\d{1,3}(?:\.\d{3})*(?:,\d{1,4})?|\d+(?:[.,]\d{1,4})?)`), "UF"},
	{regexp.MustCompile(`(?i)(\d+(?:,\d{1,4})?)\s?UF\b`), "UF"},
	{regexp.MustCompile(`(?i)(?:\bUSD|US\$)\s?(\d{1,3}(?:,\d{3})*(?:\.\d{1,2})?|\d+(?:[.,]\d{1,2})?)`), "USD"},
	{regexp.MustCompile(`\$\s?(\d{1,3}(?:\.\d{3})+|\d+)`), "CLP"},
	{regexp.MustCompile(`(?i)\b(\d{1,3}(?:\.\d{3})+|\d+)\s?(?:CLP|pesos)\b`), "CLP"},
}

// totalPattern: el total de una factura ("TOTAL $ 214.200", "Total a pagar: $1.190.000").
var totalPattern = regexp.MustCompile(`(?i)\btotal(?:\s+a\s+pagar)?\s*:?\s*\$\s?(\d{1,3}(?:\.\d{3})+|\d+)`)

// ExtractTotal devuelve el TOTAL del documento si está rotulado (prioridad sobre neto o ítems).
func ExtractTotal(text string) (Money, bool) {
	loc := totalPattern.FindStringSubmatchIndex(text)
	if loc == nil {
		return Money{}, false
	}
	cents, ok := parseCents(text[loc[2]:loc[3]], "CLP")
	return Money{Cents: cents, Currency: "CLP", Cite: cite(text, loc)}, ok
}

// ExtractAmounts devuelve los montos en el orden de las reglas (UF y USD antes que "$").
func ExtractAmounts(text string) []Money {
	var out []Money
	taken := [][]int{}
	for _, rule := range amountRules {
		out, taken = appendAmounts(out, taken, text, rule)
	}
	return out
}

func appendAmounts(out []Money, taken [][]int, text string, rule amountRule) ([]Money, [][]int) {
	for _, loc := range rule.pattern.FindAllStringSubmatchIndex(text, -1) {
		cents, ok := parseCents(text[loc[2]:loc[3]], rule.currency)
		if !ok || overlaps(taken, loc) {
			continue
		}
		taken = append(taken, loc[:2])
		out = append(out, Money{Cents: cents, Currency: rule.currency, Cite: cite(text, loc)})
	}
	return out, taken
}

func overlaps(taken [][]int, loc []int) bool {
	for _, t := range taken {
		if loc[0] < t[1] && t[0] < loc[1] {
			return true
		}
	}
	return false
}

func parseCents(raw, currency string) (int64, bool) {
	normalized := map[string]func(string) string{
		"CLP": func(s string) string { return strings.ReplaceAll(s, ".", "") },
		"UF":  decimalComma,
		"USD": func(s string) string { return strings.ReplaceAll(s, ",", "") },
	}[currency](raw)
	value, err := strconv.ParseFloat(normalized, 64)
	if err != nil || value <= 0 {
		return 0, false
	}
	return int64(value*100 + 0.5), true
}

// decimalComma: "1.234,5" → "1234.5"; "35,5" → "35.5"; "35.5" se mantiene.
func decimalComma(s string) string {
	if !strings.Contains(s, ",") {
		return s
	}
	return strings.ReplaceAll(strings.ReplaceAll(s, ".", ""), ",", ".")
}

// ── Folio ───────────────────────────────────────────────────────────────────

var folioPatterns = []*regexp.Regexp{
	// Formato SII: el folio va solo en su línea, "Nº4" o "N° 1234".
	regexp.MustCompile(`(?m)^\s*N\s?[º°o]\s*(\d{1,10})\s*$`),
	regexp.MustCompile(`(?i)\b(?:factura|boleta|nota de cr[ée]dito)(?:\s+electr[óo]nica)?\s*(?:n[°ºo.]*|nro\.?|#)?\s*:?\s*(\d{1,10})\b`),
	regexp.MustCompile(`(?i)\bfolio\s*(?:n[°ºo.]*|nro\.?|#)?\s*:?\s*(\d{1,10})\b`),
	regexp.MustCompile(`\bF-(\d{1,10})\b`),
}

// ExtractFolio devuelve el primer folio reconocible.
func ExtractFolio(text string) (Cited, bool) {
	for _, p := range folioPatterns {
		if loc := p.FindStringSubmatchIndex(text); loc != nil {
			return Cited{Value: text[loc[2]:loc[3]], Cite: cite(text, loc)}, true
		}
	}
	return Cited{}, false
}

// ── Fechas ──────────────────────────────────────────────────────────────────

var months = map[string]time.Month{
	"enero": 1, "febrero": 2, "marzo": 3, "abril": 4, "mayo": 5, "junio": 6, "julio": 7,
	"agosto": 8, "septiembre": 9, "setiembre": 9, "octubre": 10, "noviembre": 11, "diciembre": 12,
}

var (
	numericDate = regexp.MustCompile(`\b(\d{1,2})[-/.](\d{1,2})[-/.](\d{4})\b`)
	longDate    = regexp.MustCompile(`(?i)\b(\d{1,2})\s+de\s+(enero|febrero|marzo|abril|mayo|junio|julio|agosto|septiembre|setiembre|octubre|noviembre|diciembre)(?:\s+(?:de|del)\s+(\d{4}))?`)
	dueDayOnly  = regexp.MustCompile(`(?i)\bvence(?:r[áa])?\s+el\s+(\d{1,2})\b(\s*(?:de|/|-)\s*\S)?`)
	dueHint     = regexp.MustCompile(`(?i)venc|plazo|pagar antes|hasta el`)
)

// dueContext es cuántos caracteres previos a una fecha se miran para decidir si es de vencimiento.
const dueContext = 25

// DateFacts son las fechas detectadas en un texto.
type DateFacts struct {
	Issued *CitedDate `json:"issued,omitempty"`
	Due    *CitedDate `json:"due,omitempty"`
}

// CitedDate es una fecha con su cita.
type CitedDate struct {
	Date  time.Time `json:"date"`
	Cite  string    `json:"cite"`
	isDue bool
}

// ExtractDates detecta fechas; "vence el 30" infiere mes y año desde ref (fecha del mensaje).
// Una fecha precedida por una mención de vencimiento es Due; la primera otra es Issued.
func ExtractDates(text string, ref time.Time) DateFacts {
	facts := DateFacts{Due: dueFromDayOnly(text, ref)}
	for _, d := range allDates(text, ref) {
		facts = assignDate(facts, d)
	}
	return facts
}

func assignDate(facts DateFacts, d CitedDate) DateFacts {
	if d.isDue && facts.Due == nil {
		facts.Due = &d
		return facts
	}
	if !d.isDue && facts.Issued == nil {
		facts.Issued = &d
	}
	return facts
}

func allDates(text string, ref time.Time) []CitedDate {
	var out []CitedDate
	for _, loc := range numericDate.FindAllStringSubmatchIndex(text, -1) {
		out = appendDate(out, text, loc, atoi(text[loc[6]:loc[7]]), time.Month(atoi(text[loc[4]:loc[5]])))
	}
	for _, loc := range longDate.FindAllStringSubmatchIndex(text, -1) {
		out = appendDate(out, text, loc, yearOr(text, loc, ref), months[strings.ToLower(text[loc[4]:loc[5]])])
	}
	return out
}

func yearOr(text string, loc []int, ref time.Time) int {
	if loc[6] < 0 {
		return ref.Year()
	}
	return atoi(text[loc[6]:loc[7]])
}

func appendDate(out []CitedDate, text string, loc []int, year int, month time.Month) []CitedDate {
	d, ok := validDate(year, month, atoi(text[loc[2]:loc[3]]))
	if !ok {
		return out
	}
	before := text[max(0, loc[0]-dueContext):loc[0]]
	return append(out, CitedDate{Date: d, Cite: cite(text, loc), isDue: dueHint.MatchString(before)})
}

func validDate(year int, month time.Month, day int) (time.Time, bool) {
	d := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	return d, d.Day() == day && d.Month() == month && year >= 2000
}

// dueFromDayOnly cubre "vence el 30" sin mes; si trae mes ("vence el 30 de octubre") lo resuelve allDates.
func dueFromDayOnly(text string, ref time.Time) *CitedDate {
	loc := dueDayOnly.FindStringSubmatchIndex(text)
	if loc == nil || loc[4] >= 0 {
		return nil
	}
	d, ok := nextOccurrence(atoi(text[loc[2]:loc[3]]), ref)
	if !ok {
		return nil
	}
	return &CitedDate{Date: d, Cite: cite(text, loc), isDue: true}
}

// nextOccurrence: el día `day` en el mes de ref, o del mes siguiente si ya pasó.
func nextOccurrence(day int, ref time.Time) (time.Time, bool) {
	refDay := time.Date(ref.Year(), ref.Month(), ref.Day(), 0, 0, 0, 0, time.UTC)
	d, ok := validDate(ref.Year(), ref.Month(), day)
	if ok && !d.Before(refDay) {
		return d, true
	}
	next := refDay.AddDate(0, 1, 1-refDay.Day())
	return validDate(next.Year(), next.Month(), day)
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// ── Tipo de documento ───────────────────────────────────────────────────────

var docKinds = []struct {
	kind    string
	pattern *regexp.Regexp
}{
	{"nota_credito", regexp.MustCompile(`(?i)nota\s+de\s+cr[ée]dito`)},
	{"factura", regexp.MustCompile(`(?i)factura|\bdte\b`)},
	{"boleta", regexp.MustCompile(`(?i)boleta`)},
	{"orden_compra", regexp.MustCompile(`(?i)orden\s+de\s+compra|\boc\b`)},
	{"cotizacion", regexp.MustCompile(`(?i)cotizaci[óo]n|presupuesto`)},
	{"contrato", regexp.MustCompile(`(?i)contrato|convenio`)},
	{"certificado", regexp.MustCompile(`(?i)certificado|patente|vigencia`)},
	{"comprobante_pago", regexp.MustCompile(`(?i)comprobante|transferencia|pagad[oa]`)},
}

// DocKind clasifica el documento por nombre de archivo y texto, sin IA. "otro" si nada calza.
func DocKind(fileName, text string) string {
	haystack := fileName + "\n" + text
	for _, k := range docKinds {
		if k.pattern.MatchString(haystack) {
			return k.kind
		}
	}
	return "otro"
}
