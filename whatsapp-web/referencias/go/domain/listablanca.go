package domain

import (
	"sort"
	"time"
)

// Modos de vigilancia de un contacto.
const (
	WatchFull     = "completo"        // texto y adjuntos
	WatchDocsOnly = "solo_documentos" // solo mensajes con adjunto; el texto se descarta sin guardarse
)

// WatchModes son los modos válidos.
var WatchModes = map[string]bool{WatchFull: true, WatchDocsOnly: true}

// Contact es una entrada de la lista blanca: un teléfono (o un grupo agregado a mano) y a qué empresa pertenece.
type Contact struct {
	PhoneE164   string
	ChatJID     string // solo para grupos agregados a mano
	CompanyID   string
	CompanyName string
	ContactName string
	Mode        string
}

// SkipsText: en modo solo documentos, un mensaje sin adjunto se descarta.
func (c Contact) SkipsText(msg IncomingMessage) bool {
	return c.Mode == WatchDocsOnly && msg.Attachment == nil
}

// Directory es la lista blanca: solo los chats con estos teléfonos (o estos grupos) se procesan.
type Directory struct {
	byPhone  map[string]Contact
	groups   map[string]Contact
	SyncedAt time.Time
}

// NewDirectory indexa los contactos. Un teléfono en varias empresas queda con la primera por CompanyID (determinista).
func NewDirectory(contacts []Contact, syncedAt time.Time) Directory {
	sorted := append([]Contact(nil), contacts...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].CompanyID < sorted[j].CompanyID })
	d := Directory{byPhone: map[string]Contact{}, groups: map[string]Contact{}, SyncedAt: syncedAt}
	for _, c := range sorted {
		d.add(c)
	}
	return d
}

func (d Directory) add(c Contact) {
	if c.ChatJID != "" {
		d.groups[c.ChatJID] = c
		return
	}
	phone, ok := NormalizePhoneE164(c.PhoneE164)
	if _, taken := d.byPhone[phone]; !ok || taken {
		return
	}
	c.PhoneE164 = phone
	d.byPhone[phone] = c
}

// Size es cuántos chats vigila (teléfonos más grupos).
func (d Directory) Size() int {
	return len(d.byPhone) + len(d.groups)
}

// Match dice si el mensaje es de un chat vigilado. Se llama ANTES de guardar, descargar o clasificar.
func (d Directory) Match(msg IncomingMessage) (Contact, bool) {
	if msg.IsGroup {
		c, ok := d.groups[msg.ChatJID]
		return c, ok
	}
	c, ok := d.byPhone[msg.ChatPhone]
	return c, ok && msg.ChatPhone != ""
}
