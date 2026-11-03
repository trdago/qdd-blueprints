package memstore

import (
	"context"
	"time"

	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/domain"
)

// InsertAction guarda una propuesta y devuelve su id.
func (s *Store) InsertAction(_ context.Context, a domain.Action) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a.ID = int64(len(s.actions) + 1)
	s.actions = append(s.actions, a)
	return a.ID, nil
}

// ActionByMessage busca la propuesta mostrada en ese mensaje.
func (s *Store) ActionByMessage(_ context.Context, messageID string) (domain.Action, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.actions {
		if a.MessageID == messageID && messageID != "" {
			return a, true, nil
		}
	}
	return domain.Action{}, false, nil
}

// ClaimAction pasa una propuesta pendiente y vigente a «ejecutando». Solo la primera llamada gana (atómico).
func (s *Store) ClaimAction(_ context.Context, id int64, by string, at time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.action(id)
	if a == nil || !a.Approvable(at) {
		return false, nil
	}
	a.State, a.ApprovedBy, a.ApprovedAt = domain.ActionRunning, by, &at
	return true, nil
}

// FinishAction cierra una acción con su resultado.
func (s *Store) FinishAction(_ context.Context, id int64, state, result string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if a := s.action(id); a != nil {
		a.State, a.Result = state, result
	}
	return nil
}

// ExpireActions vence las propuestas pendientes cuyo plazo pasó.
func (s *Store) ExpireActions(_ context.Context, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.actions {
		if s.actions[i].State == domain.ActionPending && !now.Before(s.actions[i].ExpiresAt) {
			s.actions[i].State = domain.ActionExpired
		}
	}
	return nil
}

// Action devuelve una copia de la acción (para pruebas).
func (s *Store) Action(id int64) (domain.Action, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if a := s.action(id); a != nil {
		return *a, true
	}
	return domain.Action{}, false
}

// action requiere s.mu.
func (s *Store) action(id int64) *domain.Action {
	for i := range s.actions {
		if s.actions[i].ID == id {
			return &s.actions[i]
		}
	}
	return nil
}
