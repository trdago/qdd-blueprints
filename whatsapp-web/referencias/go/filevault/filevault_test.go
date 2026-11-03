package filevault

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/domain"
)

func check(t *testing.T, ok bool, format string, args ...any) {
	t.Helper()
	if !ok {
		t.Errorf(format, args...)
	}
}

// TestWAW_O04_RutaSeguraYTope: solo rutas absolutas sin «..», archivos regulares y bajo el tope.
func TestWAW_O04_RutaSeguraYTope(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "informe.pdf")
	check(t, os.WriteFile(file, []byte("12345"), 0o600) == nil, "escribir")
	v := New(4)
	cases := map[string]error{"informe.pdf": ErrUnsafePath, dir + "/../x/informe.pdf": ErrUnsafePath, dir: ErrNotRegular, file: ErrTooLarge}
	for path, want := range cases {
		_, err := v.Fetch(context.Background(), domain.Delivery{Path: path})
		check(t, errors.Is(err, want), "%s → %v", path, err)
	}
	doc, err := New(0).Fetch(context.Background(), domain.Delivery{Path: file, Filename: "informe.pdf"})
	check(t, err == nil && string(doc.Data) == "12345", "bajo el tope por defecto: %v", err)
}
