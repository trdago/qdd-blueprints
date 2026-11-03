package config

import (
	"strings"
	"testing"
	"time"
)

func entorno(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		v, ok := m[k]
		return v, ok
	}
}

func base() map[string]string {
	return map[string]string{
		"STAGE":                "qa",
		"DATABASE_URL":         "postgres://u:p@h:5432/db",
		"JWT_SECRET":           strings.Repeat("x", 32),
		"CORS_ALLOWED_ORIGINS": "https://app.ejemplo.com/, http://localhost:5173",
	}
}

// BEM-C04: un secreto ausente o vacío no arranca y no se usa ningún default.
func TestBEM_C04_SecretoAusenteOVacioNoArranca(t *testing.T) {
	for _, caso := range []string{"ausente", "vacio", "espacios"} {
		m := base()
		switch caso {
		case "ausente":
			delete(m, "JWT_SECRET")
		case "vacio":
			m["JWT_SECRET"] = ""
		case "espacios":
			m["JWT_SECRET"] = "   "
		}
		_, err := LoadFrom(entorno(m))
		if err == nil || !strings.Contains(err.Error(), "JWT_SECRET: falta") {
			t.Fatalf("%s: se esperaba error que nombre JWT_SECRET, llegó %v", caso, err)
		}
	}
}

// BEM-C05: el error nombra la variable, nunca su valor.
func TestBEM_C05_ElErrorNoFiltraValores(t *testing.T) {
	m := base()
	m["JWT_SECRET"] = "corto-y-secreto"
	_, err := LoadFrom(entorno(m))
	if err == nil {
		t.Fatal("un JWT_SECRET corto debe fallar")
	}
	if strings.Contains(err.Error(), "corto-y-secreto") {
		t.Fatalf("el error filtra el valor: %v", err)
	}
}

func TestJuntaTodosLosProblemas(t *testing.T) {
	_, err := LoadFrom(entorno(map[string]string{"MAX_BODY_KB": "-1"}))
	for _, want := range []string{"STAGE: falta", "DATABASE_URL: falta", "JWT_SECRET: falta", "MAX_BODY_KB"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("falta %q en %v", want, err)
		}
	}
}

func TestDefaultsSoloParaLoQueNoEsSecreto(t *testing.T) {
	c, err := LoadFrom(entorno(base()))
	if err != nil {
		t.Fatal(err)
	}
	if c.HandlerBudget != 25*time.Second || c.MaxBodyBytes != 1024*1024 || c.PresignedURLTTL != 15*time.Minute {
		t.Fatalf("defaults: %+v", c)
	}
	if len(c.AllowedOrigins) != 2 || c.AllowedOrigins[0] != "https://app.ejemplo.com" {
		t.Fatalf("orígenes normalizados: %v", c.AllowedOrigins)
	}
}
