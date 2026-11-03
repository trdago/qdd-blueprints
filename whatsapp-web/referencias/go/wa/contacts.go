package wa

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/trdago/qdd-blueprints/whatsapp-web/referencias/domain"
)

// FoundContact es un contacto de la libreta sincronizada por el teléfono (solo nombre y número).
type FoundContact struct {
	Name      string
	PhoneE164 string
}

// LookupContacts busca por nombre (sin distinguir mayúsculas) en la libreta de la sesión, abriendo
// session.db en solo lectura. No lee mensajes: whatsmeow solo guarda ahí nombres y números.
func LookupContacts(ctx context.Context, sessionPath, name string) ([]FoundContact, error) {
	db, err := sql.Open("sqlite3", "file:"+sessionPath+"?mode=ro&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.QueryContext(ctx, `
		SELECT coalesce(nullif(c.full_name, ''), nullif(c.push_name, ''), c.first_name),
		       CASE WHEN c.their_jid LIKE '%@lid' THEN coalesce(m.pn, '') ELSE c.their_jid END
		FROM whatsmeow_contacts c LEFT JOIN whatsmeow_lid_map m ON c.their_jid = m.lid || '@lid'
		WHERE lower(?) IN (lower(c.full_name), lower(c.push_name), lower(c.first_name))`, strings.TrimSpace(name))
	if err != nil {
		return nil, fmt.Errorf("leer libreta de contactos: %w", err)
	}
	defer rows.Close()
	return collectContacts(rows)
}

func collectContacts(rows *sql.Rows) ([]FoundContact, error) {
	var out []FoundContact
	for rows.Next() {
		var c FoundContact
		var jid string
		if err := rows.Scan(&c.Name, &jid); err != nil {
			return nil, err
		}
		out = appendFound(out, c, jid)
	}
	return out, rows.Err()
}

func appendFound(out []FoundContact, c FoundContact, jid string) []FoundContact {
	user, _, _ := strings.Cut(jid, "@")
	phone, ok := domain.NormalizePhoneE164("+" + user)
	if !ok {
		return out
	}
	c.PhoneE164 = phone
	return append(out, c)
}
