package usecases

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/domain"
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/memstore"
)

type approvalRig struct {
	ap    *Approvals
	store *memstore.Store
	out   *selfOut
	clock *clock
	runs  *int
}

func newApprovalRig(fail error) approvalRig {
	runs := 0
	r := approvalRig{store: memstore.New(), out: &selfOut{}, clock: &clock{now: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)}, runs: &runs}
	r.ap = NewApprovals(ApprovalsDeps{Store: r.store, Out: r.out, Now: r.clock.Now, Log: quiet})
	r.ap.Register("demo", ExecutorFunc(func(context.Context, domain.Action) (string, error) { runs++; return "listo", fail }))
	return r
}

func (r approvalRig) propose(t *testing.T) domain.Action {
	t.Helper()
	a, err := r.ap.Propose(context.Background(), "demo", "reiniciar el servicio de prueba", nil, 15*time.Minute)
	must(t, err)
	return a
}

// WAW-A01: un 👍 sobre la propuesta vigente la ejecuta una sola vez.
func TestWAW_A01_LikeEjecutaUnaVez(t *testing.T) {
	r := newApprovalRig(nil)
	a := r.propose(t)
	check(t, a.MessageID == "m1" && strings.Contains(r.out.texts[0], "👍"), "la propuesta va en su propio mensaje")
	must(t, r.ap.HandleReaction(context.Background(), a.MessageID, "👍"))
	must(t, r.ap.HandleReaction(context.Background(), a.MessageID, "👍🏻"))
	got, _ := r.store.Action(a.ID)
	check(t, *r.runs == 1 && got.State == domain.ActionDone && got.ApprovedBy == domain.ApprovedByLike, "una vez: %d %+v", *r.runs, got)
}

// WAW-A02: un «sí» escrito no ejecuta nada.
func TestWAW_A02_SiEscritoNoEjecuta(t *testing.T) {
	r := newApprovalRig(nil)
	r.propose(t)
	for _, text := range []string{"sí", "dale", "ok", "👍", "apruebo"} {
		handled, err := r.ap.HandleText(context.Background(), text)
		check(t, !handled && err == nil, "%q no es una aprobación", text)
	}
	check(t, *r.runs == 0, "nada se ejecutó")
}

// WAW-A03: una propuesta vencida no se ejecuta aunque llegue el 👍.
func TestWAW_A03_VencidaNoSeEjecuta(t *testing.T) {
	r := newApprovalRig(nil)
	a := r.propose(t)
	r.clock.now = r.clock.now.Add(16 * time.Minute)
	must(t, r.ap.HandleReaction(context.Background(), a.MessageID, "👍"))
	must(t, r.ap.Expire(context.Background()))
	got, _ := r.store.Action(a.ID)
	check(t, *r.runs == 0 && got.State == domain.ActionExpired, "vencida: %+v", got)
}

// WAW-A04: otro emoji, quitar la reacción o un 👍 sobre otro mensaje no ejecutan.
func TestWAW_A04_OtraReaccionNoEjecuta(t *testing.T) {
	r := newApprovalRig(nil)
	a := r.propose(t)
	must(t, r.ap.HandleReaction(context.Background(), a.MessageID, "❤️"))
	must(t, r.ap.HandleReaction(context.Background(), a.MessageID, ""))
	must(t, r.ap.HandleReaction(context.Background(), "otro-mensaje", "👍"))
	must(t, r.ap.HandleReaction(context.Background(), "", "👍"))
	check(t, *r.runs == 0, "nada se ejecutó")
}

// WAW-A05: tras ejecutar se avisa SIEMPRE el resultado, también si falló.
func TestWAW_A05_SiempreAvisaElResultado(t *testing.T) {
	r := newApprovalRig(errors.New("el servicio no respondió"))
	a := r.propose(t)
	must(t, r.ap.HandleReaction(context.Background(), a.MessageID, "👍"))
	got, _ := r.store.Action(a.ID)
	last := r.out.texts[len(r.out.texts)-1]
	check(t, got.State == domain.ActionFailed && strings.Contains(last, "no respondió") && domain.IsSelfReply(last), "aviso de falla: %q", last)
	_, err := r.ap.Propose(context.Background(), "desconocido", "x", nil, time.Minute)
	check(t, err != nil, "un tipo sin ejecutor no se propone")
}
