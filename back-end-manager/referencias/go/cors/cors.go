// Package cors es el middleware CORS de la aplicación (capa_cors=aplicacion). Va primero en la cadena, antes que la
// autenticación, el logger y el recuperador de pánicos, para que todo error traiga sus cabeceras (BEM-C10, BEM-C11).
//
// Con una lista (cors=lista) solo un Origin de la lista recibe Access-Control-Allow-Origin con ese origen exacto,
// Vary: Origin y, si se pide, credenciales. Con comodín (cors=comodin) se responde * y nunca credenciales: el
// navegador rechaza * con credenciales, y reflejar cualquier Origin con credenciales abre la API a cualquier sitio.
package cors

import (
	"net/http"
	"strconv"
	"strings"
)

// Politica describe qué orígenes pueden llamar desde el navegador.
type Politica struct {
	Origenes       []string // vacía = comodín
	Credenciales   bool     // solo con lista
	Metodos        []string
	Cabeceras      []string
	MaxAgeSegundos int
}

// PorDefecto devuelve una política con los métodos y cabeceras habituales de una API JSON con Bearer.
func PorDefecto(origenes []string, credenciales bool) Politica {
	return Politica{
		Origenes:       origenes,
		Credenciales:   credenciales && len(origenes) > 0,
		Metodos:        []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		Cabeceras:      []string{"Authorization", "Content-Type", "Accept", "X-Request-Id"},
		MaxAgeSegundos: 600,
	}
}

// Middleware envuelve al router.
func Middleware(p Politica) func(http.Handler) http.Handler {
	permitidos := map[string]bool{}
	for _, o := range p.Origenes {
		permitidos[strings.TrimRight(o, "/")] = true
	}
	metodos := strings.Join(p.Metodos, ", ")
	cabeceras := strings.Join(p.Cabeceras, ", ")
	maxAge := strconv.Itoa(p.MaxAgeSegundos)
	comodin := len(permitidos) == 0

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origen := r.Header.Get("Origin")
			h := w.Header()
			permitido := comodin || permitidos[origen]
			if !comodin {
				h.Add("Vary", "Origin")
			}
			if origen != "" && permitido {
				h.Set("Access-Control-Allow-Origin", valorOrigen(comodin, origen))
				h.Set("Access-Control-Expose-Headers", "X-Request-Id")
			}
			if origen != "" && permitido && p.Credenciales && !comodin {
				h.Set("Access-Control-Allow-Credentials", "true")
			}
			esPreflight := r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != ""
			if !esPreflight {
				next.ServeHTTP(w, r)
				return
			}
			if permitido {
				h.Set("Access-Control-Allow-Methods", metodos)
				h.Set("Access-Control-Allow-Headers", cabeceras)
				h.Set("Access-Control-Max-Age", maxAge)
			}
			w.WriteHeader(http.StatusNoContent)
		})
	}
}

func valorOrigen(comodin bool, origen string) string {
	if comodin {
		return "*"
	}
	return origen
}
