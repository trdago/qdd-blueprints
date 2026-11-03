// Ejemplo responder: atiende SOLO el chat propio («Mensaje a ti mismo») y contesta ahí. Nadie más puede activarlo ni
// recibe nada; las respuestas llevan la marca 🤖 para no volver a procesarse.
//
//	go run ./cmd/responder               escucha permanente: todo texto del chat propio se atiende
//	go run ./cmd/responder -prefijo bot  solo «bot <pregunta>»
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/cmd/internal/demo"
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/domain"
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/memstore"
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/ports"
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/usecases"
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/wa"
)

func main() {
	fs := flag.NewFlagSet("responder", flag.ContinueOnError)
	common := demo.Register(fs)
	prefix := fs.String("prefijo", "", "prefijo que activa al agente (vacío = escucha permanente)")
	if !demo.Parse(fs, os.Args[1:], os.Stderr) {
		os.Exit(2)
	}
	os.Exit(demo.Main(func(ctx context.Context, log *slog.Logger) error { return serve(ctx, log, common, *prefix) }))
}

func serve(ctx context.Context, log *slog.Logger, f *demo.Flags, prefix string) error {
	vault, err := wa.NewHistoryVault(f.History)
	if err != nil {
		return err
	}
	cli, err := demo.Enrolled(ctx, f.Session)
	if err != nil {
		return err
	}
	defer cli.Disconnect()
	empty := usecases.NewIngestor(usecases.IngestDeps{Store: memstore.New(), Now: time.Now, Log: log}, usecases.DefaultIngestConfig())
	echo := echoHandler{out: wa.NewSelfReplier(cli), prefix: prefix}
	listener := wa.NewListener(ctx, cli, wa.ListenerOptions{Ingest: empty, Log: log, Vault: vault, Self: echo, HoldHistory: true})
	go listener.Run()
	if err := cli.ConnectContext(ctx); err != nil {
		return err
	}
	<-ctx.Done()
	return nil
}

// echoHandler contesta lo que recibe; reemplázalo por el agente del proyecto.
type echoHandler struct {
	out    ports.SelfMessenger
	prefix string
}

func (h echoHandler) HandleText(ctx context.Context, text string) error {
	body, ok := h.question(text)
	if !ok {
		return nil
	}
	_, err := h.out.SendToSelfID(ctx, domain.WithReplyMark("Recibido: «"+body+"»"))
	return err
}

func (h echoHandler) question(text string) (string, bool) {
	if h.prefix == "" {
		return text, text != ""
	}
	return domain.ParseSelfCommand(text, h.prefix)
}

func (echoHandler) HandleReaction(context.Context, string, string) error { return nil }
