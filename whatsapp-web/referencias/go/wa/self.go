package wa

import (
	"context"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/domain"
)

// SelfHandler es lo que el listener necesita del canal del chat propio.
type SelfHandler interface {
	HandleText(ctx context.Context, text string) error
	// HandleReaction recibe la reacción de la persona sobre un mensaje del chat propio (emoji vacío = quitó la reacción).
	HandleReaction(ctx context.Context, targetID, emoji string) error
}

type selfJob struct {
	text     string
	reaction *selfReaction
}

// selfReaction es una reacción: el id del mensaje reaccionado y el emoji.
type selfReaction struct{ target, emoji string }

func reactionOf(m *waE2E.Message) (selfReaction, bool) {
	r := m.GetReactionMessage()
	if r == nil {
		return selfReaction{}, false
	}
	return selfReaction{target: r.GetKey().GetID(), emoji: r.GetText()}, true
}

// isSelfChat: chat 1:1 con uno mismo (por número o por LID) y escrito por la propia persona.
func isSelfChat(info types.MessageInfo, ownPN, ownLID types.JID) bool {
	if info.IsGroup || !info.IsFromMe {
		return false
	}
	return sameUser(info.Chat, ownPN) || sameUser(info.Chat, ownLID)
}

func sameUser(a, b types.JID) bool {
	return b.User != "" && a.User == b.User && a.Server == b.Server
}

// ignoredSelf: historial, mensajes de este mismo dispositivo vinculado o respuestas del agente.
func ignoredSelf(info types.MessageInfo, own types.JID, text string, source domain.Source) bool {
	return source != domain.SourceRealtime || info.Sender.Device == own.Device || domain.IsSelfReply(text)
}

// routeSelf desvía los mensajes del chat propio al canal del agente. true = el mensaje ya se manejó (nunca pasa a la
// ingesta de la lista blanca).
func (l *Listener) routeSelf(evt *events.Message, source domain.Source) bool {
	own := l.cli.Store.ID
	if l.opts.Self == nil || own == nil || !isSelfChat(evt.Info, *own, l.cli.Store.LID) {
		return false
	}
	if ignoredSelf(evt.Info, *own, messageText(evt.Message), source) {
		return true
	}
	l.enqueueSelf(selfJobOf(evt))
	return true
}

func selfJobOf(evt *events.Message) selfJob {
	if r, ok := reactionOf(evt.Message); ok {
		return selfJob{reaction: &r}
	}
	return selfJob{text: messageText(evt.Message)}
}

func (l *Listener) enqueueSelf(j selfJob) {
	select {
	case l.selfJobs <- j:
	case <-l.ctx.Done():
	}
}

// runSelf atiende el chat propio en orden, aparte de la ingesta (una respuesta puede tardar).
func (l *Listener) runSelf() {
	for {
		select {
		case <-l.ctx.Done():
			return
		case j := <-l.selfJobs:
			l.processSelf(j)
		}
	}
}

func (l *Listener) processSelf(j selfJob) {
	if err := l.handleSelf(j); err != nil {
		l.log.Warn("no se pudo atender el chat propio", "err", err)
	}
}

func (l *Listener) handleSelf(j selfJob) error {
	if j.reaction != nil {
		return l.opts.Self.HandleReaction(l.ctx, j.reaction.target, j.reaction.emoji)
	}
	if j.text == "" {
		return nil
	}
	return l.opts.Self.HandleText(l.ctx, j.text)
}
