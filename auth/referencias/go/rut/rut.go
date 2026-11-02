// Package rut es la implementación de referencia del blueprint auth para
// normalizar y validar RUT/RUN chilenos (módulo 11). Copiar al proyecto y
// usar SIEMPRE en el backend: la validación del frontend es solo de ayuda.
package rut

import (
	"errors"
	"strings"
)

// ErrInvalido indica un RUT mal formado o con dígito verificador incorrecto.
var ErrInvalido = errors.New("rut inválido")

// Normalizar devuelve el RUT como cuerpo de 7 u 8 dígitos sin ceros iniciales,
// guion y DV en mayúscula, si es válido. Acepta "12.345.678-5", "12345678-5" y
// "123456785" (RUT ficticio).
func Normalizar(entrada string) (string, error) {
	limpio := strings.ToUpper(strings.NewReplacer(".", "", "-", "", " ", "").Replace(strings.TrimSpace(entrada)))
	if len(limpio) < 2 || len(limpio) > 9 {
		return "", ErrInvalido
	}
	cuerpo, dv := limpio[:len(limpio)-1], limpio[len(limpio)-1:]
	if !soloDigitos(cuerpo) {
		return "", ErrInvalido
	}
	cuerpo = strings.TrimLeft(cuerpo, "0")
	if len(cuerpo) < 7 || len(cuerpo) > 8 {
		return "", ErrInvalido
	}
	if DigitoVerificador(cuerpo) != dv {
		return "", ErrInvalido
	}
	return cuerpo + "-" + dv, nil
}

// PareceRUT indica si la entrada tiene forma de RUT (cuerpo de 7 u 8 dígitos y
// DV), para decidir en un campo único de login si se busca por RUT o por
// correo. No valida el DV.
func PareceRUT(entrada string) bool {
	limpio := strings.ToUpper(strings.NewReplacer(".", "", "-", "", " ", "").Replace(strings.TrimSpace(entrada)))
	if len(limpio) < 8 || len(limpio) > 9 {
		return false
	}
	dv := limpio[len(limpio)-1]
	return soloDigitos(limpio[:len(limpio)-1]) && (dv == 'K' || (dv >= '0' && dv <= '9'))
}

// DigitoVerificador calcula el DV (módulo 11) de un cuerpo numérico.
func DigitoVerificador(cuerpo string) string {
	suma, factor := 0, 2
	for i := len(cuerpo) - 1; i >= 0; i-- {
		suma += int(cuerpo[i]-'0') * factor
		factor++
		if factor > 7 {
			factor = 2
		}
	}
	resto := 11 - suma%11
	if resto == 11 {
		return "0"
	}
	if resto == 10 {
		return "K"
	}
	return string(rune('0' + resto))
}

func soloDigitos(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
