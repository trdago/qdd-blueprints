// Entrada de la función (paso «entrada» del blueprint back-end-manager).
//
// Todo lo caro se prepara una sola vez en el arranque en frío (configuración, pool, clientes, router) y cualquier
// error ahí termina el proceso con log.Fatal: Lambda lo informa como Runtime.ExitError y reintenta el arranque, en
// vez de dejar un handler nulo que entra en pánico en cada invocación (BEM-C01). El mismo router corre en local con
// http.ListenAndServe, en pruebas con httptest y en Lambda con el adaptador del gateway.
//
// Build (CI):
//
//	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -trimpath \
//	  -ldflags "-s -w -X main.version=$GITHUB_SHA" -o bootstrap ./cmd/api-lambda
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"

	"github.com/trdago/qdd-blueprints/back-end-manager/referencias/config"
	"github.com/trdago/qdd-blueprints/back-end-manager/referencias/cors"
)

// version la fija el build con -ldflags "-X main.version=<commit>"; /health la informa para el smoke test.
var version = "dev"

// Dependencias son los puertos que el router necesita; en producción se crean en main, en pruebas se simulan.
type Dependencias struct {
	// Consultar representa una dependencia externa (base, tercero); recibe un contexto con plazo.
	Consultar func(ctx context.Context) (string, error)
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	cfg, err := config.Load()
	if err != nil {
		fatal("configuración", err) // el error nombra variables, nunca valores (BEM-C05)
	}
	deps := Dependencias{Consultar: func(ctx context.Context) (string, error) { return "ok", nil }}
	h := NuevoRouter(cfg, deps)

	if os.Getenv("AWS_LAMBDA_RUNTIME_API") == "" {
		slog.Info("servidor local", "addr", ":8080")
		fatal("servidor local", http.ListenAndServe(":8080", h))
	}
	lambda.Start(httpadapter.NewV2(h).ProxyWithContext) // gateway=http_api; REST: httpadapter.New; Function URL: NewFunctionURL
}

func fatal(etapa string, err error) {
	slog.Error("arranque fallido", "etapa", etapa, "error", err.Error())
	os.Exit(1)
}

// NuevoRouter arma la API. CORS va primero, luego request id, recuperador y presupuesto (BEM-C11, BEM-C12, BEM-C25).
func NuevoRouter(cfg config.Config, deps Dependencias) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) { escribirJSON(w, 200, map[string]string{"api": "ok"}) })
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		escribirJSON(w, 200, map[string]string{"status": "ok", "version": version}) // sin datos internos
	})
	mux.HandleFunc("GET /v1/consulta", func(w http.ResponseWriter, r *http.Request) {
		v, err := deps.Consultar(r.Context())
		if err != nil {
			responderError(w, r, err)
			return
		}
		escribirJSON(w, 200, map[string]string{"valor": v})
	})
	mux.HandleFunc("POST /v1/eco", func(w http.ResponseWriter, r *http.Request) {
		var cuerpo map[string]any
		if err := json.NewDecoder(r.Body).Decode(&cuerpo); err != nil {
			responderError(w, r, err)
			return
		}
		escribirJSON(w, 200, cuerpo)
	})
	mux.HandleFunc("GET /v1/archivo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write(pdfDeEjemplo) // el adaptador lo codifica en base64 porque no es UTF-8 (BEM-C15)
	})

	var h http.Handler = mux
	h = limites(cfg.HandlerBudget, cfg.MaxBodyBytes)(h)
	h = recuperador(h)
	h = requestID(h)
	h = cors.Middleware(cors.PorDefecto(cfg.AllowedOrigins, false))(h)
	return h
}

// limites da a cada petición un plazo propio menor que el corte del gateway y un tope de cuerpo (BEM-C12, BEM-C13).
func limites(presupuesto time.Duration, maxCuerpo int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), presupuesto)
			defer cancel()
			r.Body = http.MaxBytesReader(w, r.Body, maxCuerpo)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type claveRequestID struct{}

// requestID toma el id de la invocación de Lambda (o crea uno en local), lo deja en el contexto y lo devuelve en
// X-Request-Id: con ese id se encuentra la línea del error en CloudWatch (BEM-C25).
func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := ""
		if lc, ok := lambdacontext.FromContext(r.Context()); ok {
			id = lc.AwsRequestID
		}
		if id == "" {
			b := make([]byte, 8)
			_, _ = rand.Read(b)
			id = hex.EncodeToString(b)
		}
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), claveRequestID{}, id)))
	})
}

func recuperador(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				slog.Error("pánico", "request_id", r.Context().Value(claveRequestID{}), "ruta", r.URL.Path, "panico", p)
				escribirJSON(w, http.StatusInternalServerError, map[string]string{"error": "error interno"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// responderError traduce un error a un código sin filtrar detalles internos; el detalle va al log con el request id.
func responderError(w http.ResponseWriter, r *http.Request, err error) {
	var tooBig *http.MaxBytesError
	codigo, mensaje := http.StatusInternalServerError, "error interno"
	switch {
	case errors.As(err, &tooBig):
		codigo, mensaje = http.StatusRequestEntityTooLarge, "cuerpo demasiado grande"
	case errors.Is(err, context.DeadlineExceeded):
		codigo, mensaje = http.StatusGatewayTimeout, "la operación tardó demasiado; intenta de nuevo"
	}
	slog.Error("petición fallida", "request_id", r.Context().Value(claveRequestID{}), "ruta", r.URL.Path, "codigo", codigo, "error", err.Error())
	escribirJSON(w, codigo, map[string]string{"error": mensaje})
}

func escribirJSON(w http.ResponseWriter, codigo int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(codigo)
	_ = json.NewEncoder(w).Encode(v)
}

var pdfDeEjemplo = []byte("%PDF-1.4\n\xe2\xe3\xcf\xd3\n%%EOF\n")
