package mail

import (
	"regexp"
	"strings"
)

// ablageKeywords sind Schlagworte, die typischerweise auf eine
// transaktionale Mail hindeuten (Rechnung, Beleg, Bestätigung ...).
// Bewusst simpel gehalten und leicht erweiterbar — keine Magie, die
// sich der Nutzer nicht erklären kann.
var ablageKeywords = []string{
	"rechnung", "beleg", "bestätigung", "bestellung", "quittung",
	"zahlung", "abrechnung", "abo", "vertrag", "kündigung",
	"invoice", "receipt", "order confirmation", "subscription", "payment",
}

var ablageSenderPattern = regexp.MustCompile(`(?i)^(no[-_]?reply|noreply|rechnung|billing|invoice|payment)[@.]`)

// IsAblage entscheidet anhand von Betreff und Absenderadresse, ob eine
// Mail eher in die Ablage (Belege/Rechnungen) als in Post gehört.
func IsAblage(senderEmail, subject string) bool {
	if ablageSenderPattern.MatchString(strings.TrimSpace(senderEmail)) {
		return true
	}
	lowerSubject := strings.ToLower(subject)
	for _, kw := range ablageKeywords {
		if strings.Contains(lowerSubject, kw) {
			return true
		}
	}
	return false
}
