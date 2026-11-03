// Ejemplo encolar: deja un archivo en la cola de salida y la drena, adjuntándolo en el chat propio. En un proyecto real
// encola quien quiera (otro proceso, una tool) y solo el proceso con la sesión drena cada pocos segundos.
//
//	go run ./cmd/encolar -archivo /ruta/absoluta/informe.pdf -pie "*Informe:* demo"
package main

import (
	"context"
	"flag"
	"log/slog"
	"mime"
	"os"
	"path/filepath"
	"time"

	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/cmd/internal/demo"
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/domain"
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/filevault"
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/memstore"
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/usecases"
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/wa"
)

func main() {
	fs := flag.NewFlagSet("encolar", flag.ContinueOnError)
	common := demo.Register(fs)
	file := fs.String("archivo", "", "ruta absoluta del archivo a entregar")
	caption := fs.String("pie", "", "pie de foto del documento (opcional)")
	if !demo.Parse(fs, os.Args[1:], os.Stderr) {
		os.Exit(2)
	}
	os.Exit(demo.Main(func(ctx context.Context, log *slog.Logger) error { return deliver(ctx, log, common, *file, *caption) }))
}

func deliver(ctx context.Context, log *slog.Logger, f *demo.Flags, file, caption string) error {
	cli, err := demo.Enrolled(ctx, f.Session)
	if err != nil {
		return err
	}
	defer cli.Disconnect()
	store := memstore.New()
	item := domain.Delivery{DocType: "documento", Path: file, Filename: filepath.Base(file),
		Mime: mime.TypeByExtension(filepath.Ext(file)), Caption: caption}
	if err := store.Enqueue(ctx, []domain.Delivery{item}, time.Now()); err != nil {
		return err
	}
	if err := demo.ConnectAndWait(ctx, cli, 45*time.Second); err != nil {
		return err
	}
	outbox := usecases.NewOutbox(usecases.OutboxDeps{Store: store, Vault: filevault.New(filevault.DefaultMaxBytes),
		Sender: wa.NewSelfReplier(cli), Now: time.Now, Log: log}, usecases.OutboxConfig{})
	if err := outbox.Run(ctx); err != nil {
		return err
	}
	state, attempts := store.OutboxStatus(1)
	log.Info("cola de salida", "estado", state, "intentos", attempts)
	return nil
}
