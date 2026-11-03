package cors

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

const app = "https://app.ejemplo.com"

func router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /privado", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"no autorizado"}`, http.StatusUnauthorized)
	})
	mux.HandleFunc("GET /panico", func(w http.ResponseWriter, r *http.Request) { panic("x") })
	return mux
}

// recuperador va DESPUÉS de CORS en la cadena: el 500 también trae las cabeceras.
func recuperador(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recover() != nil {
				http.Error(w, `{"error":"interno"}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func pedir(h http.Handler, metodo, ruta, origen string, preflight bool) *httptest.ResponseRecorder {
	r := httptest.NewRequest(metodo, ruta, nil)
	if origen != "" {
		r.Header.Set("Origin", origen)
	}
	if preflight {
		r.Header.Set("Access-Control-Request-Method", "POST")
		r.Header.Set("Access-Control-Request-Headers", "authorization,content-type")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func conLista() http.Handler {
	return Middleware(PorDefecto([]string{app}, true))(recuperador(router()))
}

// BEM-C10: el preflight responde 204 sin autenticación, con métodos, cabeceras y Max-Age.
func TestBEM_C10_PreflightSinAutenticacion(t *testing.T) {
	w := pedir(conLista(), http.MethodOptions, "/privado", app, true)
	if w.Code != http.StatusNoContent {
		t.Fatalf("preflight: %d", w.Code)
	}
	for _, h := range []string{"Access-Control-Allow-Origin", "Access-Control-Allow-Methods", "Access-Control-Allow-Headers", "Access-Control-Max-Age"} {
		if w.Header().Get(h) == "" {
			t.Fatalf("falta %s", h)
		}
	}
}

// BEM-C11: 404, 401 y 500 traen las cabeceras CORS.
func TestBEM_C11_LosErroresTraenCORS(t *testing.T) {
	for _, ruta := range []string{"/no-existe", "/privado", "/panico"} {
		w := pedir(conLista(), http.MethodGet, ruta, app, false)
		if w.Code < 400 || w.Header().Get("Access-Control-Allow-Origin") != app {
			t.Fatalf("%s: código %d, Allow-Origin %q", ruta, w.Code, w.Header().Get("Access-Control-Allow-Origin"))
		}
	}
}

// BEM-K01: un origen fuera de la lista no recibe cabeceras CORS.
func TestBEM_K01_OrigenFueraDeLaLista(t *testing.T) {
	for _, preflight := range []bool{false, true} {
		w := pedir(conLista(), http.MethodOptions, "/privado", "https://malo.example", preflight)
		if w.Header().Get("Access-Control-Allow-Origin") != "" || w.Header().Get("Access-Control-Allow-Credentials") != "" {
			t.Fatalf("preflight=%v: se reflejó un origen no permitido: %v", preflight, w.Header())
		}
	}
}

// BEM-K02: un origen de la lista recibe ese origen exacto y Vary: Origin.
func TestBEM_K02_OrigenDeLaLista(t *testing.T) {
	w := pedir(conLista(), http.MethodGet, "/privado", app, false)
	if w.Header().Get("Access-Control-Allow-Origin") != app || w.Header().Get("Vary") != "Origin" {
		t.Fatalf("cabeceras: %v", w.Header())
	}
	if w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatal("con lista y credenciales pedidas, Allow-Credentials true")
	}
}

// BEM-K03: con comodín, * y nunca credenciales, aunque se pidan.
func TestBEM_K03_ComodinSinCredenciales(t *testing.T) {
	p := PorDefecto(nil, true)
	h := Middleware(p)(router())
	w := pedir(h, http.MethodGet, "/privado", "https://cualquiera.example", false)
	if w.Header().Get("Access-Control-Allow-Origin") != "*" || w.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Fatalf("comodín: %v", w.Header())
	}
}
