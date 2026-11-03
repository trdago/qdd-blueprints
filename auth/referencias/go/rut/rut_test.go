package rut

import "testing"

// Escenarios AUTH-R01 y AUTH-R02 del blueprint auth.
func TestNormalizar_FormatosEquivalentes(t *testing.T) {
	for _, in := range []string{"12.345.678-5", "12345678-5", "123456785", " 12345678-5 "} {
		got, err := Normalizar(in)
		if err != nil || got != "12345678-5" {
			t.Errorf("Normalizar(%q) = %q, %v; se esperaba 12345678-5", in, got, err)
		}
	}
}

func TestNormalizar_DVConK(t *testing.T) {
	got, err := Normalizar("10.000.013-k")
	if err != nil || got != "10000013-K" {
		t.Fatalf("Normalizar = %q, %v", got, err)
	}
}

func TestNormalizar_RechazaInvalidos(t *testing.T) {
	for _, in := range []string{"12345678-0", "", "-", "abc", "1234567890-1", "12.3a5.678-5", "0-0", "1-9", "000001-9"} {
		if _, err := Normalizar(in); err == nil {
			t.Errorf("Normalizar(%q) debía fallar", in)
		}
	}
}

func TestPareceRUT(t *testing.T) {
	if !PareceRUT("12.345.678-5") || PareceRUT("ana@example.com") || PareceRUT("12") {
		t.Fatal("PareceRUT clasifica mal")
	}
}
