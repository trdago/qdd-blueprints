// Package memstore implementa los puertos de almacenamiento en memoria, para pruebas y ejemplos. En producción la cola
// de salida y las propuestas van en la base del proyecto (sobreviven a un reinicio y otro proceso puede encolar).
package memstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"sync"
	"time"

	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/domain"
	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/ports"
)

// Store guarda mensajes, clasificaciones, propuestas, adjuntos, la cola de salida y las acciones.
type Store struct {
	mu          sync.Mutex
	Messages    map[string]ports.StoredMessage
	Classified  map[string]domain.Classification
	Proposals   []domain.Proposal
	Media       map[string][]byte
	classifyDay map[string]int
	outbox      []outboxRow
	actions     []domain.Action
}

type outboxRow struct {
	item   domain.QueuedDelivery
	status string
	err    string
}

var (
	_ ports.MessageStore = (*Store)(nil)
	_ ports.MediaStore   = (*Store)(nil)
	_ ports.OutboxStore  = (*Store)(nil)
	_ ports.ActionStore  = (*Store)(nil)
)

// New crea un almacenamiento vacío.
func New() *Store {
	return &Store{Messages: map[string]ports.StoredMessage{}, Classified: map[string]domain.Classification{},
		Media: map[string][]byte{}, classifyDay: map[string]int{}}
}

// InsertMessage es idempotente por (chat, id del mensaje).
func (s *Store) InsertMessage(_ context.Context, m ports.StoredMessage) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := m.Message.Key()
	if _, ok := s.Messages[key]; ok {
		return false, nil
	}
	s.Messages[key] = m
	return true, nil
}

// ReserveClassification consume un cupo del día; false si ya no quedan.
func (s *Store) ReserveClassification(_ context.Context, day time.Time, dailyMax int) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := day.UTC().Format(time.DateOnly)
	if s.classifyDay[key] >= dailyMax {
		return false, nil
	}
	s.classifyDay[key]++
	return true, nil
}

// SetClassification guarda la categoría y la confianza de un mensaje.
func (s *Store) SetClassification(_ context.Context, chatJID, messageID string, c domain.Classification, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Classified[chatJID+"/"+messageID] = c
	return nil
}

// InsertProposal guarda una propuesta pendiente.
func (s *Store) InsertProposal(_ context.Context, p domain.Proposal) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Proposals = append(s.Proposals, p)
	return nil
}

// Save guarda los bytes de un adjunto por su sha256 (idempotente por contenido).
func (s *Store) Save(_ context.Context, data []byte, _, mime string) (ports.SavedAttachment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])
	s.Media[sha] = data
	return ports.SavedAttachment{SHA256: sha, Mime: mime, Bytes: int64(len(data))}, nil
}

// Enqueue agrega entregas a la cola de salida.
func (s *Store) Enqueue(_ context.Context, items []domain.Delivery, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, it := range items {
		id := int64(len(s.outbox) + 1)
		s.outbox = append(s.outbox, outboxRow{item: domain.QueuedDelivery{Delivery: it, ID: id, QueuedAt: at}, status: domain.OutboxPending})
	}
	return nil
}

// Pending devuelve las entregas pendientes con menos de maxAttempts intentos, en orden de encolado.
func (s *Store) Pending(_ context.Context, limit, maxAttempts int) ([]domain.QueuedDelivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.QueuedDelivery
	for _, r := range s.outbox {
		if r.status == domain.OutboxPending && r.item.Attempts < maxAttempts && len(out) < limit {
			out = append(out, r.item)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// MarkSent marca una entrega como enviada.
func (s *Store) MarkSent(_ context.Context, id int64, _ time.Time) error {
	return s.updateOutbox(id, domain.OutboxSent, "")
}

// MarkFailed marca un error permanente: no se reintenta.
func (s *Store) MarkFailed(_ context.Context, id int64, reason string) error {
	return s.updateOutbox(id, domain.OutboxError, reason)
}

// MarkRetry deja la entrega pendiente para el próximo drenaje.
func (s *Store) MarkRetry(_ context.Context, id int64, reason string) error {
	return s.updateOutbox(id, domain.OutboxPending, reason)
}

func (s *Store) updateOutbox(id int64, status, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.outbox {
		if s.outbox[i].item.ID == id {
			s.outbox[i].status, s.outbox[i].err = status, reason
			s.outbox[i].item.Attempts++
		}
	}
	return nil
}

// OutboxStatus devuelve el estado y los intentos de una entrega (para pruebas).
func (s *Store) OutboxStatus(id int64) (string, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.outbox {
		if r.item.ID == id {
			return r.status, r.item.Attempts
		}
	}
	return "", 0
}
