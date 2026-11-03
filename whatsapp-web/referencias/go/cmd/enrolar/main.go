// Ejemplo enrolar: vincula el número como dispositivo de WhatsApp Web, con código de emparejamiento o con QR, y guarda
// sin leer el historial que WhatsApp entrega al vincular.
//
//	go run ./cmd/enrolar -telefono +56900000000   código de 8 caracteres (WhatsApp → Dispositivos vinculados →
//	                                               Vincular con número de teléfono)
//	go run ./cmd/enrolar                           QR en la terminal (se renueva solo)
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	qrcode "github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"

	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/cmd/internal/demo"
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/domain"
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/ports"
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/wa"
)

// capture es cuánto se espera tras vincular para recibir los avisos de historial (se guardan, no se leen).
const capture = 2 * time.Minute

func main() {
	fs := flag.NewFlagSet("enrolar", flag.ContinueOnError)
	common := demo.Register(fs)
	phone := fs.String("telefono", "", "número a vincular en E.164 (vacío = QR)")
	if !demo.Parse(fs, os.Args[1:], os.Stderr) {
		os.Exit(2)
	}
	os.Exit(demo.Main(func(ctx context.Context, log *slog.Logger) error { return enroll(ctx, log, common, *phone) }))
}

func enroll(ctx context.Context, log *slog.Logger, f *demo.Flags, phone string) error {
	vault, err := wa.NewHistoryVault(f.History)
	if err != nil {
		return err
	}
	cli, err := wa.OpenClient(ctx, f.Session, demo.DeviceName)
	if err != nil {
		return err
	}
	defer cli.Disconnect()
	listener := wa.NewListener(ctx, cli, wa.ListenerOptions{Ingest: dropAll{}, Log: log, Vault: vault, HoldHistory: true})
	go listener.Run()
	if err := pair(ctx, cli, phone); err != nil {
		return err
	}
	fmt.Println("✅ Vinculado. Esperando los avisos de historial (se guardan sin leer)…")
	demo.Wait(ctx, capture)
	fmt.Printf("Listo: %d bloques de historial guardados sin leer hasta tener la lista blanca.\n", vault.Pending())
	return nil
}

func pair(ctx context.Context, cli *whatsmeow.Client, phone string) error {
	if phone != "" {
		return wa.Pair(ctx, cli, phone, showCode)
	}
	return wa.PairQR(ctx, cli, showQR)
}

func showCode(code string) {
	fmt.Printf("\n  Código de vinculación: %s\n\n", code)
	fmt.Println("  En el teléfono: WhatsApp → Dispositivos vinculados → Vincular un dispositivo →")
	fmt.Println("  «Vincular con número de teléfono» e ingresa el código. Tienes unos 2 minutos.")
}

func showQR(code string) {
	qr, err := qrcode.New(code, qrcode.Low)
	if err != nil {
		fmt.Fprintln(os.Stderr, "no se pudo dibujar el QR:", err)
		return
	}
	fmt.Println(qr.ToSmallString(false))
	fmt.Println("Escanea en WhatsApp → Dispositivos vinculados → Vincular un dispositivo (se renueva solo).")
}

// dropAll: durante la vinculación no hay lista blanca; ningún mensaje se acepta.
type dropAll struct{}

func (dropAll) Accepts(domain.IncomingMessage) (domain.Contact, bool) { return domain.Contact{}, false }

func (dropAll) Handle(context.Context, domain.IncomingMessage, ports.AttachmentFetcher) error {
	return nil
}
