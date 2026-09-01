# Sicherheit

Letztes Gesamtaudit: 2. September 2026.

## Ergebnis

Zum Audit-Zeitpunkt meldeten `govulncheck ./...` und `npm audit` keine
bekannten verwundbaren Abhängigkeiten. `go test ./...`, `go vet ./...`, der
Frontend-Produktionsbuild und `gosec` wurden zusätzlich ausgeführt. Die CI
wiederholt Tests, Vet, npm Audit und govulncheck bei jedem Build.

Dabei wurden folgende Probleme behoben:

- Der IMAP-Syncstand hatte historisch `account_id` statt
  `(account_id, mailbox)` als Primärschlüssel. Dadurch überschrieben sich
  Posteingang und Gesendet und Nachrichten konnten erneut erscheinen. Eine
  automatische, datenbewahrende Migration und ein Regressionstest beheben das.
- Bereits lokal vorhandene IMAP-Nachrichten werden vor dem Import geprüft;
  auch nach einem unterbrochenen Sync entstehen keine doppelten Anhänge.
- IMAP- und SMTP-Adressen verwenden `net.JoinHostPort` und funktionieren damit
  korrekt und sicher auch mit IPv6-Literalen.
- RFC-822-Nachrichten werden nur bis 50 MiB abgerufen; einzelne Anhänge sind
  auf 25 MiB begrenzt. Das verhindert unbeschränkten Speicher-/DB-Verbrauch
  durch präparierte Mails. Größere Nachrichten werden übersprungen.
- Ungültige gespeicherte IMAP-UIDs und unsichere Integer-Konvertierungen werden
  abgefangen.
- Schlägt das Speichern eines neuen Kontopassworts fehl, werden unvollständige
  Schlüsselbund- und Datenbankeinträge zurückgerollt.
- SQLite wird unter einem `0700`-Verzeichnis angelegt und auf `0600` gesetzt.
- Build, Docker und Flatpak verwenden den kanonischen `fmail/`-Quellstand,
  Lockfiles und eine gepinnte Wails-Version statt `latest` und Laufzeitgerüsten.

## Schutzmaßnahmen

- IMAP und SMTP erlauben nur TLS oder STARTTLS, mindestens TLS 1.2, mit normaler
  Zertifikatsprüfung und ohne Klartext-Fallback.
- Passwörter liegen ausschließlich im Schlüsselbund des Betriebssystems.
- SQL verwendet parametrisierte Abfragen; Maildaten werden nicht in Queries
  interpoliert.
- SMTP-Adressen und Header werden validiert bzw. von CR/LF bereinigt.
- HTML-Mail wird mit `bluemonday` sanitisiert und in einem sandboxed Iframe ohne
  Skripte gerendert. Eine eigene CSP blockiert externe Bilder, bis der Nutzer
  sie für die konkrete Mail freigibt. Links werden nur für `http`, `https` und
  `mailto` an den Systembrowser gegeben.
- Anhänge werden nicht automatisch geöffnet. Speichern erfordert einen nativen
  Dialog und erzeugt eine Datei mit Modus `0600`.
- Löschen verschiebt serverseitige IMAP-Nachrichten zuerst in den Papierkorb;
  erst bei Erfolg wird die lokale Kopie entfernt.

## Verbleibende Grenzen

- Die SQLite-Datenbank enthält vollständige Mails und Anhänge unverschlüsselt.
  Das Berechtigungsmodell schützt andere lokale Nutzer, nicht aber vor einem
  kompromittierten Benutzerkonto. Festplattenverschlüsselung wird empfohlen.
- OAuth2 ist noch nicht implementiert. Anbieter wie Microsoft 365, die Basic
  Authentication deaktiviert haben, können daher derzeit nicht genutzt werden.
- Synchronisation ist Polling statt IMAP IDLE. Netzwerkoperationen besitzen
  noch kein durchgängiges Gesamt-Timeout/Cancel-Konzept.
- Externe Bilder können nach ausdrücklicher Freigabe Tracking-Anfragen senden.
- Gespeicherte Anhänge werden nicht auf Schadsoftware geprüft.
- Wails 3 und go-imap v2 sind Beta-Abhängigkeiten. Vor Releases sollten Updates
  bewusst getestet statt automatisch über `latest` eingespielt werden.
- GitHub Actions sind auf Major-Tags statt vollständige Commit-SHAs gepinnt.
  Für ein besonders strenges Supply-Chain-Modell sollten die Actions-SHAs
  zusätzlich festgeschrieben und regelmäßig automatisiert aktualisiert werden.

Sicherheitsprobleme bitte zunächst vertraulich an den Maintainer melden und
keine echten Zugangsdaten, Mailinhalte oder Datenbanken an ein Issue anhängen.
