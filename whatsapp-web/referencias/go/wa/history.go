package wa

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

// HistoryVault guarda los avisos de historial SIN descargarlos ni descifrarlos, mientras no exista
// lista blanca (por ejemplo, al enrolar). Después, Drain los descarga y los pasa por el filtro de clientes.
// Los archivos quedan con permisos 600 y se borran apenas se procesan.
type HistoryVault struct {
	dir string
}

// NewHistoryVault crea el directorio (700) si no existe.
func NewHistoryVault(dir string) (*HistoryVault, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("crear directorio de historial pendiente: %w", err)
	}
	return &HistoryVault{dir: dir}, nil
}

// Save persiste un aviso de historial. Idempotente por contenido.
func (v *HistoryVault) Save(notif *waE2E.HistorySyncNotification) error {
	raw, err := proto.Marshal(notif)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(raw)
	path := filepath.Join(v.dir, hex.EncodeToString(sum[:8])+".pb")
	return os.WriteFile(path, raw, 0o600)
}

// Pending cuenta los avisos guardados.
func (v *HistoryVault) Pending() int {
	files, _ := filepath.Glob(filepath.Join(v.dir, "*.pb"))
	return len(files)
}

// Drain entrega cada aviso guardado a fn (en orden) y borra los que se procesaron bien.
func (v *HistoryVault) Drain(ctx context.Context, fn func(context.Context, *waE2E.HistorySyncNotification) error) error {
	files, err := filepath.Glob(filepath.Join(v.dir, "*.pb"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, f := range files {
		if err := drainOne(ctx, f, fn); err != nil {
			return fmt.Errorf("historial pendiente %s: %w", filepath.Base(f), err)
		}
	}
	return nil
}

func drainOne(ctx context.Context, path string, fn func(context.Context, *waE2E.HistorySyncNotification) error) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var notif waE2E.HistorySyncNotification
	if err := proto.Unmarshal(raw, &notif); err != nil {
		return err
	}
	if err := fn(ctx, &notif); err != nil {
		return err
	}
	return os.Remove(path)
}
