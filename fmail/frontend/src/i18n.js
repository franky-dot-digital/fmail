// i18n.js — Übersetzungswörterbuch + t()-Helfer.
//
// Deckt die Oberfläche ab (Tabs, Buttons, Formulare, Statusmeldungen).
// NICHT übersetzt: Fehlermeldungen, die direkt vom Go-Backend kommen
// (z. B. IMAP/SMTP-Fehler, Kontoformular-Validierung) — die sind auf
// Deutsch fest im Backend formuliert. Eine vollständige Backend-i18n
// hätte bedeutet, jede einzelne Fehlermeldung in mailservice.go,
// accountservice.go und internal/mail/* durch Fehlercodes zu ersetzen,
// die das Frontend dann übersetzt — das hätte den Umfang dieser Änderung
// um ein Vielfaches gesprengt. Wo ein Backend-Fehler angezeigt wird,
// bleibt er also Deutsch, auch bei englischer Oberfläche.
const translations = {
    de: {
        tabs: {
            post: 'Post',
            ablage: 'Ablage',
            gesendet: 'Gesendet',
            pfoertner: 'Pförtner',
            einstellungen: 'Einstellungen',
        },
        search: {
            placeholder: 'Suchen …',
            aria: 'Post/Ablage/Pförtner durchsuchen',
        },
        post: {
            title: 'Post',
            sub: 'Neu für dich – alles, was direkt in deine Inbox gehört.',
        },
        ablage: {
            title: 'Ablage',
            sub: 'Belege, Rechnungen und Bestätigungen. Zum Nachschlagen, nicht zum Abarbeiten.',
        },
        gesendet: {
            title: 'Gesendet',
            sub: 'Mails, die du verschickt hast — landen zusätzlich im Gesendet-Ordner deines Servers.',
        },
        mail: {
            new: 'Neu',
            showRead: 'Gelesene anzeigen',
            onlyUnread: 'Nur Ungelesene',
        },
        sync: {
            now: 'Jetzt synchronisieren',
            inProgress: 'Synchronisiere …',
            resultPrefix: 'Synchronisierung: ',
        },
        empty: {
            nothingHereTitle: 'Nichts hier',
            nothingHereSub: 'Diese Ansicht ist gerade leer.',
            noOneWaitingTitle: 'Niemand wartet',
            noOneWaitingSub: 'Neue Absender erscheinen hier nach dem nächsten Sync.',
            noAccountTitle: 'Noch kein Konto',
            noAccountSub: 'Leg dein erstes Konto an, um Post zu empfangen.',
        },
        pfoertner: {
            title: 'Pförtner',
            sub: 'Diese Absender schreiben dir zum ersten Mal. Du entscheidest, ob sie künftig deine Post erreichen — bei „Sperren" landet nichts mehr von ihnen in der Inbox oder hier.',
            approve: 'Ja',
            block: 'Nein',
            approveTitle: 'Freigeben',
            blockTitle: 'Sperren',
        },
        toast: {
            approved: 'Freigegeben — landet ab jetzt in Post',
            blocked: 'Gesperrt — erreicht dich künftig nicht mehr',
            movedToAblage: 'In Ablage verschoben',
            movedToPost: 'Zurück nach Post verschoben',
            mailDeleted: 'Mail gelöscht',
            mailSent: 'Mail gesendet',
            accountUpdated: 'Konto aktualisiert',
            accountDeleted: 'Konto gelöscht',
            attachmentSaved: 'Gespeichert: {path}',
        },
        detail: {
            back: 'Zurück',
            loadImages: 'Bilder laden',
            saveAttachment: 'Speichern',
            reply: 'Antworten',
            forward: 'Weiterleiten',
            moveToAblage: 'In Ablage verschieben',
            moveBackToPost: 'Zurück nach Post',
            delete: 'Löschen',
            toLabel: 'An:',
        },
        confirmDeleteMail: {
            title: 'Mail löschen',
            message: 'Diese Mail wirklich löschen? Sie bleibt auf dem Server erhalten, verschwindet aber aus f:mail.',
        },
        confirmDeleteAccount: {
            title: 'Konto löschen',
            message: '„{label}" wirklich löschen? Alle lokal gespeicherten Mails dieses Kontos werden entfernt.',
        },
        confirmModal: {
            delete: 'Löschen',
            cancel: 'Abbrechen',
        },
        compose: {
            reply: 'Antworten',
            forward: 'Weiterleiten',
            new: 'Neue Mail',
            from: 'Von',
            to: 'An',
            toPlaceholder: 'empfänger@example.com',
            subject: 'Betreff',
            subjectPlaceholder: '(kein Betreff)',
            bodyPlaceholder: 'Deine Nachricht …',
            attach: 'Anhang',
            removeAttachment: 'Anhang entfernen',
            send: 'Senden',
            format: 'Format',
            modePlain: 'Nur Text',
            modeHtml: 'HTML',
            quoteHeader: '— Am {date} schrieb {sender} <{email}>: —',
            replyPrefix: 'Re: ',
            forwardPrefix: 'Fwd: ',
        },
        status: {
            readingAttachments: 'Lese Anhänge …',
            sending: 'Sende …',
            failedPrefix: 'Fehlgeschlagen: ',
            attachmentReadFailedPrefix: 'Anhang konnte nicht gelesen werden: ',
            attachmentsTooLarge: 'Anhänge sind zusammen zu groß ({size}, Limit 20 MB)',
            testing: 'Teste Verbindung …',
            testSuccessSuffix: 'Verbindung erfolgreich.',
            testFailedSuffix: 'fehlgeschlagen: ',
            savingAccount: 'Speichere Konto …',
            savingChanges: 'Speichere Änderungen …',
            syncingAfterCreate: 'Konto gespeichert. Synchronisiere …',
        },
        settings: {
            title: 'Einstellungen',
            general: 'Allgemein',
            appearance: 'Erscheinungsbild',
            appearanceSub: 'Hell, dunkel oder wie im System eingestellt.',
            language: 'Sprache',
            languageSub: 'Sprache der Oberfläche.',
            accounts: 'Konten',
            addAccount: 'Neues Konto hinzufügen',
            createAccount: 'Konto anlegen',
        },
        theme: { system: 'System', light: 'Hell', dark: 'Dunkel' },
        lang: { de: 'Deutsch', en: 'English' },
        account: { edit: 'Bearbeiten', delete: 'Löschen' },
        accountModal: {
            editTitle: '„{label}" bearbeiten',
            newTitle: 'Neues Konto',
            close: 'Schließen',
            saveChanges: 'Änderungen speichern',
            saveAndSync: 'Speichern & synchronisieren',
        },
        field: {
            label: 'Label',
            labelPlaceholder: 'Privat',
            email: 'E-Mail-Adresse',
            emailPlaceholder: 'ich@example.com',
            displayName: 'Anzeigename ("Von")',
            displayNamePlaceholder: 'z. B. Frank Lewandowski',
            syncInterval: 'Automatisch abrufen',
            host: 'Host',
            port: 'Port',
            security: 'Sicherheit',
            username: 'Benutzername',
            usernamePlaceholderEmail: 'wie E-Mail-Adresse, falls leer',
            usernamePlaceholderImap: 'wie IMAP, falls leer',
            password: 'Passwort',
            passwordKeepPlaceholder: 'Leer lassen, um das gespeicherte Passwort zu behalten',
        },
        syncInterval: {
            manual: 'Nur manuell',
            m5: 'Alle 5 Minuten',
            m15: 'Alle 15 Minuten',
            m30: 'Alle 30 Minuten',
            hourly: 'Stündlich',
        },
        section: { imap: 'IMAP (Empfang)', smtp: 'SMTP (Versand)' },
        button: { testImap: 'IMAP-Verbindung testen', testSmtp: 'SMTP-Verbindung testen' },
        alert: {
            accountLoadFailedPrefix: 'Konto konnte nicht geladen werden: ',
            accountDeleteFailedPrefix: 'Löschen fehlgeschlagen: ',
            mailDeleteFailedPrefix: 'Löschen fehlgeschlagen: ',
            moveFailedPrefix: 'Verschieben fehlgeschlagen: ',
            attachmentSaveFailedPrefix: 'Anhang konnte nicht gespeichert werden: ',
        },
        editor: {
            bold: 'Fett',
            italic: 'Kursiv',
            underline: 'Unterstrichen',
            bulletList: 'Aufzählung',
            numberedList: 'Nummerierte Liste',
            link: 'Link einfügen',
            linkPrompt: 'Ziel-URL des Links:',
            clearFormat: 'Formatierung entfernen',
        },
    },
    en: {
        tabs: {
            post: 'Inbox',
            ablage: 'Archive',
            gesendet: 'Sent',
            pfoertner: 'Gatekeeper',
            einstellungen: 'Settings',
        },
        search: {
            placeholder: 'Search …',
            aria: 'Search Inbox/Archive/Gatekeeper',
        },
        post: {
            title: 'Inbox',
            sub: 'New for you – everything that belongs straight in your inbox.',
        },
        ablage: {
            title: 'Archive',
            sub: 'Receipts, invoices and confirmations. For reference, not for action.',
        },
        gesendet: {
            title: 'Sent',
            sub: 'Emails you sent — also filed in your server’s Sent folder.',
        },
        mail: {
            new: 'New',
            showRead: 'Show read',
            onlyUnread: 'Unread only',
        },
        sync: {
            now: 'Sync now',
            inProgress: 'Syncing …',
            resultPrefix: 'Sync: ',
        },
        empty: {
            nothingHereTitle: 'Nothing here',
            nothingHereSub: 'This view is currently empty.',
            noOneWaitingTitle: 'No one waiting',
            noOneWaitingSub: 'New senders appear here after the next sync.',
            noAccountTitle: 'No account yet',
            noAccountSub: 'Set up your first account to start receiving mail.',
        },
        pfoertner: {
            title: 'Gatekeeper',
            sub: 'These senders are writing to you for the first time. You decide whether they reach your inbox from now on — "Block" means nothing more from them ever lands in your inbox or here.',
            approve: 'Yes',
            block: 'No',
            approveTitle: 'Approve',
            blockTitle: 'Block',
        },
        toast: {
            approved: 'Approved — now lands in Inbox',
            blocked: "Blocked — won't reach you anymore",
            movedToAblage: 'Moved to Archive',
            movedToPost: 'Moved back to Inbox',
            mailDeleted: 'Email deleted',
            mailSent: 'Email sent',
            accountUpdated: 'Account updated',
            accountDeleted: 'Account deleted',
            attachmentSaved: 'Saved: {path}',
        },
        detail: {
            back: 'Back',
            loadImages: 'Load images',
            saveAttachment: 'Save',
            reply: 'Reply',
            forward: 'Forward',
            moveToAblage: 'Move to Archive',
            moveBackToPost: 'Move back to Inbox',
            delete: 'Delete',
            toLabel: 'To:',
        },
        confirmDeleteMail: {
            title: 'Delete email',
            message: 'Really delete this email? It stays on the server, but disappears from f:mail.',
        },
        confirmDeleteAccount: {
            title: 'Delete account',
            message: 'Really delete "{label}"? All locally stored emails for this account will be removed.',
        },
        confirmModal: {
            delete: 'Delete',
            cancel: 'Cancel',
        },
        compose: {
            reply: 'Reply',
            forward: 'Forward',
            new: 'New email',
            from: 'From',
            to: 'To',
            toPlaceholder: 'recipient@example.com',
            subject: 'Subject',
            subjectPlaceholder: '(no subject)',
            bodyPlaceholder: 'Your message …',
            attach: 'Attach',
            removeAttachment: 'Remove attachment',
            send: 'Send',
            format: 'Format',
            modePlain: 'Plain text',
            modeHtml: 'HTML',
            quoteHeader: '— On {date}, {sender} <{email}> wrote: —',
            replyPrefix: 'Re: ',
            forwardPrefix: 'Fwd: ',
        },
        status: {
            readingAttachments: 'Reading attachments …',
            sending: 'Sending …',
            failedPrefix: 'Failed: ',
            attachmentReadFailedPrefix: "Couldn't read attachment: ",
            attachmentsTooLarge: 'Attachments are too large together ({size}, limit 20 MB)',
            testing: 'Testing connection …',
            testSuccessSuffix: 'Connection successful.',
            testFailedSuffix: 'failed: ',
            savingAccount: 'Saving account …',
            savingChanges: 'Saving changes …',
            syncingAfterCreate: 'Account saved. Syncing …',
        },
        settings: {
            title: 'Settings',
            general: 'General',
            appearance: 'Appearance',
            appearanceSub: 'Light, dark, or match the system.',
            language: 'Language',
            languageSub: 'Interface language.',
            accounts: 'Accounts',
            addAccount: 'Add new account',
            createAccount: 'Create account',
        },
        theme: { system: 'System', light: 'Light', dark: 'Dark' },
        lang: { de: 'Deutsch', en: 'English' },
        account: { edit: 'Edit', delete: 'Delete' },
        accountModal: {
            editTitle: 'Edit "{label}"',
            newTitle: 'New account',
            close: 'Close',
            saveChanges: 'Save changes',
            saveAndSync: 'Save & sync',
        },
        field: {
            label: 'Label',
            labelPlaceholder: 'Personal',
            email: 'Email address',
            emailPlaceholder: 'me@example.com',
            displayName: 'Display name ("From")',
            displayNamePlaceholder: 'e.g. Frank Lewandowski',
            syncInterval: 'Fetch automatically',
            host: 'Host',
            port: 'Port',
            security: 'Security',
            username: 'Username',
            usernamePlaceholderEmail: 'same as email if empty',
            usernamePlaceholderImap: 'same as IMAP if empty',
            password: 'Password',
            passwordKeepPlaceholder: 'Leave empty to keep the saved password',
        },
        syncInterval: {
            manual: 'Manual only',
            m5: 'Every 5 minutes',
            m15: 'Every 15 minutes',
            m30: 'Every 30 minutes',
            hourly: 'Hourly',
        },
        section: { imap: 'IMAP (Receiving)', smtp: 'SMTP (Sending)' },
        button: { testImap: 'Test IMAP connection', testSmtp: 'Test SMTP connection' },
        alert: {
            accountLoadFailedPrefix: "Couldn't load account: ",
            accountDeleteFailedPrefix: 'Delete failed: ',
            mailDeleteFailedPrefix: 'Delete failed: ',
            moveFailedPrefix: 'Move failed: ',
            attachmentSaveFailedPrefix: "Couldn't save attachment: ",
        },
        editor: {
            bold: 'Bold',
            italic: 'Italic',
            underline: 'Underline',
            bulletList: 'Bullet list',
            numberedList: 'Numbered list',
            link: 'Insert link',
            linkPrompt: 'Link target URL:',
            clearFormat: 'Clear formatting',
        },
    },
}

let currentLang = localStorage.getItem('fmail:lang') || 'de'

export function getLang() {
    return currentLang
}

export function setLang(lang) {
    currentLang = translations[lang] ? lang : 'de'
    localStorage.setItem('fmail:lang', currentLang)
}

function getPath(obj, path) {
    return path.split('.').reduce((o, k) => (o && o[k] !== undefined ? o[k] : undefined), obj)
}

// t(key, vars): key ist ein Punktpfad wie "compose.subject". Fällt auf
// Deutsch zurück, falls in der aktuellen Sprache ein Key fehlt, und auf
// den Key selbst, falls er nirgends existiert (sichtbar statt stillem
// Leerstring — leichter zu entdecken, dass eine Übersetzung fehlt).
export function t(key, vars) {
    let str = getPath(translations[currentLang], key)
    if (str === undefined) str = getPath(translations.de, key)
    if (str === undefined) return key
    if (vars) {
        for (const [k, v] of Object.entries(vars)) {
            str = str.replaceAll(`{${k}}`, v)
        }
    }
    return str
}

// applyStaticTranslations übersetzt HTML-Elemente, die main.js nicht bei
// jedem render() neu aufbaut (Topbar: Tabs, Suchfeld) — über
// data-i18n[-placeholder|-aria-label]-Attribute in index.html.
export function applyStaticTranslations() {
    document.documentElement.lang = currentLang
    document.querySelectorAll('[data-i18n]').forEach(el => {
        el.textContent = t(el.dataset.i18n)
    })
    document.querySelectorAll('[data-i18n-placeholder]').forEach(el => {
        el.placeholder = t(el.dataset.i18nPlaceholder)
    })
    document.querySelectorAll('[data-i18n-aria-label]').forEach(el => {
        el.setAttribute('aria-label', t(el.dataset.i18nAriaLabel))
    })
}
