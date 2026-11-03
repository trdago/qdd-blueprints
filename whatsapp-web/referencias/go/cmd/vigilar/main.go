// Ejemplo vigilar: escucha en solo lectura los chats de la lista blanca y los pasa por la ingesta (adjunto con tope,
// trivial, clasificador, propuesta). Todo lo demás se descarta sin guardarse.
//
//	go run ./cmd/vigilar -lista +56900000000,+56900000001
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/cmd/internal/demo"
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/domain"
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/memstore"
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/ports"
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/usecases"
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/wa"
)

func main() {
	fs := flag.NewFlagSet("vigilar", flag.ContinueOnError)
	common := demo.Register(fs)
	list := fs.String("lista", "", "teléfonos vigilados, separados por coma (vacía = el historial se guarda sin leer)")
	if !demo.Parse(fs, os.Args[1:], os.Stderr) {
		os.Exit(2)
	}
	os.Exit(demo.Main(func(ctx context.Context, log *slog.Logger) error { return watch(ctx, log, common, *list) }))
}

func watch(ctx context.Context, log *slog.Logger, f *demo.Flags, list string) error {
	vault, err := wa.NewHistoryVault(f.History)
	if err != nil {
		return err
	}
	cli, err := demo.Enrolled(ctx, f.Session)
	if err != nil {
		return err
	}
	defer cli.Disconnect()
	store := memstore.New()
	ingestor := usecases.NewIngestor(usecases.IngestDeps{Classifier: noClassifier(), Store: store, Media: store, Now: time.Now, Log: log},
		usecases.DefaultIngestConfig())
	dir := directory(list)
	ingestor.SetDirectory(dir)
	listener := wa.NewListener(ctx, cli, wa.ListenerOptions{Ingest: ingestor, Log: log, Vault: vault, HoldHistory: dir.Size() == 0,
		Status: func(v map[string]string) { log.Info("estado", "estado", v["estado"]) }})
	go listener.Run()
	if err := cli.ConnectContext(ctx); err != nil {
		return err
	}
	return waitStop(ctx, listener.Fatal())
}

// directory arma la lista blanca desde el flag (en un proyecto real viene de su fuente y se recarga cada pocos minutos).
func directory(list string) domain.Directory {
	var contacts []domain.Contact
	for _, phone := range strings.Split(list, ",") {
		contacts = append(contacts, domain.Contact{PhoneE164: strings.TrimSpace(phone), CompanyID: "demo",
			CompanyName: "Empresa Demo SpA", Mode: domain.WatchFull})
	}
	return domain.NewDirectory(contacts, time.Now())
}

// noClassifier deja los mensajes como pendientes: conecta aquí el clasificador del proyecto (servicio o LLM).
func noClassifier() ports.Classifier {
	return ports.ClassifierFunc(func(context.Context, string) (domain.Classification, error) {
		return domain.Classification{}, errors.New("sin clasificador configurado")
	})
}

func waitStop(ctx context.Context, fatal <-chan error) error {
	select {
	case <-ctx.Done():
		return nil
	case err := <-fatal:
		return err
	}
}
