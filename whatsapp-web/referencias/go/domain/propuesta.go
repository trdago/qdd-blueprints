package domain

import "time"

// ProposalStatus es el estado de revisión de una propuesta.
type ProposalStatus string

// Estados de una propuesta: nada pasa a la documentación de la empresa sin que la persona la confirme.
const (
	ProposalPending   ProposalStatus = "pendiente"
	ProposalConfirmed ProposalStatus = "confirmada"
	ProposalDiscarded ProposalStatus = "descartada"
)

// Proposal es un registro normalizado desde un mensaje que la persona debe confirmar.
type Proposal struct {
	ChatJID          string
	MessageID        string
	CompanyID        string
	Counterparty     string
	Category         Category
	DocType          string
	Direction        string // emitida | recibida (si se reconoce el RUT propio)
	RUT              string
	Folio            string
	Amount           *Money
	IssuedOn         *time.Time
	DueOn            *time.Time
	AttachmentSHA256 string
	Citations        map[string]string // campo → fragmento exacto del que salió
	Status           ProposalStatus
	CreatedAt        time.Time
}

// ProposalInput es lo necesario para normalizar un mensaje clasificado.
type ProposalInput struct {
	Message          IncomingMessage
	Contact          Contact
	Category         Category
	AttachmentText   string
	AttachmentSHA256 string
	Now              time.Time
	OwnRUT           string // RUT de la propia empresa: decide emitida/recibida y nunca se propone como contraparte
}

// BuildProposal normaliza sin IA: tipo, RUT, folio, monto y fechas, cada uno con su cita.
func BuildProposal(in ProposalInput) Proposal {
	text := in.Message.Text + "\n" + in.AttachmentText
	p := Proposal{
		ChatJID:          in.Message.ChatJID,
		MessageID:        in.Message.MessageID,
		CompanyID:        in.Contact.CompanyID,
		Counterparty:     in.Contact.CompanyName,
		Category:         in.Category,
		DocType:          DocKind(attachmentName(in.Message), text),
		AttachmentSHA256: in.AttachmentSHA256,
		Citations:        map[string]string{},
		Status:           ProposalPending,
		CreatedAt:        in.Now,
	}
	ruts := ExtractRUTs(text)
	p.Direction = direction(ruts, in.OwnRUT)
	p = withRUT(p, otherThan(ruts, in.OwnRUT))
	p = withFolio(p, text)
	p = withAmount(p, preferTotal(text))
	return withDates(p, ExtractDates(text, in.Message.SentAt))
}

// direction: en un documento tributario el primer RUT es el del emisor.
func direction(ruts []Cited, own string) string {
	if own == "" || len(ruts) == 0 {
		return ""
	}
	if ruts[0].Value == own {
		return "emitida"
	}
	return "recibida"
}

func otherThan(ruts []Cited, own string) []Cited {
	out := make([]Cited, 0, len(ruts))
	for _, r := range ruts {
		if r.Value != own {
			out = append(out, r)
		}
	}
	return out
}

func preferTotal(text string) []Money {
	if total, ok := ExtractTotal(text); ok {
		return []Money{total}
	}
	return ExtractAmounts(text)
}

func attachmentName(msg IncomingMessage) string {
	if msg.Attachment == nil {
		return ""
	}
	return msg.Attachment.FileName
}

func withRUT(p Proposal, ruts []Cited) Proposal {
	if len(ruts) == 0 {
		return p
	}
	p.RUT = ruts[0].Value
	p.Citations["rut"] = ruts[0].Cite
	return p
}

func withFolio(p Proposal, text string) Proposal {
	folio, ok := ExtractFolio(text)
	if !ok {
		return p
	}
	p.Folio = folio.Value
	p.Citations["folio"] = folio.Cite
	return p
}

func withAmount(p Proposal, amounts []Money) Proposal {
	if len(amounts) == 0 {
		return p
	}
	p.Amount = &amounts[0]
	p.Citations["monto"] = amounts[0].Cite
	return p
}

func withDates(p Proposal, facts DateFacts) Proposal {
	if facts.Issued != nil {
		p.IssuedOn = &facts.Issued.Date
		p.Citations["emision"] = facts.Issued.Cite
	}
	if facts.Due != nil {
		p.DueOn = &facts.Due.Date
		p.Citations["vencimiento"] = facts.Due.Cite
	}
	return p
}
