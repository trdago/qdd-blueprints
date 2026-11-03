package wa

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/proto/waHistorySync"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/domain"
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/ports"
)

// Ingest es lo que el listener necesita del caso de uso de ingesta.
type Ingest interface {
	Accepts(msg domain.IncomingMessage) (domain.Contact, bool)
	Handle(ctx context.Context, msg domain.IncomingMessage, fetch ports.AttachmentFetcher) error
}

// StatusSink recibe los cambios de estado del proceso (conectado, desconectado, sesión revocada…).
type StatusSink func(values map[string]string)

// GroupSink recibe los nombres de los grupos (solo metadatos), para que la persona elija cuáles vigilar.
type GroupSink func(ctx context.Context, groups []*types.GroupInfo)

type historyMsg = waHistorySync.HistorySyncMsg

type job struct {
	msg   domain.IncomingMessage
	fetch ports.AttachmentFetcher
}

// Tamaños de las colas en memoria y reintentos de la ingesta.
const (
	jobQueue     = 1024
	selfQueue    = 32
	maxTries     = 8
	firstBackoff = time.Second
	maxBackoff   = time.Minute
)

// ListenerOptions configura el listener.
type ListenerOptions struct {
	Ingest Ingest
	Status StatusSink
	Groups GroupSink // nil = no listar grupos
	Log    *slog.Logger
	Vault  *HistoryVault
	// Self atiende el chat propio. nil = canal desactivado.
	Self SelfHandler
	// HoldHistory guarda los avisos de historial sin descargarlos (todavía no hay lista blanca).
	HoldHistory bool
}

// Listener filtra en el handler (lista blanca) y procesa en un worker con reintentos, para no bloquear a whatsmeow si la
// base está ocupada. El chat propio va en otra cola, en orden.
type Listener struct {
	cli      *whatsmeow.Client
	opts     ListenerOptions
	log      *slog.Logger
	jobs     chan job
	selfJobs chan selfJob
	fatal    chan error
	ctx      context.Context
	hold     atomic.Bool
	phone    *phoneRequests
}

// NewListener registra el handler en el cliente. Run arranca los workers.
func NewListener(ctx context.Context, cli *whatsmeow.Client, opts ListenerOptions) *Listener {
	l := &Listener{
		cli: cli, opts: opts, log: opts.Log, ctx: ctx, phone: newPhoneRequests(cli),
		jobs: make(chan job, jobQueue), selfJobs: make(chan selfJob, selfQueue), fatal: make(chan error, 1),
	}
	l.hold.Store(opts.HoldHistory)
	cli.AddEventHandler(l.handle)
	return l
}

// SetHoldHistory activa o desactiva el resguardo del historial. Al desactivarlo (llegó la lista blanca) procesa lo guardado.
func (l *Listener) SetHoldHistory(hold bool) {
	if l.hold.Swap(hold) && !hold {
		l.drainHistory()
	}
}

// Fatal emite un error cuando la sesión deja de servir (revocada o reemplazada).
func (l *Listener) Fatal() <-chan error {
	return l.fatal
}

func (l *Listener) handle(evt any) {
	switch v := evt.(type) {
	case *events.Message:
		l.onMessage(v, domain.SourceRealtime)
	case *events.HistorySync:
		l.onHistory(v)
	case *events.MediaRetry:
		l.phone.deliver(v)
	default:
		l.onLifecycle(evt)
	}
}

// onMessage descarta en el acto lo que no está en la lista blanca: no se encola, no se guarda, no se registra.
func (l *Listener) onMessage(evt *events.Message, source domain.Source) {
	if notif := evt.Message.GetProtocolMessage().GetHistorySyncNotification(); notif != nil {
		l.onHistoryNotification(notif)
		return
	}
	if l.routeSelf(evt, source) {
		return
	}
	msg, fetch, ok := toIncoming(l.ctx, l.cli, evt, source, l.fetcher)
	if !ok || !l.accepted(msg) {
		return
	}
	l.enqueue(job{msg: msg, fetch: fetch})
}

// accepted: chat vigilado y, en modo solo documentos, con adjunto.
func (l *Listener) accepted(msg domain.IncomingMessage) bool {
	contact, ok := l.opts.Ingest.Accepts(msg)
	return ok && !contact.SkipsText(msg)
}

func (l *Listener) enqueue(j job) {
	select {
	case l.jobs <- j:
	case <-l.ctx.Done():
	}
}

func (l *Listener) onHistory(evt *events.HistorySync) {
	for _, conv := range evt.Data.GetConversations() {
		l.onConversation(conv.GetID(), conv.GetMessages())
	}
}

func (l *Listener) onConversation(chatID string, msgs []*historyMsg) {
	chat, err := types.ParseJID(chatID)
	if err != nil || !supportedChat[chat.Server] {
		return
	}
	for _, hm := range msgs {
		evt, err := l.cli.ParseWebMessage(chat, hm.GetMessage())
		if err != nil {
			continue
		}
		l.onMessage(evt, domain.SourceHistory)
	}
}

func (l *Listener) onLifecycle(evt any) {
	switch evt.(type) {
	case *events.Connected:
		l.setStatus("conectado")
		l.snapshotGroups()
		l.drainHistory()
	case *events.Disconnected:
		l.setStatus("desconectado")
	case *events.LoggedOut:
		l.stop("sesion_revocada", ErrSessionRevoked)
	case *events.StreamReplaced:
		l.stop("reemplazado_por_otra_instancia", ErrStreamReplaced)
	}
}

func (l *Listener) setStatus(state string) {
	if l.opts.Status == nil {
		return
	}
	l.opts.Status(map[string]string{"estado": state, "ultimo_evento": time.Now().UTC().Format(time.RFC3339)})
}

func (l *Listener) stop(state string, err error) {
	l.setStatus(state)
	select {
	case l.fatal <- err:
	default:
	}
}

// snapshotGroups guarda los nombres de los grupos para poder elegir cuáles vigilar.
func (l *Listener) snapshotGroups() {
	if l.opts.Groups == nil {
		return
	}
	go func() {
		groups, err := l.cli.GetJoinedGroups(l.ctx)
		if err != nil {
			l.log.Warn("no se pudo listar grupos", "err", err)
			return
		}
		l.opts.Groups(l.ctx, groups)
	}()
}

// onHistoryNotification: sin lista blanca se guarda el aviso (sin descargar ni leer); con lista blanca se descarga y se
// filtra. Si la descarga falla, se guarda para reintentar.
func (l *Listener) onHistoryNotification(notif *waE2E.HistorySyncNotification) {
	if l.hold.Load() {
		l.saveHistory(notif)
		return
	}
	go func() {
		if err := l.processHistory(l.ctx, notif); err != nil {
			l.log.Warn("no se pudo descargar un bloque de historial; queda pendiente", "err", err)
			l.saveHistory(notif)
		}
	}()
}

func (l *Listener) saveHistory(notif *waE2E.HistorySyncNotification) {
	if err := l.opts.Vault.Save(notif); err != nil {
		l.log.Error("no se pudo guardar el aviso de historial", "err", err)
	}
}

// processHistory descarga un bloque de historial y pasa cada mensaje por la lista blanca.
func (l *Listener) processHistory(ctx context.Context, notif *waE2E.HistorySyncNotification) error {
	data, err := l.cli.DownloadHistorySync(ctx, notif, false)
	if err != nil {
		return err
	}
	l.onHistory(&events.HistorySync{Data: data})
	return nil
}

// drainHistory procesa el historial guardado (solo con lista blanca).
func (l *Listener) drainHistory() {
	if l.hold.Load() {
		return
	}
	go func() {
		if err := l.opts.Vault.Drain(l.ctx, l.processHistory); err != nil {
			l.log.Warn("historial pendiente sin procesar; se reintenta en la próxima conexión", "err", err)
		}
	}()
}

// Run procesa la cola hasta que ctx termine (y atiende el chat propio en paralelo).
func (l *Listener) Run() {
	go l.runSelf()
	for {
		select {
		case <-l.ctx.Done():
			return
		case j := <-l.jobs:
			l.process(j)
		}
	}
}

// process reintenta con espera creciente (la base puede estar tomada por otro proceso un momento).
func (l *Listener) process(j job) {
	backoff := firstBackoff
	for try := 1; try <= maxTries; try++ {
		err := l.opts.Ingest.Handle(l.ctx, j.msg, j.fetch)
		if err == nil || l.ctx.Err() != nil {
			return
		}
		l.log.Warn("no se pudo procesar un mensaje; se reintenta", "intento", try, "err", err)
		wait(l.ctx, backoff)
		backoff = nextBackoff(backoff)
	}
	l.log.Error("mensaje descartado tras reintentos; sigue en el historial de WhatsApp",
		"chat", domain.MaskPhone(j.msg.ChatPhone), "id", j.msg.MessageID)
}

// nextBackoff duplica la espera hasta maxBackoff.
func nextBackoff(d time.Duration) time.Duration {
	return min(d*2, maxBackoff)
}

func wait(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

// fetcher descarga un adjunto; si expiró en el servidor, pide al teléfono propio volver a subirlo.
func (l *Listener) fetcher(info types.MessageInfo, msg whatsmeow.DownloadableMessage) ports.AttachmentFetcher {
	return func(ctx context.Context) ([]byte, error) {
		data, err := l.cli.Download(ctx, msg)
		if !expiredMedia(err) {
			return data, err
		}
		l.log.Info("adjunto expirado en el servidor; se pide re-subida al teléfono", "id", info.ID)
		return l.phone.reupload(ctx, info, msg)
	}
}

// RequestHistory pide al teléfono propio los mensajes anteriores al ancla (para recuperar adjuntos faltantes).
func (l *Listener) RequestHistory(ctx context.Context, anchor types.MessageInfo, count int) error {
	return l.phone.requestHistory(ctx, anchor, count)
}
