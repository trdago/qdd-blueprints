package domain

import (
	"testing"
	"time"
)

func check(t *testing.T, ok bool, format string, args ...any) {
	t.Helper()
	if !ok {
		t.Errorf(format, args...)
	}
}

func values(cs []Cited) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.Value)
	}
	return out
}

// WAW-N01: un RUT con dígito verificador inválido se descarta.
func TestWAW_N01_RUTInvalidoSeDescarta(t *testing.T) {
	check(t, len(ExtractRUTs("RUT 12.345.678-9")) == 0, "DV inválido debe descartarse")
	check(t, len(ExtractRUTs("llámame al 900000000")) == 0, "sin guion no es RUT")
	check(t, ValidRUT("11111111", "1") && !ValidRUT("", "1"), "módulo 11")
}

// WAW-N02: formatos equivalentes del mismo RUT (ficticio) dan el mismo valor.
func TestWAW_N02_FormatosDeRUTEquivalentes(t *testing.T) {
	for _, text := range []string{"12.345.678-5", "12345678-5", "RUT: 12.345.678- 5", "11.111.111-1 y 12345678-5"} {
		got := values(ExtractRUTs(text))
		check(t, len(got) > 0 && got[len(got)-1] == "12.345.678-5", "%q → %v", text, got)
	}
	check(t, len(ExtractRUTs("12.345.678-5 otra vez 12345678-5")) == 1, "sin repetir")
}

// WAW-N03: montos CLP (punto de miles), UF (coma decimal) y USD, en centésimas.
func TestWAW_N03_Montos(t *testing.T) {
	cases := map[string]Money{
		"$1.200.000":         {Cents: 120000000, Currency: "CLP"},
		"1.200.000 CLP":      {Cents: 120000000, Currency: "CLP"},
		"UF 35,5":            {Cents: 3550, Currency: "UF"},
		"USD 120":            {Cents: 12000, Currency: "USD"},
		"TOTAL $ 214.200 ok": {Cents: 21420000, Currency: "CLP"},
	}
	for text, want := range cases {
		got := preferTotal(text)
		check(t, len(got) > 0 && got[0].Cents == want.Cents && got[0].Currency == want.Currency, "%q → %+v", text, got)
	}
	total, ok := ExtractTotal("Neto $180.000\nTotal a pagar: $214.200")
	check(t, ok && total.Cents == 21420000, "el total rotulado gana: %+v", total)
}

// WAW-N04: fechas numéricas, largas y «vence el 30» con el mes de la fecha del mensaje.
func TestWAW_N04_Fechas(t *testing.T) {
	ref := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	f := ExtractDates("Emitida el 05-10-2026, vence el 30 de octubre de 2026", ref)
	check(t, f.Issued != nil && f.Issued.Date.Day() == 5, "emisión: %+v", f.Issued)
	check(t, f.Due != nil && f.Due.Date.Equal(time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)), "vencimiento: %+v", f.Due)
	d := ExtractDates("la factura vence el 30", ref)
	check(t, d.Due != nil && d.Due.Date.Month() == time.October, "mes inferido: %+v", d.Due)
	late := ExtractDates("vence el 5", ref)
	check(t, late.Due != nil && late.Due.Date.Month() == time.November, "ya pasó: mes siguiente: %+v", late.Due)
	check(t, ExtractDates("31/02/2026", ref).Issued == nil, "fecha imposible se descarta")
}

// WAW-N05: folios en sus formas habituales.
func TestWAW_N05_Folio(t *testing.T) {
	for text, want := range map[string]string{"factura N° 4": "4", "folio 1234": "1234", "ref F-1234": "1234", "Nº 77\n": "77"} {
		got, ok := ExtractFolio(text)
		check(t, ok && got.Value == want, "%q → %+v", text, got)
	}
	_, ok := ExtractFolio("sin número")
	check(t, !ok, "sin folio")
}

// WAW-N06: cada campo de la propuesta guarda el fragmento exacto del que salió.
func TestWAW_N06_CitaPorCampo(t *testing.T) {
	msg := IncomingMessage{ChatJID: "c", MessageID: "m", Text: "Factura N° 4 de 12.345.678-5 por $1.200.000", SentAt: time.Now()}
	p := BuildProposal(ProposalInput{Message: msg, Contact: Contact{CompanyName: "Empresa Demo SpA"}, Category: "factura_o_pago"})
	check(t, p.DocType == "factura" && p.Folio == "4" && p.RUT == "12.345.678-5", "campos: %+v", p)
	check(t, p.Amount != nil && p.Amount.Cents == 120000000, "monto: %+v", p.Amount)
	for _, field := range []string{"rut", "folio", "monto"} {
		check(t, p.Citations[field] != "", "%s sin cita", field)
	}
	check(t, p.Status == ProposalPending, "toda propuesta nace pendiente")
}

// WAW-K05: el RUT propio decide emitida/recibida y nunca se propone como contraparte.
func TestWAW_K05_RUTPropio(t *testing.T) {
	own := "11.111.111-1"
	emitted := BuildProposal(ProposalInput{Message: IncomingMessage{Text: "Emisor 11.111.111-1 Receptor 12.345.678-5"}, OwnRUT: own})
	check(t, emitted.Direction == "emitida" && emitted.RUT == "12.345.678-5", "emitida: %+v", emitted)
	received := BuildProposal(ProposalInput{Message: IncomingMessage{Text: "Emisor 12.345.678-5 Receptor 11.111.111-1"}, OwnRUT: own})
	check(t, received.Direction == "recibida" && received.RUT == "12.345.678-5", "recibida: %+v", received)
}

func TestTipoDeDocumento(t *testing.T) {
	for text, want := range map[string]string{"nota de crédito 3": "nota_credito", "boleta": "boleta", "orden de compra": "orden_compra",
		"cotización": "cotizacion", "contrato": "contrato", "certificado de vigencia": "certificado", "hola": "otro"} {
		check(t, DocKind("", text) == want, "%q → %s", text, DocKind("", text))
	}
}
