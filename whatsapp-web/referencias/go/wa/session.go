// Package wa conecta un número como dispositivo vinculado de WhatsApp Web (whatsmeow) en SOLO LECTURA.
//
// Regla dura: este paquete no envía, no marca como leído y no publica presencia. Las únicas excepciones viven en tres
// archivos: self_reply.go (siempre al chat propio), third_party.go (solo a un destinatario validado) y phone_requests.go
// (peticiones al teléfono propio). TestNoSendAPIs lo verifica sobre el código fuente.
package wa

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3" // driver "sqlite3" para la sesión de whatsmeow
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// sessionDSN: claves foráneas, WAL y espera ante bloqueos; whatsmeow solo admite SQLite o Postgres para su estado.
const sessionDSN = "?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000"

// Errores terminales: reintentar no sirve, hay que volver a vincular o cerrar la otra instancia.
var (
	ErrSessionRevoked = errors.New("WhatsApp revocó el dispositivo vinculado: hay que volver a vincular")
	ErrStreamReplaced = errors.New("otra instancia abrió la misma sesión; este proceso se detiene")
)

// ExitTerminal es el código de salida de un error terminal: el servicio no debe reiniciarse en bucle contra WhatsApp.
const ExitTerminal = 3

// ExitCode traduce el error final del proceso a su código de salida.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, ErrSessionRevoked) || errors.Is(err, ErrStreamReplaced) {
		return ExitTerminal
	}
	return 1
}

// OpenClient abre (o crea) la sesión en sessionPath: SQLite con permisos 600 en un directorio 700. deviceName es el
// nombre que la persona ve en «Dispositivos vinculados».
func OpenClient(ctx context.Context, sessionPath, deviceName string) (*whatsmeow.Client, error) {
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0o700); err != nil {
		return nil, fmt.Errorf("crear directorio de sesión: %w", err)
	}
	logger := waLog.Stdout("whatsmeow", "WARN", false)
	container, err := sqlstore.New(ctx, "sqlite3", "file:"+sessionPath+sessionDSN, logger)
	if err != nil {
		return nil, fmt.Errorf("abrir sesión de WhatsApp: %w", err)
	}
	if err := os.Chmod(sessionPath, 0o600); err != nil {
		return nil, fmt.Errorf("permisos de la sesión: %w", err)
	}
	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		return nil, fmt.Errorf("leer dispositivo vinculado: %w", err)
	}
	return newReadOnlyClient(device, logger, deviceName), nil
}

func newReadOnlyClient(device *store.Device, logger waLog.Logger, deviceName string) *whatsmeow.Client {
	store.DeviceProps.Os = &deviceName
	cli := whatsmeow.NewClient(device, logger)
	// No pedir al teléfono reenvíos de mensajes que no se pudieron descifrar: menos tráfico, mismo resultado.
	cli.AutomaticMessageRerequestFromPhone = false
	// El historial se descarga a mano y solo con lista blanca (HistoryVault).
	cli.ManualHistorySyncDownload = true
	return cli
}

// IsEnrolled indica si la sesión ya tiene un dispositivo vinculado.
func IsEnrolled(cli *whatsmeow.Client) bool {
	return cli.Store.ID != nil
}
