package usecases

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/domain"
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/ports"
)

// Executor corre una acción ya aprobada y devuelve el detalle de su resultado.
type Executor interface {
	Execute(ctx context.Context, a domain.Action) (string, error)
}

// ExecutorFunc adapta una función a Executor.
type ExecutorFunc func(ctx context.Context, a domain.Action) (string, error)

// Execute implementa Executor.
func (f ExecutorFunc) Execute(ctx context.Context, a domain.Action) (string, error) { return f(ctx, a) }

// ApprovalsDeps son las dependencias del motor de aprobación.
type ApprovalsDeps struct {
	Store     ports.ActionStore
	Out       ports.SelfMessenger
	Executors map[string]Executor // por tipo de acción
	Now       func() time.Time
	Log       *slog.Logger
}

// Approvals propone acciones en el chat propio y las ejecuta SOLO con un 👍 de la persona sobre el mensaje de la propuesta,
// antes de que venza y una sola vez. Un texto («sí», «dale», «ok») nunca ejecuta nada.
type Approvals struct{ deps ApprovalsDeps }

// NewApprovals crea el motor.
func NewApprovals(deps ApprovalsDeps) *Approvals {
	if deps.Executors == nil {
		deps.Executors = map[string]Executor{}
	}
	return &Approvals{deps: deps}
}

// Register agrega un ejecutor para un tipo de acción. Se llama al armar el proceso, antes de atender mensajes.
func (p *Approvals) Register(kind string, ex Executor) {
	p.deps.Executors[kind] = ex
}

var errNoExecutor = errors.New("no hay ejecutor para este tipo de acción")

// Propose escribe la propuesta en un mensaje propio (una acción por mensaje) y la guarda con el id de ese mensaje.
func (p *Approvals) Propose(ctx context.Context, kind, summary string, payload []byte, ttl time.Duration) (domain.Action, error) {
	if _, ok := p.deps.Executors[kind]; !ok {
		return domain.Action{}, fmt.Errorf("%w: %q", errNoExecutor, kind)
	}
	now := p.deps.Now()
	a := domain.Action{Kind: kind, Summary: summary, Payload: payload, State: domain.ActionPending, CreatedAt: now, ExpiresAt: now.Add(ttl)}
	id, err := p.deps.Out.SendToSelfID(ctx, domain.FormatProposal(a))
	if err != nil {
		return a, err
	}
	a.MessageID = id
	a.ID, err = p.deps.Store.InsertAction(ctx, a)
	return a, err
}

// HandleText: ninguna propuesta se aprueba por texto. handled=false siempre.
func (p *Approvals) HandleText(context.Context, string) (bool, error) { return false, nil }

// HandleReaction: un 👍 sobre el mensaje de una propuesta vigente la reclama (atómico) y la ejecuta. Lo demás se ignora.
func (p *Approvals) HandleReaction(ctx context.Context, targetID, emoji string) error {
	if !domain.IsLike(emoji) || targetID == "" {
		return nil
	}
	a, ok, err := p.claim(ctx, targetID)
	if err != nil || !ok {
		return err
	}
	return p.run(ctx, a)
}

func (p *Approvals) claim(ctx context.Context, targetID string) (domain.Action, bool, error) {
	now := p.deps.Now()
	a, found, err := p.deps.Store.ActionByMessage(ctx, targetID)
	if err != nil || !found || !a.Approvable(now) {
		return a, false, err
	}
	claimed, err := p.deps.Store.ClaimAction(ctx, a.ID, domain.ApprovedByLike, now)
	return a, claimed, err
}

// run ejecuta, cierra la acción y avisa SIEMPRE el resultado en el chat propio (bien o mal).
func (p *Approvals) run(ctx context.Context, a domain.Action) error {
	detail, err := p.deps.Executors[a.Kind].Execute(ctx, a)
	a.State, a.Result = domain.ActionDone, detail
	if err != nil {
		a.State, a.Result = domain.ActionFailed, err.Error()
	}
	if ferr := p.deps.Store.FinishAction(ctx, a.ID, a.State, a.Result); ferr != nil {
		p.deps.Log.Warn("no se pudo cerrar la acción", "id", a.ID, "err", ferr)
	}
	_, serr := p.deps.Out.SendToSelfID(ctx, domain.FormatResult(a))
	return serr
}

// Expire vence las propuestas cuyo plazo pasó (también al arrancar).
func (p *Approvals) Expire(ctx context.Context) error {
	return p.deps.Store.ExpireActions(ctx, p.deps.Now())
}
