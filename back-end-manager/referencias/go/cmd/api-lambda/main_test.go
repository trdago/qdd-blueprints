package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"

	"github.com/trdago/qdd-blueprints/back-end-manager/referencias/config"
)

func cfgPrueba() config.Config {
	return config.Config{
		Stage:          "test",
		AllowedOrigins: []string{"https://app.ejemplo.com"},
		HandlerBudget:  200 * time.Millisecond,
		MaxBodyBytes:   1024,
	}
}

func depsOK() Dependencias {
	return Dependencias{Consultar: func(ctx context.Context) (string, error) { return "ok", nil }}
}

func evento(metodo, ruta string) events.APIGatewayV2HTTPRequest {
	return events.APIGatewayV2HTTPRequest{
		Version:  "2.0",
		RouteKey: "$default",
		RawPath:  ruta,
		Headers:  map[string]string{"origin": "https://app.ejemplo.com"},
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{Method: metodo, Path: ruta},
		},
	}
}

// BEM-C02: el handler de Lambda, con un evento sintético del gateway, llega al router igual que httptest.
func TestBEM_C02_ElHandlerDeLambdaLlegaAlRouter(t *testing.T) {
	version = "abc1234"
	adapter := httpadapter.NewV2(NuevoRouter(cfgPrueba(), depsOK()))
	resp, err := adapter.ProxyWithContext(context.Background(), evento("GET", "/health"))
	if err != nil || resp.StatusCode != 200 || !strings.Contains(resp.Body, `"version":"abc1234"`) {
		t.Fatalf("health por Lambda: %d %q %v", resp.StatusCode, resp.Body, err)
	}
	if resp.Headers["Access-Control-Allow-Origin"] != "https://app.ejemplo.com" || resp.Headers["X-Request-Id"] == "" {
		t.Fatalf("cabeceras: %v", resp.Headers)
	}
	raiz, _ := adapter.ProxyWithContext(context.Background(), evento("GET", "/"))
	if raiz.StatusCode != 200 {
		t.Fatalf("la raíz también llega al router (BEM-H01): %d", raiz.StatusCode)
	}
}

// BEM-C12: con una dependencia lenta, el handler responde 504 propio, con CORS, dentro del presupuesto.
func TestBEM_C12_PresupuestoDeTiempo(t *testing.T) {
	lenta := Dependencias{Consultar: func(ctx context.Context) (string, error) {
		select {
		case <-time.After(5 * time.Second):
			return "tarde", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}}
	h := NuevoRouter(cfgPrueba(), lenta)
	r := httptest.NewRequest("GET", "/v1/consulta", nil)
	r.Header.Set("Origin", "https://app.ejemplo.com")
	w := httptest.NewRecorder()
	inicio := time.Now()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusGatewayTimeout || time.Since(inicio) > time.Second {
		t.Fatalf("504 a tiempo: %d en %v", w.Code, time.Since(inicio))
	}
	if w.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Fatal("el 504 trae CORS")
	}
}

// BEM-C13: un cuerpo sobre el tope responde 413.
func TestBEM_C13_CuerpoDemasiadoGrande(t *testing.T) {
	h := NuevoRouter(cfgPrueba(), depsOK())
	grande, _ := json.Marshal(map[string]string{"x": strings.Repeat("a", 4096)})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/v1/eco", bytes.NewReader(grande)))
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("413: %d", w.Code)
	}
}

// BEM-C15: un PDF pasa por el adaptador en base64 y llega íntegro.
func TestBEM_C15_BinarioIntegro(t *testing.T) {
	adapter := httpadapter.NewV2(NuevoRouter(cfgPrueba(), depsOK()))
	resp, err := adapter.ProxyWithContext(context.Background(), evento("GET", "/v1/archivo"))
	if err != nil || !resp.IsBase64Encoded {
		t.Fatalf("binario en base64: %v %v", resp.IsBase64Encoded, err)
	}
	got, _ := base64.StdEncoding.DecodeString(resp.Body)
	if !bytes.Equal(got, pdfDeEjemplo) {
		t.Fatal("el PDF llegó cambiado")
	}
}

// BEM-C11: un pánico responde 500 con CORS y X-Request-Id, sin detalles internos.
func TestBEM_C11_PanicoCon500YCORS(t *testing.T) {
	cfg := cfgPrueba()
	h := NuevoRouter(cfg, Dependencias{Consultar: func(ctx context.Context) (string, error) { panic("detalle interno") }})
	r := httptest.NewRequest("GET", "/v1/consulta", nil)
	r.Header.Set("Origin", "https://app.ejemplo.com")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 500 || w.Header().Get("Access-Control-Allow-Origin") == "" || w.Header().Get("X-Request-Id") == "" {
		t.Fatalf("500: %d %v", w.Code, w.Header())
	}
	if strings.Contains(w.Body.String(), "detalle interno") {
		t.Fatal("el detalle interno llegó al cliente")
	}
}
