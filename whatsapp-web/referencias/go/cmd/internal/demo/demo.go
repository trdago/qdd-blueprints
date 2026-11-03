// Package demo junta lo que comparten los ejemplos: flags comunes, sesión vinculada y espera de conexión.
// Los logs van a stderr; stdout queda para lo que la persona debe leer (código de vinculación, QR, resultados).
package demo

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"

	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/wa"
)

// DeviceName es el nombre que la persona ve en «Dispositivos vinculados».
const DeviceName = "Empresa Demo Vigía"

// Flags comunes a los ejemplos.
type Flags struct {
	Session string
	History string
}

// Register agrega -sesion y -historial al conjunto de flags.
func Register(fs *flag.FlagSet) *Flags {
	f := &Flags{}
	fs.StringVar(&f.Session, "sesion", "session.db", "archivo SQLite de la sesión (queda con permisos 600)")
	fs.StringVar(&f.History, "historial", "historial", "carpeta del historial guardado sin leer (permisos 700)")
	return f
}

// Main corre fn con un contexto que termina con Ctrl+C o SIGTERM y devuelve el código de salida (3 = no reiniciar).
func Main(fn func(ctx context.Context, log *slog.Logger) error) int {
	syscall.Umask(0o077) // sesión, adjuntos e historial: nada legible por otros usuarios (incluye -wal y -shm)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	err := fn(ctx, log)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
	return wa.ExitCode(err)
}

// Parse lee los flags; con un error de uso devuelve false.
func Parse(fs *flag.FlagSet, args []string, stderr io.Writer) bool {
	fs.SetOutput(stderr)
	return fs.Parse(args) == nil
}

var errNotEnrolled = errors.New("la sesión no está vinculada: corre primero el ejemplo enrolar")

// Enrolled abre la sesión y exige que ya esté vinculada.
func Enrolled(ctx context.Context, sessionPath string) (*whatsmeow.Client, error) {
	cli, err := wa.OpenClient(ctx, sessionPath, DeviceName)
	if err != nil {
		return nil, err
	}
	if !wa.IsEnrolled(cli) {
		return nil, errNotEnrolled
	}
	return cli, nil
}

// ConnectAndWait conecta y espera el evento Connected (o el plazo).
func ConnectAndWait(ctx context.Context, cli *whatsmeow.Client, timeout time.Duration) error {
	connected := make(chan struct{}, 1)
	cli.AddEventHandler(func(evt any) {
		if _, ok := evt.(*events.Connected); ok {
			signalOnce(connected)
		}
	})
	if err := cli.ConnectContext(ctx); err != nil {
		return fmt.Errorf("conectar a WhatsApp: %w", err)
	}
	return awaitSignal(ctx, connected, timeout)
}

func signalOnce(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

func awaitSignal(ctx context.Context, ch <-chan struct{}, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-ch:
		return nil
	case <-timer.C:
		return errors.New("WhatsApp no confirmó la conexión a tiempo")
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Wait espera d o hasta que ctx termine.
func Wait(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
