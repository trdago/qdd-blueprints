// Package config carga la configuración de la función desde variables de entorno (BEM-C04, BEM-C05).
//
// Reglas: un secreto u obligatorio vacío cuenta como ausente; un secreto nunca tiene valor por defecto; el error
// nombra cada variable que falta o está mal, nunca su valor. Un default solo existe para lo que no es secreto y
// está escrito aquí, junto a su variable.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config es todo lo que la función necesita para arrancar.
type Config struct {
	Stage           string
	DatabaseURL     string // secreto
	JWTSecret       string // secreto
	AllowedOrigins  []string
	HandlerBudget   time.Duration
	MaxBodyBytes    int64
	PresignedURLTTL time.Duration
}

// Load lee el entorno con os.LookupEnv. Devuelve un error que junta todos los problemas, para arreglarlos de una vez.
func Load() (Config, error) {
	return LoadFrom(os.LookupEnv)
}

// LoadFrom permite probar sin tocar el entorno del proceso.
func LoadFrom(lookup func(string) (string, bool)) (Config, error) {
	l := lector{lookup: lookup}
	c := Config{
		Stage:           l.obligatoria("STAGE"),
		DatabaseURL:     l.secreto("DATABASE_URL"),
		JWTSecret:       l.secreto("JWT_SECRET"),
		AllowedOrigins:  l.lista("CORS_ALLOWED_ORIGINS"),
		HandlerBudget:   l.segundos("HANDLER_BUDGET_SECONDS", 25),
		MaxBodyBytes:    int64(l.entero("MAX_BODY_KB", 1024)) * 1024,
		PresignedURLTTL: time.Duration(l.entero("PRESIGNED_URL_MINUTES", 15)) * time.Minute,
	}
	if len(c.JWTSecret) > 0 && len(c.JWTSecret) < 32 {
		l.problema("JWT_SECRET: debe tener al menos 32 caracteres")
	}
	if len(l.problemas) > 0 {
		return Config{}, errors.New("configuración inválida: " + strings.Join(l.problemas, "; "))
	}
	return c, nil
}

type lector struct {
	lookup    func(string) (string, bool)
	problemas []string
}

func (l *lector) problema(p string) { l.problemas = append(l.problemas, p) }

func (l *lector) valor(nombre string) string {
	v, _ := l.lookup(nombre)
	return strings.TrimSpace(v)
}

func (l *lector) obligatoria(nombre string) string {
	v := l.valor(nombre)
	if v == "" {
		l.problema(nombre + ": falta")
	}
	return v
}

// secreto es igual que obligatoria; existe aparte para que nadie le ponga un default.
func (l *lector) secreto(nombre string) string {
	return l.obligatoria(nombre)
}

func (l *lector) lista(nombre string) []string {
	var out []string
	for _, p := range strings.Split(l.valor(nombre), ",") {
		p = strings.TrimRight(strings.TrimSpace(p), "/")
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func (l *lector) entero(nombre string, porDefecto int) int {
	v := l.valor(nombre)
	if v == "" {
		return porDefecto
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		l.problema(fmt.Sprintf("%s: debe ser un entero positivo", nombre))
		return porDefecto
	}
	return n
}

func (l *lector) segundos(nombre string, porDefecto int) time.Duration {
	return time.Duration(l.entero(nombre, porDefecto)) * time.Second
}
