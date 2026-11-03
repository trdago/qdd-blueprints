// Package filevault lee del disco local los archivos a entregar, con ruta segura y tope de tamaño.
package filevault

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/domain"
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/ports"
)

// DefaultMaxBytes es el tope por archivo si no se indica otro (16 MiB).
const DefaultMaxBytes = 16 << 20

// Errores permanentes: reintentar no los arregla.
var (
	ErrUnsafePath = errors.New("la ubicación no es una ruta absoluta segura")
	ErrNotRegular = errors.New("la ubicación no es un archivo regular")
	ErrTooLarge   = errors.New("el archivo supera el tamaño máximo permitido")
)

// Vault lee archivos locales bajo un tope de tamaño.
type Vault struct {
	maxBytes int64
}

var _ ports.DocumentFetcher = Vault{}

// New crea el vault; un tope <= 0 usa DefaultMaxBytes.
func New(maxBytes int64) Vault {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBytes
	}
	return Vault{maxBytes: maxBytes}
}

// Fetch valida que la ruta sea absoluta y sin «..», que sea un archivo regular y que no pase el tope; recién ahí lo lee.
func (v Vault) Fetch(_ context.Context, d domain.Delivery) (ports.OutDocument, error) {
	if !safePath(d.Path) {
		return ports.OutDocument{}, fmt.Errorf("%w: %q", ErrUnsafePath, d.Path)
	}
	info, err := os.Stat(d.Path)
	if err != nil {
		return ports.OutDocument{}, err
	}
	if !info.Mode().IsRegular() {
		return ports.OutDocument{}, fmt.Errorf("%w: %q", ErrNotRegular, d.Path)
	}
	if info.Size() > v.maxBytes {
		return ports.OutDocument{}, fmt.Errorf("%w (%d bytes)", ErrTooLarge, info.Size())
	}
	data, err := os.ReadFile(d.Path)
	return ports.OutDocument{Filename: d.Filename, Mime: d.Mime, Data: data}, err
}

func safePath(p string) bool {
	return filepath.IsAbs(p) && !strings.Contains(p, "..")
}
