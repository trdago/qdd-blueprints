package manifiesto

import (
	"strings"
	"testing"
)

// BEM-C08: toda variable que lee el código de referencia está declarada en serverless.example.yml.
func TestBEM_C08_TodaVariableLeidaEstaDeclarada(t *testing.T) {
	leidas, err := Leidas("..")
	if err != nil {
		t.Fatal(err)
	}
	declaradas, err := Declaradas("../../serverless.example.yml")
	if err != nil {
		t.Fatal(err)
	}
	// AWS_LAMBDA_RUNTIME_API la pone Lambda; no se declara.
	if faltan := Faltan(leidas, declaradas, "AWS_LAMBDA_RUNTIME_API"); len(faltan) > 0 {
		t.Fatalf("leídas sin declarar en el manifiesto: %s", strings.Join(faltan, ", "))
	}
	for _, want := range []string{"JWT_SECRET", "DATABASE_URL", "CORS_ALLOWED_ORIGINS"} {
		if !contiene(leidas, want) {
			t.Fatalf("el análisis no encontró %s entre las leídas: %v", want, leidas)
		}
	}
}

func TestFaltanDetectaLaNoDeclarada(t *testing.T) {
	if f := Faltan([]string{"A", "B"}, []string{"A"}); len(f) != 1 || f[0] != "B" {
		t.Fatalf("faltan: %v", f)
	}
}

func contiene(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
