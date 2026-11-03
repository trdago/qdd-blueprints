// Ejemplo aprobar: propone una acción en el chat propio y la ejecuta solo si la persona reacciona 👍 a ESE mensaje antes
// de que venza. Escribir «sí» no la ejecuta. Siempre avisa el resultado.
//
//	go run ./cmd/aprobar -accion "reiniciar el servicio de prueba" -vence 15m
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
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/usecases"
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/wa"
)

const demoKind = "demo"

func main() {
	fs := flag.NewFlagSet("aprobar", flag.ContinueOnError)
	common := demo.Register(fs)
	summary := fs.String("accion", "acción de prueba (no hace nada)", "lo que se propone, tal como lo verá la persona")
	ttl := fs.Duration("vence", domain.DefaultProposal, "plazo para aprobar con 👍")
	if !demo.Parse(fs, os.Args[1:], os.Stderr) {
		os.Exit(2)
	}
	os.Exit(demo.Main(func(ctx context.Context, log *slog.Logger) error { return approve(ctx, log, common, *summary, *ttl) }))
}

func approve(ctx context.Context, log *slog.Logger, f *demo.Flags, summary string, ttl time.Duration) error {
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
	done := make(chan struct{}, 1)
	approvals := usecases.NewApprovals(usecases.ApprovalsDeps{Store: store, Out: wa.NewSelfReplier(cli), Now: time.Now, Log: log})
	approvals.Register(demoKind, usecases.ExecutorFunc(func(context.Context, domain.Action) (string, error) {
		log.Info("acción aprobada con 👍: aquí se ejecutaría")
		done <- struct{}{}
		return "ejecutada (demo)", nil
	}))
	empty := usecases.NewIngestor(usecases.IngestDeps{Store: store, Now: time.Now, Log: log}, usecases.DefaultIngestConfig())
	listener := wa.NewListener(ctx, cli, wa.ListenerOptions{Ingest: empty, Log: log, Vault: vault, Self: selfChat{approvals}, HoldHistory: true})
	go listener.Run()
	if err := demo.ConnectAndWait(ctx, cli, 45*time.Second); err != nil {
		return err
	}
	if _, err := approvals.Propose(ctx, demoKind, summary, nil, ttl); err != nil {
		return err
	}
	return waitApproval(ctx, done, ttl, approvals)
}

func waitApproval(ctx context.Context, done <-chan struct{}, ttl time.Duration, approvals *usecases.Approvals) error {
	timer := time.NewTimer(ttl)
	defer timer.Stop()
	select {
	case <-done:
		demo.Wait(ctx, 3*time.Second) // deja salir el aviso de resultado
	case <-timer.C:
		return approvals.Expire(ctx)
	case <-ctx.Done():
	}
	return nil
}

// selfChat adapta el motor de aprobación al canal del chat propio: los textos no aprueban nada.
type selfChat struct{ approvals *usecases.Approvals }

func (s selfChat) HandleText(ctx context.Context, text string) error {
	_, err := s.approvals.HandleText(ctx, text)
	return err
}

func (s selfChat) HandleReaction(ctx context.Context, targetID, emoji string) error {
	return s.approvals.HandleReaction(ctx, targetID, emoji)
}
