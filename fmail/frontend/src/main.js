// Hinweis: Dieser Importpfad wird erst nach dem ersten `wails3 dev`-Lauf
// erzeugt (Wails generiert die Bindings aus mailservice.go/accountservice.go).
// Falls der Pfad nicht passt, in frontend/bindings/ nachsehen und anpassen.
import { MailService, AccountService } from '../bindings/fmail'
import { brandIcons } from './brandIcons.js'
import { t, getLang, setLang, applyStaticTranslations } from './i18n.js'

const state = {
    view: 'post',
    theme: localStorage.getItem('fmail:theme') || 'system',
    accounts: [],
    post: [],
    ablage: [],
    gesendet: [],
    pförtner: [],
    detailId: null,
    detail: null,
    htmlImagesAllowed: false, // pro geöffneter Mail, siehe openMail() / "Bilder laden"
    compose: null,
    accountModal: null, // null | {mode:'create'} | {mode:'edit', id, data}
    confirmModal: null, // null | {title, message, confirmLabel, cancelLabel, danger, onConfirm}
    showRead: false, // Zero-Inbox-Filter für Post: standardmäßig nur Ungelesene
    search: '',
}

// Der ausschließlich per ?demo=1 aktivierte Modus versorgt Screenshots und
// Projekt-Demos mit erfundenen Daten. Er ruft keine Wails-Bindings auf und
// kann daher niemals persönliche Konten oder Nachrichten laden.
const demoMode = new URLSearchParams(location.search).get('demo') === '1'
const demoDetails = new Map()

function loadDemoData() {
    state.accounts = [
        { id: 1, label: 'Privat', email: 'alex@example.com', displayName: 'Alex Beispiel', syncIntervalMinutes: 15 },
        { id: 2, label: 'Studio', email: 'hello@atelier.example', displayName: 'Atelier Nord', syncIntervalMinutes: 30 },
    ]
    state.post = [
        { id: 101, accountId: 1, accountLabel: 'Privat', sender: 'Mira Sommer', initials: 'MS', avatarColor: 'moss', brandIcon: '', subject: 'Die Pläne für Samstag', preview: 'Ich habe die Route noch einmal angesehen – der kleine Weg am See klingt perfekt.', date: '1. Sep', unread: true, hasAttachment: false },
        { id: 102, accountId: 2, accountLabel: 'Studio', sender: 'GitHub', initials: 'GH', avatarColor: 'slate', brandIcon: 'github', subject: 'Review requested: calm inbox', preview: 'Sam has requested your review on pull request #42.', date: '1. Sep', unread: true, hasAttachment: true },
        { id: 103, accountId: 1, accountLabel: 'Privat', sender: 'Jonas Feld', initials: 'JF', avatarColor: 'amber', brandIcon: '', subject: 'Rezept für das Gartenfest', preview: 'Hier ist wie versprochen das Rezept. Die Mengen reichen für ungefähr acht Personen.', date: '31. Aug', unread: false, hasAttachment: true },
    ]
    state.ablage = [
        { id: 201, accountId: 1, accountLabel: 'Privat', sender: 'Bücherstube West', initials: 'BW', avatarColor: 'rust', brandIcon: '', subject: 'Deine Bestellung ist unterwegs', preview: 'Das Paket wurde heute an den Versand übergeben.', date: '30. Aug', unread: false, hasAttachment: false },
        { id: 202, accountId: 2, accountLabel: 'Studio', sender: 'Cloudflare', initials: 'CF', avatarColor: 'amber', brandIcon: 'cloudflare', subject: 'Monthly security summary', preview: 'Your August security summary is ready.', date: '29. Aug', unread: false, hasAttachment: false },
    ]
    state.gesendet = [
        { id: 301, accountId: 2, accountLabel: 'Studio', sender: 'team@beispiel.de', initials: 'TB', avatarColor: 'steel', brandIcon: '', subject: 'Entwurf für die neue Startseite', preview: 'Hallo zusammen, anbei findet ihr den überarbeiteten Entwurf.', date: '1. Sep', unread: false, hasAttachment: true },
        { id: 302, accountId: 1, accountLabel: 'Privat', sender: 'mira@example.net', initials: 'MI', avatarColor: 'moss', brandIcon: '', subject: 'Re: Die Pläne für Samstag', preview: 'Sehr gern – dann treffen wir uns um zehn am alten Bahnhof.', date: '31. Aug', unread: false, hasAttachment: false },
    ]
    state.pförtner = [
        { id: '1:newsletter@kaffeekollektiv.example', accountId: 1, accountLabel: 'Privat', name: 'Kaffee Kollektiv', email: 'newsletter@kaffeekollektiv.example', initials: 'KK', avatarColor: 'rust', brandIcon: '', subject: 'Neue Ernte aus Kolumbien', preview: 'Fruchtig, klar und diese Woche frisch geröstet.' },
        { id: '2:lea@neuesprojekt.example', accountId: 2, accountLabel: 'Studio', name: 'Lea Hartmann', email: 'lea@neuesprojekt.example', initials: 'LH', avatarColor: 'steel', brandIcon: '', subject: 'Anfrage für ein kleines Webprojekt', preview: 'Hallo, mir gefällt eure ruhige Gestaltung sehr. Habt ihr im Oktober Kapazität?' },
    ]
    demoDetails.set(101, { ...state.post[0], senderEmail: 'mira@example.net', folder: 'post', recipients: [], date: '1. Sep 2026, 09:24', body: 'Hallo Alex,\n\nich habe die Route noch einmal angesehen – der kleine Weg am See klingt perfekt. Wenn das Wetter hält, können wir dort eine Pause machen.\n\nBis Samstag!\nMira', bodyHtml: '<p>Hallo Alex,</p><p>ich habe die Route noch einmal angesehen – der kleine Weg am See klingt perfekt. Wenn das Wetter hält, können wir dort eine Pause machen.</p><p>Bis Samstag!<br><strong>Mira</strong></p>', attachments: [] })
    demoDetails.set(102, { ...state.post[1], senderEmail: 'notifications@github.com', folder: 'post', recipients: [], date: '1. Sep 2026, 08:12', body: 'Sam has requested your review on pull request #42.', bodyHtml: '', attachments: [{ id: 1, filename: 'review-notes.pdf', contentType: 'application/pdf', size: 184320 }] })
}

const content = document.getElementById('content')
const tabs = document.getElementById('tabs')
const searchInput = document.getElementById('search-input')

tabs.addEventListener('click', (e) => {
    const btn = e.target.closest('.tab')
    if (!btn) return
    setView(btn.dataset.view)
})

searchInput.addEventListener('input', () => {
    state.search = searchInput.value.trim().toLowerCase()
    render()
})

document.addEventListener('keydown', (e) => {
    if (e.key !== 'Escape') return
    if (state.confirmModal) closeConfirmModal()
    else if (state.accountModal) closeAccountModal()
})

function setView(view) {
    state.view = view
    state.detailId = null
    state.detail = null
    state.compose = null
    closeAccountModal()
    closeConfirmModal()
    document.querySelectorAll('.tab').forEach(tabEl => {
        tabEl.classList.toggle('is-active', tabEl.dataset.view === view)
    })
    render()
}

// hasMultipleAccounts steuert, ob Post/Pförtner ein Konto-Badge pro Zeile
// zeigen — bei genau einem Konto wäre das nur redundantes Rauschen.
function hasMultipleAccounts() {
    return state.accounts.length > 1
}

// ---------- Erscheinungsbild (Hell/Dunkel/System) ----------

function applyTheme() {
    if (state.theme === 'light' || state.theme === 'dark') {
        document.documentElement.setAttribute('data-theme', state.theme)
    } else {
        document.documentElement.removeAttribute('data-theme')
    }
}

function setTheme(theme) {
    state.theme = theme
    localStorage.setItem('fmail:theme', theme)
    applyTheme()
    if (state.view === 'einstellungen') renderEinstellungen()
}

// ---------- Sprache (Deutsch/Englisch) ----------

function setAppLang(lang) {
    setLang(lang)
    applyStaticTranslations()
    render()
}

// ---------- Automatischer Sync ----------

// lastSyncedAt lebt nur im Speicher (nicht persistiert) — beim Neustart
// beginnt die Zählung für jedes Konto neu, siehe init().
const lastSyncedAt = {}
let autoSyncStarted = false

// startAutoSync ist idempotent: sie wird sowohl beim Start (falls schon
// Konten existieren) als auch nach dem Anlegen des ersten Kontos über das
// Modal aufgerufen — ohne die Guard-Variable liefe sonst pro Aufruf ein
// weiterer Timer.
function startAutoSync() {
    if (autoSyncStarted) return
    autoSyncStarted = true
    setInterval(checkAutoSync, 60 * 1000)
}

async function checkAutoSync() {
    const now = Date.now()
    const due = state.accounts.filter(a => {
        if (!a.syncIntervalMinutes) return false
        return now - (lastSyncedAt[a.id] || 0) >= a.syncIntervalMinutes * 60 * 1000
    })
    if (due.length === 0) return

    for (const a of due) {
        lastSyncedAt[a.id] = now
        try {
            await AccountService.SyncNow(a.id)
        } catch (err) {
            console.error(`Automatischer Sync für "${a.label}" fehlgeschlagen:`, err)
        }
    }
    await loadAll()
}

async function init() {
	if (demoMode) {
		const params = new URLSearchParams(location.search)
		state.theme = params.get('theme') || 'light'
		setLang(params.get('lang') || 'de')
		loadDemoData()
		state.view = params.get('view') || 'post'
		document.querySelectorAll('.tab').forEach(tabEl => tabEl.classList.toggle('is-active', tabEl.dataset.view === state.view))
		const detailID = Number(params.get('detail'))
		if (detailID && demoDetails.has(detailID)) {
			state.detailId = detailID
			state.detail = demoDetails.get(detailID)
		}
		applyTheme()
		applyStaticTranslations()
		updateCounts()
		render()
		return
	}
	applyTheme()
    applyStaticTranslations()
    try {
        state.accounts = await AccountService.ListAccounts() || []
    } catch (err) {
        console.error('Konten konnten nicht geladen werden:', err)
    }

    if (state.accounts.length === 0) {
        setView('einstellungen')
    } else {
        const now = Date.now()
        for (const a of state.accounts) lastSyncedAt[a.id] = now
        await loadAll()
        startAutoSync()
    }
}

// loadAll lädt Post/Ablage/Pförtner als gemeinsames Postfach über alle
// Konten hinweg — es gibt kein "aktives" Einzelkonto mehr, siehe
// mailservice.go GetPost/GetAblage/GetPfoertner.
async function loadAll() {
    try {
        const [post, ablage, gesendet, pförtner] = await Promise.all([
            MailService.GetPost(),
            MailService.GetAblage(),
            MailService.GetGesendet(),
            MailService.GetPfoertner(),
        ])
        state.post = post || []
        state.ablage = ablage || []
        state.gesendet = gesendet || []
        state.pförtner = pförtner || []
    } catch (err) {
        console.error('Mails konnten nicht geladen werden:', err)
    }
    updateCounts()
    render()
}

function updateCounts() {
    const unread = state.post.filter(m => m.unread).length
    document.getElementById('count-post').textContent = unread > 0 ? unread : ''
    document.getElementById('count-pförtner').textContent =
        state.pförtner.length > 0 ? state.pförtner.length : ''
}

function applySearch(mails) {
    if (!state.search) return mails
    return mails.filter(m =>
        (m.subject || '').toLowerCase().includes(state.search) ||
        (m.sender || '').toLowerCase().includes(state.search) ||
        (m.preview || '').toLowerCase().includes(state.search)
    )
}

function render() {
    if (state.view === 'einstellungen') return renderEinstellungen()
    if (state.accounts.length === 0) return setView('einstellungen')
    if (state.compose) return renderCompose()
    if (state.detailId) return renderDetail()

    if (state.view === 'post') {
        // Zero-Inbox: standardmäßig nur Ungelesene, Gelesene erst nach
        // Klick auf "Gelesene anzeigen".
        const unread = state.post.filter(m => m.unread)
        const readCount = state.post.length - unread.length
        const base = state.showRead ? state.post : unread
        return renderMailView(applySearch(base), t('post.title'), t('post.sub'), {
            showCompose: true, showReadToggle: true, readCount,
        })
    }
    if (state.view === 'ablage') return renderMailView(
        applySearch(state.ablage), t('ablage.title'), t('ablage.sub')
    )
    if (state.view === 'gesendet') return renderMailView(
        applySearch(state.gesendet), t('gesendet.title'), t('gesendet.sub')
    )
    if (state.view === 'pförtner') return renderPförtner()
}

function renderMailView(mails, title, sub, opts = {}) {
    content.innerHTML = `
        <div class="view">
            <div class="view-header">
                <div class="view-title">${title}</div>
                <div class="view-sub">${sub}</div>
                <div class="view-header-actions">
                    ${opts.showCompose ? `<button class="header-action-btn" id="compose-new-btn">${iconPencil()} ${t('mail.new')}</button>` : ''}
                    ${opts.showReadToggle ? `
                        <button class="header-action-btn" id="toggle-read-btn">
                            ${state.showRead ? t('mail.onlyUnread') : `${t('mail.showRead')}${opts.readCount ? ` (${opts.readCount})` : ''}`}
                        </button>
                    ` : ''}
                    <button class="sync-btn" id="sync-btn">${iconSync()} <span id="sync-btn-label">${t('sync.now')}</span></button>
                </div>
            </div>
            ${mails.length === 0 ? emptyState(t('empty.nothingHereTitle'), t('empty.nothingHereSub')) : `
                <div class="mail-list">${mails.map(mailRow).join('')}</div>
            `}
        </div>
    `
    const syncBtn = document.getElementById('sync-btn')
    if (syncBtn) syncBtn.addEventListener('click', syncNow)

    const composeBtn = document.getElementById('compose-new-btn')
    if (composeBtn) composeBtn.addEventListener('click', openComposeNew)

    const toggleBtn = document.getElementById('toggle-read-btn')
    if (toggleBtn) toggleBtn.addEventListener('click', () => { state.showRead = !state.showRead; render() })

    const list = content.querySelector('.mail-list')
    if (list) list.addEventListener('click', (e) => {
        const row = e.target.closest('.mail-row')
        if (row) openMail(Number(row.dataset.id))
    })
}

// syncNow synchronisiert alle Konten (SyncAll ist Best-Effort — ein
// einzelnes fehlschlagendes Konto bricht die anderen nicht ab), lädt
// aber in jedem Fall neu, damit bereits importierte Mails sichtbar
// werden, auch wenn ein anderes Konto einen Fehler gemeldet hat.
async function syncNow() {
    const btn = document.getElementById('sync-btn')
    const label = document.getElementById('sync-btn-label')
    if (btn) btn.disabled = true
    if (label) label.textContent = t('sync.inProgress')
    try {
        await AccountService.SyncAll()
    } catch (err) {
        console.error('Sync teilweise fehlgeschlagen:', err)
        alert(t('sync.resultPrefix') + err)
    }
    await loadAll()
    if (btn) btn.disabled = false
    if (label) label.textContent = t('sync.now')
}

function mailRow(m) {
    return `
        <div class="mail-row ${m.unread ? 'is-unread' : ''}" data-id="${m.id}">
            ${avatarMarkup(m)}
            <div class="mail-body">
                <div class="mail-top-line">
                    ${m.unread ? '<span class="unread-dot"></span>' : ''}
                    <span class="mail-subject">${escapeHtml(m.subject)}</span>
                    ${hasMultipleAccounts() ? `<span class="account-badge">${escapeHtml(m.accountLabel)}</span>` : ''}
                </div>
                <div class="mail-preview">
                    <span class="sender">${escapeHtml(m.sender)}</span> — ${escapeHtml(m.preview)}
                </div>
            </div>
            <div class="mail-meta">
                <span class="mail-date">${m.date}</span>
                ${m.hasAttachment ? '<span class="clip">&#128206;</span>' : ''}
            </div>
        </div>
    `
}

function applySearchPending(list) {
    if (!state.search) return list
    return list.filter(p =>
        (p.subject || '').toLowerCase().includes(state.search) ||
        (p.name || '').toLowerCase().includes(state.search) ||
        (p.email || '').toLowerCase().includes(state.search) ||
        (p.preview || '').toLowerCase().includes(state.search)
    )
}

function renderPförtner() {
    const pending = applySearchPending(state.pförtner)
    content.innerHTML = `
        <div class="view">
            <div class="view-header">
                <div class="view-title">${t('pfoertner.title')}</div>
                <div class="view-sub">${t('pfoertner.sub')}</div>
            </div>
            ${pending.length === 0 ? emptyState(t('empty.noOneWaitingTitle'), t('empty.noOneWaitingSub')) : `
                <div id="gate-list">${pending.map(gateCard).join('')}</div>
            `}
        </div>
    `
    content.querySelectorAll('.stamp-btn').forEach(btn => {
        btn.addEventListener('click', () => onDecide(
            btn.dataset.cardId, Number(btn.dataset.accountId), btn.dataset.email, btn.dataset.approve === '1'
        ))
    })
}

function gateCard(p) {
    return `
        <div class="gate-card" id="gate-${cssEscape(p.id)}">
            <div class="gate-stamps">
                <button class="stamp-btn approve" data-card-id="${escapeAttr(p.id)}" data-account-id="${p.accountId}" data-email="${escapeAttr(p.email)}" data-approve="1" title="${t('pfoertner.approveTitle')}">
                    ${iconCheck()} ${t('pfoertner.approve')}
                </button>
                <button class="stamp-btn block" data-card-id="${escapeAttr(p.id)}" data-account-id="${p.accountId}" data-email="${escapeAttr(p.email)}" data-approve="0" title="${t('pfoertner.blockTitle')}">
                    ${iconCross()} ${t('pfoertner.block')}
                </button>
            </div>
            <div class="gate-body">
                <div class="gate-sender-line">
                    <span class="gate-name">${escapeHtml(p.name)}</span>
                    <span class="gate-email">${escapeHtml(p.email)}</span>
                    ${hasMultipleAccounts() ? `<span class="account-badge">${escapeHtml(p.accountLabel)}</span>` : ''}
                </div>
                <div class="gate-subject">${escapeHtml(p.subject)}</div>
                <div class="gate-preview">${escapeHtml(p.preview)}</div>
            </div>
        </div>
    `
}

async function onDecide(cardId, accountId, email, approve) {
    const card = document.getElementById(`gate-${cssEscape(cardId)}`)
    if (!card) return
    card.classList.add('is-deciding')

    try {
        await MailService.Entscheiden(accountId, email, approve)
    } catch (err) {
        card.classList.remove('is-deciding')
        console.error('Entscheidung fehlgeschlagen:', err)
        return
    }

    toast(approve ? t('toast.approved') : t('toast.blocked'))
    card.classList.add('leaving')
    setTimeout(async () => {
        state.pförtner = state.pförtner.filter(p => p.id !== cardId)
        updateCounts()
        if (approve) await loadAll()
        else render()
    }, 220)
}

let toastTimeout
function toast(msg) {
    const el = document.getElementById('gate-toast')
    if (!el) return
    el.textContent = msg
    el.classList.add('show')
    clearTimeout(toastTimeout)
    toastTimeout = setTimeout(() => el.classList.remove('show'), 2200)
}

// ---------- Mail-Detailansicht ----------

async function openMail(id) {
    state.detailId = id
    state.htmlImagesAllowed = false
    if (demoMode && demoDetails.has(id)) {
        state.detail = demoDetails.get(id)
    } else try {
        state.detail = await MailService.GetMail(id)
    } catch (err) {
        console.error('Mail konnte nicht geladen werden:', err)
        state.detailId = null
        return
    }
    for (const list of [state.post, state.ablage, state.gesendet]) {
        const m = list.find(x => x.id === id)
        if (m) m.unread = false
    }
    updateCounts()
    renderDetail()
}

function closeDetail() {
    state.detailId = null
    state.detail = null
    render()
}

function renderDetail() {
    const d = state.detail
    const isSent = d.folder === 'gesendet'
    const moveLabel = d.folder === 'ablage' ? t('detail.moveBackToPost') : t('detail.moveToAblage')
    content.innerHTML = `
        <div class="view view-detail">
            <button class="back-btn" id="back-btn">${iconBack()} ${t('detail.back')}</button>
            <div class="detail-header">
                ${avatarMarkup(d)}
                <div class="detail-header-text">
                    <div class="detail-subject">${escapeHtml(d.subject)}</div>
                    <div class="detail-sender-line">
                        ${isSent ? `
                            <span class="detail-sender">${t('detail.toLabel')} ${escapeHtml((d.recipients || []).join(', '))}</span>
                        ` : `
                            <span class="detail-sender">${escapeHtml(d.sender)}</span>
                            <span class="detail-email">${escapeHtml(d.senderEmail)}</span>
                        `}
                        ${hasMultipleAccounts() ? `<span class="account-badge">${escapeHtml(d.accountLabel)}</span>` : ''}
                    </div>
                </div>
                <div class="detail-date">${escapeHtml(d.date)}</div>
            </div>
            ${renderMailBody(d)}
            ${d.attachments && d.attachments.length > 0 ? `
                <div class="attachment-list">${d.attachments.map(attachmentRow).join('')}</div>
            ` : ''}
            <div class="detail-actions">
                ${isSent ? '' : `<button class="secondary-btn" id="reply-btn">${iconReply()} ${t('detail.reply')}</button>`}
                <button class="secondary-btn" id="forward-btn">${iconForward()} ${t('detail.forward')}</button>
                ${isSent ? '' : `<button class="secondary-btn" id="move-btn">${iconMove()} ${moveLabel}</button>`}
                <button class="secondary-btn danger-btn" id="delete-btn">${iconTrash()} ${t('detail.delete')}</button>
            </div>
        </div>
    `
    document.getElementById('back-btn').addEventListener('click', closeDetail)
    document.getElementById('forward-btn').addEventListener('click', () => openCompose('forward'))
    document.getElementById('delete-btn').addEventListener('click', onDelete)
    const replyBtn = document.getElementById('reply-btn')
    if (replyBtn) replyBtn.addEventListener('click', () => openCompose('reply'))
    const moveBtn = document.getElementById('move-btn')
    if (moveBtn) moveBtn.addEventListener('click', onMove)

    if (d.bodyHtml) {
        const iframe = document.getElementById('mail-html-frame')
        iframe.srcdoc = buildHTMLFrameDoc(d.bodyHtml, state.htmlImagesAllowed, isDarkTheme())
        wireHTMLFrame(iframe)
        const loadImagesBtn = document.getElementById('load-images-btn')
        if (loadImagesBtn) loadImagesBtn.addEventListener('click', () => {
            state.htmlImagesAllowed = true
            renderDetail()
        })
    }

    content.querySelectorAll('.attachment-save-btn').forEach(btn => {
        btn.addEventListener('click', () => onSaveAttachment(Number(btn.dataset.id)))
    })
}

// renderMailBody zeigt HTML-Mails in einem sandboxed <iframe> (kein
// allow-scripts -> keine Skriptausführung, egal was die Sanitize-Policy
// im Backend übersehen haben sollte) statt als Klartext, sofern eine
// HTML-Variante vorliegt. allow-same-origin ist nötig, damit das
// Elternfenster die Höhe auslesen und Linkklicks abfangen kann (siehe
// wireHTMLFrame) — riskant wäre das nur in Kombination mit allow-scripts,
// das hier nie gesetzt wird.
function renderMailBody(d) {
    if (!d.bodyHtml) {
        return `<div class="detail-body">${escapeHtml(d.body)}</div>`
    }
    return `
        <div class="detail-html-toolbar">
            ${!state.htmlImagesAllowed ? `<button type="button" class="header-action-btn" id="load-images-btn">${iconImage()} ${t('detail.loadImages')}</button>` : ''}
        </div>
        <div class="detail-html-surface">
            <iframe class="detail-html-frame" id="mail-html-frame" sandbox="allow-same-origin"></iframe>
        </div>
    `
}

// buildHTMLFrameDoc kapselt das bereits sanitierte HTML in ein
// eigenständiges Dokument mit eigener CSP — die eigentliche Bremse für
// Tracking-Pixel & Co: img-src erlaubt standardmäßig nur data:/blob:
// (also eingebettete Inline-Bilder), keine http(s)-URLs, bis "Bilder
// laden" das für diese eine Mail explizit aufhebt.
function buildHTMLFrameDoc(html, allowImages, dark) {
    const csp = allowImages
        ? "default-src 'none'; img-src * data: blob:; style-src 'unsafe-inline'; font-src data: https: http:;"
        : "default-src 'none'; img-src data: blob:; style-src 'unsafe-inline'; font-src data:;"
    const ink = dark ? '#F1EFEA' : '#1C1E21'
    const scheme = dark ? 'dark' : 'light'
    return `<!doctype html><html><head><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="${csp}"><meta name="color-scheme" content="${scheme}"><style>html,body{margin:0!important;min-height:100%;background:transparent!important}body{color:${ink};color-scheme:${scheme};overflow-wrap:anywhere}img{max-width:100%;height:auto}</style></head><body>${html}</body></html>`
}

function isDarkTheme() {
    return state.theme === 'dark' ||
        (state.theme === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches)
}

// wireHTMLFrame passt die iframe-Höhe an den Inhalt an und leitet Klicks
// auf Links in den Systembrowser um (MailService.OpenExternalLink) —
// ohne allow-popups/allow-top-navigation würden sie sonst einfach ins
// Leere laufen.
function wireHTMLFrame(iframe) {
    iframe.addEventListener('load', () => {
        let doc
        try {
            doc = iframe.contentDocument
        } catch {
            return
        }
        if (!doc) return
        iframe.style.height = doc.documentElement.scrollHeight + 'px'
        doc.addEventListener('click', (e) => {
            const a = e.target.closest('a')
            if (!a || !a.href) return
            e.preventDefault()
            MailService.OpenExternalLink(a.href).catch(err => console.error('Link konnte nicht geöffnet werden:', err))
        })
    })
}

function attachmentRow(a) {
    return `
        <div class="attachment-row">
            ${iconClip()}
            <div class="attachment-row-info">
                <div class="attachment-row-name">${escapeHtml(a.filename)}</div>
                <div class="attachment-row-size">${formatBytes(a.size)}</div>
            </div>
            <button type="button" class="header-action-btn attachment-save-btn" data-id="${a.id}">${t('detail.saveAttachment')}</button>
        </div>
    `
}

async function onSaveAttachment(id) {
    try {
        const path = await MailService.SaveAttachment(id)
        if (path) toast(t('toast.attachmentSaved', { path }))
    } catch (err) {
        console.error('Anhang konnte nicht gespeichert werden:', err)
        alert(t('alert.attachmentSaveFailedPrefix') + err)
    }
}

async function onMove() {
    const d = state.detail
    const btn = document.getElementById('move-btn')
    if (btn) btn.disabled = true
    try {
        if (d.folder === 'ablage') await MailService.MoveToPost(d.id)
        else await MailService.MoveToAblage(d.id)
    } catch (err) {
        console.error('Verschieben fehlgeschlagen:', err)
        alert(t('alert.moveFailedPrefix') + err)
        if (btn) btn.disabled = false
        return
    }
    toast(d.folder === 'ablage' ? t('toast.movedToPost') : t('toast.movedToAblage'))
    state.detailId = null
    state.detail = null
    await loadAll()
}

function onDelete() {
    const d = state.detail
    openConfirmModal({
        title: t('confirmDeleteMail.title'),
        message: t('confirmDeleteMail.message'),
        onConfirm: async () => {
            try {
                await MailService.DeleteMail(d.id)
            } catch (err) {
                console.error('Löschen fehlgeschlagen:', err)
                alert(t('alert.mailDeleteFailedPrefix') + err)
                return
            }
            toast(t('toast.mailDeleted'))
            state.detailId = null
            state.detail = null
            await loadAll()
        },
    })
}

// ---------- Antworten / Weiterleiten / Neu ----------

function openCompose(mode) {
    const d = state.detail
    const quoted = d.body.split('\n').map(line => '> ' + line).join('\n')
    const prefix = mode === 'reply' ? t('compose.replyPrefix') : t('compose.forwardPrefix')
    const quoteHeader = t('compose.quoteHeader', { date: d.date, sender: d.sender, email: d.senderEmail })
    state.compose = {
        mode,
        accountId: d.accountId,
        to: mode === 'reply' ? d.senderEmail : '',
        subject: withPrefix(d.subject, prefix),
        body: `\n\n${quoteHeader}\n${quoted}`,
    }
    render()
}

// openComposeNew startet eine neue Mail ohne Vorlage — im Gegensatz zu
// Antworten/Weiterleiten gibt es keine Ausgangsmail, aus der sich das
// Absenderkonto ableiten ließe, daher der Konto-Auswahl in renderCompose.
function openComposeNew() {
    if (state.accounts.length === 0) return
    state.compose = {
        mode: 'new',
        accountId: state.accounts[0].id,
        to: '',
        subject: '',
        body: '',
    }
    render()
}

function withPrefix(subject, prefix) {
    return subject.toLowerCase().startsWith(prefix.toLowerCase()) ? subject : prefix + subject
}

function composeTitle(mode) {
    return t(mode === 'reply' ? 'compose.reply' : mode === 'forward' ? 'compose.forward' : 'compose.new')
}

function renderCompose() {
    const c = state.compose
    if (!c.files) c.files = [] // native File-Objekte, siehe renderAttachmentChips
    if (c.htmlMode === undefined) c.htmlMode = false

    content.innerHTML = `
        <div class="view view-compose">
            <button class="back-btn" id="compose-back-btn">${iconBack()} ${t('detail.back')}</button>
            <div class="compose-card">
                <div class="compose-card-titlebar">
                    <div class="compose-card-title">${composeTitle(c.mode)}</div>
                    <div class="compose-mode-switch theme-switch" role="radiogroup" aria-label="${t('compose.format')}">
                        <button type="button" class="theme-btn ${!c.htmlMode ? 'is-active' : ''}" data-mode="plain">${t('compose.modePlain')}</button>
                        <button type="button" class="theme-btn ${c.htmlMode ? 'is-active' : ''}" data-mode="html">${t('compose.modeHtml')}</button>
                    </div>
                </div>
                <form id="compose-form">
                    ${hasMultipleAccounts() ? `
                        <div class="compose-row">
                            <label class="compose-row-label" for="compose-from">${t('compose.from')}</label>
                            <select class="compose-row-input" name="fromAccount" id="compose-from">
                                ${state.accounts.map(a => `
                                    <option value="${a.id}" ${a.id === c.accountId ? 'selected' : ''}>${escapeHtml(a.label)} (${escapeHtml(a.email)})</option>
                                `).join('')}
                            </select>
                        </div>
                    ` : ''}
                    <div class="compose-row">
                        <label class="compose-row-label" for="compose-to">${t('compose.to')}</label>
                        <input class="compose-row-input" id="compose-to" name="to" type="email" required value="${escapeAttr(c.to)}" placeholder="${t('compose.toPlaceholder')}" />
                    </div>
                    <div class="compose-row">
                        <label class="compose-row-label" for="compose-subject">${t('compose.subject')}</label>
                        <input class="compose-row-input compose-subject-input" id="compose-subject" name="subject" required value="${escapeAttr(c.subject)}" placeholder="${t('compose.subjectPlaceholder')}" />
                    </div>
                    ${c.htmlMode ? `
                        <div class="compose-html-toolbar">
                            <button type="button" class="editor-btn editor-btn-bold" data-cmd="bold" title="${t('editor.bold')}">B</button>
                            <button type="button" class="editor-btn editor-btn-italic" data-cmd="italic" title="${t('editor.italic')}">I</button>
                            <button type="button" class="editor-btn editor-btn-underline" data-cmd="underline" title="${t('editor.underline')}">U</button>
                            <span class="editor-toolbar-divider"></span>
                            <button type="button" class="editor-btn" data-cmd="insertUnorderedList" title="${t('editor.bulletList')}">&bull;</button>
                            <button type="button" class="editor-btn" data-cmd="insertOrderedList" title="${t('editor.numberedList')}">1.</button>
                            <span class="editor-toolbar-divider"></span>
                            <button type="button" class="editor-btn" data-cmd="link" title="${t('editor.link')}">${iconLink()}</button>
                            <button type="button" class="editor-btn" data-cmd="removeFormat" title="${t('editor.clearFormat')}">${iconCross()}</button>
                        </div>
                        <div class="compose-body-input compose-html-editor" id="compose-html-editor" contenteditable="true" data-placeholder="${escapeAttr(t('compose.bodyPlaceholder'))}">${plainTextToEditorHTML(c.body)}</div>
                    ` : `
                        <textarea class="compose-body-input" id="compose-body-textarea" name="body" rows="14" placeholder="${t('compose.bodyPlaceholder')}">${escapeHtml(c.body)}</textarea>
                    `}
                    <div class="compose-attachments" id="compose-attachments-list"></div>
                    <div class="compose-footer">
                        <button type="button" class="header-action-btn" id="compose-attach-btn">${iconClip()} ${t('compose.attach')}</button>
                        <input type="file" id="compose-file-input" multiple hidden />
                        <div class="compose-footer-right">
                            <span class="konto-status" id="compose-status"></span>
                            <button type="submit" class="primary-btn">${t('compose.send')}</button>
                        </div>
                    </div>
                </form>
            </div>
        </div>
    `
    document.getElementById('compose-back-btn').addEventListener('click', () => { state.compose = null; render() })
    document.getElementById('compose-form').addEventListener('submit', onSendCompose)
    document.getElementById('compose-attach-btn').addEventListener('click', () => {
        document.getElementById('compose-file-input').click()
    })
    document.getElementById('compose-file-input').addEventListener('change', (e) => {
        c.files.push(...Array.from(e.target.files))
        e.target.value = '' // sonst lässt sich dieselbe Datei kein zweites Mal auswählen
        renderAttachmentChips()
    })
    content.querySelectorAll('.compose-mode-switch [data-mode]').forEach(btn => {
        btn.addEventListener('click', () => toggleComposeMode(btn.dataset.mode))
    })
    content.querySelectorAll('.editor-btn').forEach(btn => {
        btn.addEventListener('click', () => execEditorCommand(btn.dataset.cmd))
    })
    renderAttachmentChips()
}

// toggleComposeMode wechselt zwischen Klartext-Textarea und
// HTML-Editor — übernimmt den bisherigen Inhalt bestmöglich (Klartext
// über innerText beim Wechsel HTML -> Text, direkt übernommen beim
// Wechsel Text -> HTML) und rendert das Formular komplett neu, da sich
// dabei das Eingabeelement selbst ändert.
function toggleComposeMode(mode) {
    const c = state.compose
    const wantHtml = mode === 'html'
    if (wantHtml === c.htmlMode) return
    if (c.htmlMode) {
        const editor = document.getElementById('compose-html-editor')
        if (editor) c.body = editor.innerText
    } else {
        const textarea = document.getElementById('compose-body-textarea')
        if (textarea) c.body = textarea.value
    }
    c.htmlMode = wantHtml
    renderCompose()
}

// plainTextToEditorHTML baut aus reinem Text (c.body ist das immer,
// siehe toggleComposeMode) sicheres Start-HTML für den contenteditable-
// Editor — escaped, mit \n -> <br>, kein roher String direkt in innerHTML.
function plainTextToEditorHTML(text) {
    return escapeHtml(text).replaceAll('\n', '<br>')
}

// execEditorCommand nutzt document.execCommand für den simplen
// HTML-Editor -- veraltet, aber ohne Bibliothek die einzige Möglichkeit
// für eine WYSIWYG-Formatierung aus dem Stand, und für Fett/Kursiv/
// Listen/Links breit genug unterstützt.
function execEditorCommand(cmd) {
    const editor = document.getElementById('compose-html-editor')
    if (!editor) return
    editor.focus()
    if (cmd === 'link') {
        const url = prompt(t('editor.linkPrompt'))
        if (url) document.execCommand('createLink', false, url)
        return
    }
    document.execCommand(cmd, false, null)
}

// renderAttachmentChips aktualisiert NUR die Anhang-Chip-Liste, nicht das
// ganze Formular — sonst würde jedes Hinzufügen/Entfernen eines Anhangs
// den Cursor aus Betreff/Nachricht reißen bzw. bereits Getipptes verwerfen.
function renderAttachmentChips() {
    const el = document.getElementById('compose-attachments-list')
    if (!el || !state.compose) return
    el.innerHTML = state.compose.files.map((f, i) => `
        <div class="attachment-chip">
            ${iconClip()}
            <span class="attachment-chip-name">${escapeHtml(f.name)}</span>
            <span class="attachment-chip-size">${formatBytes(f.size)}</span>
            <button type="button" class="attachment-chip-remove" data-index="${i}" aria-label="${t('compose.removeAttachment')}">${iconCross()}</button>
        </div>
    `).join('')
    el.querySelectorAll('.attachment-chip-remove').forEach(btn => {
        btn.addEventListener('click', () => {
            state.compose.files.splice(Number(btn.dataset.index), 1)
            renderAttachmentChips()
        })
    })
}

function formatBytes(bytes) {
    if (bytes < 1024) return `${bytes} B`
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

// readFileAsAttachment liest eine Datei als data:-URL und schneidet den
// Base64-Teil heraus — SendMail erwartet reines Base64 (siehe
// mailservice.go AttachmentInput), ohne das "data:<mime>;base64,"-Präfix.
function readFileAsAttachment(file) {
    return new Promise((resolve, reject) => {
        const reader = new FileReader()
        reader.onload = () => {
            const base64 = String(reader.result).split(',')[1] || ''
            resolve({ filename: file.name, contentType: file.type || 'application/octet-stream', data: base64 })
        }
        reader.onerror = () => reject(reader.error || new Error(t('status.attachmentReadFailedPrefix')))
        reader.readAsDataURL(file)
    })
}

const MAX_ATTACHMENTS_BYTES = 20 * 1024 * 1024 // muss zu mailservice.go maxAttachmentsSize passen

async function onSendCompose(e) {
    e.preventDefault()
    const statusEl = document.getElementById('compose-status')
    const data = new FormData(e.target)
    const to = data.get('to')?.trim() || ''
    const subject = data.get('subject')?.trim() || ''
    const accountId = data.get('fromAccount') ? Number(data.get('fromAccount')) : state.compose.accountId

    // Im HTML-Modus lebt der eigentliche Inhalt im contenteditable-Element,
    // nicht in der FormData (kein <textarea>) — innerText liefert den
    // Klartext-Fallback fürs multipart/alternative, innerHTML das HTML
    // selbst (wird serverseitig nochmal sanitiert, siehe smtpclient.go).
    const htmlEditor = document.getElementById('compose-html-editor')
    const body = htmlEditor ? htmlEditor.innerText : (data.get('body') || '')
    const bodyHTML = htmlEditor ? htmlEditor.innerHTML : ''

    const files = state.compose.files || []
    const totalBytes = files.reduce((sum, f) => sum + f.size, 0)
    if (totalBytes > MAX_ATTACHMENTS_BYTES) {
        statusEl.textContent = t('status.attachmentsTooLarge', { size: formatBytes(totalBytes) })
        return
    }

    let attachments = []
    if (files.length > 0) {
        statusEl.textContent = t('status.readingAttachments')
        try {
            attachments = await Promise.all(files.map(readFileAsAttachment))
        } catch (err) {
            statusEl.textContent = t('status.attachmentReadFailedPrefix') + err
            return
        }
    }

    statusEl.textContent = t('status.sending')
    try {
        await MailService.SendMail(accountId, [to], subject, body, bodyHTML, attachments)
    } catch (err) {
        statusEl.textContent = t('status.failedPrefix') + err
        return
    }
    toast(t('toast.mailSent'))
    state.compose = null
    state.detailId = null
    state.detail = null
    await loadAll()
}

// ---------- Einstellungen: Allgemein + Kontoverwaltung ----------

function renderEinstellungen() {
    content.innerHTML = `
        <div class="view">
            <div class="view-header">
                <div class="view-title">${t('settings.title')}</div>
            </div>

            <div class="section-label">${t('settings.general')}</div>
            <div class="settings-block">
                <div class="settings-row">
                    <div>
                        <div class="settings-row-title">${t('settings.appearance')}</div>
                        <div class="settings-row-sub">${t('settings.appearanceSub')}</div>
                    </div>
                    <div class="theme-switch" role="radiogroup" aria-label="${t('settings.appearance')}">
                        <button type="button" class="theme-btn ${state.theme === 'system' ? 'is-active' : ''}" data-theme="system">${t('theme.system')}</button>
                        <button type="button" class="theme-btn ${state.theme === 'light' ? 'is-active' : ''}" data-theme="light">${t('theme.light')}</button>
                        <button type="button" class="theme-btn ${state.theme === 'dark' ? 'is-active' : ''}" data-theme="dark">${t('theme.dark')}</button>
                    </div>
                </div>
                <div class="settings-row settings-row-divider">
                    <div>
                        <div class="settings-row-title">${t('settings.language')}</div>
                        <div class="settings-row-sub">${t('settings.languageSub')}</div>
                    </div>
                    <div class="theme-switch" role="radiogroup" aria-label="${t('settings.language')}">
                        <button type="button" class="theme-btn ${getLang() === 'de' ? 'is-active' : ''}" data-lang="de">${t('lang.de')}</button>
                        <button type="button" class="theme-btn ${getLang() === 'en' ? 'is-active' : ''}" data-lang="en">${t('lang.en')}</button>
                    </div>
                </div>
            </div>

            <div class="section-label-row">
                <div class="section-label">${t('settings.accounts')}</div>
                <button type="button" class="add-account-btn" id="add-account-btn" title="${t('settings.addAccount')}" aria-label="${t('settings.addAccount')}">${iconPlus()}</button>
            </div>
            ${state.accounts.length === 0 ? `
                <div class="empty">
                    <div class="empty-title">${t('empty.noAccountTitle')}</div>
                    <div class="empty-sub">${t('empty.noAccountSub')}</div>
                    <button type="button" class="primary-btn" id="add-first-account-btn">${t('settings.createAccount')}</button>
                </div>
            ` : `<div id="account-list">${state.accounts.map(accountRow).join('')}</div>`}
        </div>
    `

    content.querySelectorAll('.theme-btn[data-theme]').forEach(btn => {
        btn.addEventListener('click', () => setTheme(btn.dataset.theme))
    })
    content.querySelectorAll('.theme-btn[data-lang]').forEach(btn => {
        btn.addEventListener('click', () => setAppLang(btn.dataset.lang))
    })
    document.getElementById('add-account-btn').addEventListener('click', () => openAccountModal('create'))
    const addFirstBtn = document.getElementById('add-first-account-btn')
    if (addFirstBtn) addFirstBtn.addEventListener('click', () => openAccountModal('create'))

    content.querySelectorAll('.account-row').forEach(row => {
        const id = Number(row.dataset.id)
        row.querySelector('.account-edit-btn').addEventListener('click', () => openAccountModal('edit', id))
        row.querySelector('.account-delete-btn').addEventListener('click', () => onDeleteAccount(id))
    })
}

// ---------- Konto-Modal (Anlegen/Bearbeiten) ----------

function openAccountModal(mode, id) {
    if (mode === 'create') {
        state.accountModal = { mode: 'create' }
        renderModal()
        return
    }
    AccountService.GetAccountDetails(id).then(data => {
        state.accountModal = { mode: 'edit', id, data }
        renderModal()
    }).catch(err => {
        console.error('Konto konnte nicht geladen werden:', err)
        alert(t('alert.accountLoadFailedPrefix') + err)
    })
}

function closeAccountModal() {
    if (!state.accountModal) return
    state.accountModal = null
    renderModal()
}

function renderModal() {
    const root = document.getElementById('modal-root')
    const m = state.accountModal
    if (!m) { root.innerHTML = ''; return }

    const editing = m.mode === 'edit'
    const v = editing ? m.data : {}
    const title = editing ? t('accountModal.editTitle', { label: v.label }) : t('accountModal.newTitle')

    root.innerHTML = `
        <div class="modal-backdrop" id="modal-backdrop">
            <div class="modal-card">
                <div class="modal-header">
                    <div class="modal-title">${escapeHtml(title)}</div>
                    <button type="button" class="modal-close-btn" id="modal-close-btn" aria-label="${t('accountModal.close')}">${iconCross()}</button>
                </div>
                <form id="konto-form" class="konto-form">
                    ${accountFormFields(v, editing)}
                    <div class="konto-actions">
                        <span class="konto-status" id="konto-status"></span>
                        <button type="submit" class="primary-btn">${editing ? t('accountModal.saveChanges') : t('accountModal.saveAndSync')}</button>
                    </div>
                </form>
            </div>
        </div>
    `

    document.getElementById('modal-backdrop').addEventListener('click', (e) => {
        if (e.target.id === 'modal-backdrop') closeAccountModal()
    })
    document.getElementById('modal-close-btn').addEventListener('click', closeAccountModal)
    document.getElementById('test-imap-btn').addEventListener('click', () => testConnection('imap'))
    document.getElementById('test-smtp-btn').addEventListener('click', () => testConnection('smtp'))
    document.getElementById('konto-form').addEventListener('submit', editing ? onUpdateAccount : onSaveAccount)
}

// ---------- Bestätigungs-Modal (ersetzt window.confirm) ----------
// Generisch für alle "wirklich löschen?"-Rückfragen — passt optisch zum
// Rest der App statt eines nackten Browser-Dialogs.

function openConfirmModal({ title, message, confirmLabel, cancelLabel, danger = true, onConfirm }) {
    state.confirmModal = {
        title, message, onConfirm,
        confirmLabel: confirmLabel || t('confirmModal.delete'),
        cancelLabel: cancelLabel || t('confirmModal.cancel'),
        danger,
    }
    renderConfirmModal()
}

function closeConfirmModal() {
    if (!state.confirmModal) return
    state.confirmModal = null
    renderConfirmModal()
}

function renderConfirmModal() {
    const root = document.getElementById('confirm-modal-root')
    const c = state.confirmModal
    if (!c) { root.innerHTML = ''; return }

    root.innerHTML = `
        <div class="modal-backdrop" id="confirm-modal-backdrop">
            <div class="modal-card modal-card-narrow">
                <div class="modal-header">
                    <div class="modal-title">${escapeHtml(c.title)}</div>
                    <button type="button" class="modal-close-btn" id="confirm-modal-close-btn" aria-label="${t('accountModal.close')}">${iconCross()}</button>
                </div>
                <p class="confirm-modal-message">${escapeHtml(c.message)}</p>
                <div class="confirm-modal-actions">
                    <button type="button" class="secondary-btn" id="confirm-modal-cancel-btn">${escapeHtml(c.cancelLabel)}</button>
                    <button type="button" class="primary-btn ${c.danger ? 'primary-btn-danger' : ''}" id="confirm-modal-confirm-btn">${escapeHtml(c.confirmLabel)}</button>
                </div>
            </div>
        </div>
    `
    document.getElementById('confirm-modal-backdrop').addEventListener('click', (e) => {
        if (e.target.id === 'confirm-modal-backdrop') closeConfirmModal()
    })
    document.getElementById('confirm-modal-close-btn').addEventListener('click', closeConfirmModal)
    document.getElementById('confirm-modal-cancel-btn').addEventListener('click', closeConfirmModal)
    document.getElementById('confirm-modal-confirm-btn').addEventListener('click', async () => {
        const btn = document.getElementById('confirm-modal-confirm-btn')
        if (btn) btn.disabled = true
        await c.onConfirm()
        closeConfirmModal()
    })
}

// accountFormFields rendert die Formularfelder für Anlegen UND Bearbeiten
// eines Kontos — im Bearbeiten-Modus vorausgefüllt und mit optionalen
// Passwortfeldern (leer lassen = gespeichertes Passwort behalten).
function accountFormFields(v, editing) {
    const pwAttrs = editing ? `placeholder="${escapeAttr(t('field.passwordKeepPlaceholder'))}"` : `required`
    const interval = v.syncIntervalMinutes || 0
    return `
        <label>${t('field.label')}<input name="label" required placeholder="${t('field.labelPlaceholder')}" value="${escapeAttr(v.label || '')}" /></label>
        <label>${t('field.email')}<input name="email" type="email" required placeholder="${t('field.emailPlaceholder')}" value="${escapeAttr(v.email || '')}" /></label>
        <label>${t('field.displayName')}<input name="displayName" placeholder="${t('field.displayNamePlaceholder')}" value="${escapeAttr(v.displayName || '')}" /></label>
        <label>${t('field.syncInterval')}
            <select name="syncIntervalMinutes">
                <option value="0" ${interval === 0 ? 'selected' : ''}>${t('syncInterval.manual')}</option>
                <option value="5" ${interval === 5 ? 'selected' : ''}>${t('syncInterval.m5')}</option>
                <option value="15" ${interval === 15 ? 'selected' : ''}>${t('syncInterval.m15')}</option>
                <option value="30" ${interval === 30 ? 'selected' : ''}>${t('syncInterval.m30')}</option>
                <option value="60" ${interval === 60 ? 'selected' : ''}>${t('syncInterval.hourly')}</option>
            </select>
        </label>

        <div class="konto-section-label">${t('section.imap')}</div>
        <div class="konto-row">
            <label>${t('field.host')}<input name="imapHost" required placeholder="imap.example.com" value="${escapeAttr(v.imapHost || '')}" /></label>
            <label class="narrow">${t('field.port')}<input name="imapPort" type="number" required value="${v.imapPort || 993}" /></label>
            <label class="narrow">${t('field.security')}
                <select name="imapSecurity">
                    <option value="tls" ${v.imapSecurity !== 'starttls' ? 'selected' : ''}>TLS</option>
                    <option value="starttls" ${v.imapSecurity === 'starttls' ? 'selected' : ''}>STARTTLS</option>
                </select>
            </label>
        </div>
        <div class="konto-row">
            <label>${t('field.username')}<input name="imapUsername" placeholder="${t('field.usernamePlaceholderEmail')}" value="${escapeAttr(v.imapUsername || '')}" /></label>
            <label>${t('field.password')}<input name="imapPassword" type="password" ${pwAttrs} /></label>
        </div>
        <button type="button" class="secondary-btn" id="test-imap-btn">${t('button.testImap')}</button>

        <div class="konto-section-label">${t('section.smtp')}</div>
        <div class="konto-row">
            <label>${t('field.host')}<input name="smtpHost" required placeholder="smtp.example.com" value="${escapeAttr(v.smtpHost || '')}" /></label>
            <label class="narrow">${t('field.port')}<input name="smtpPort" type="number" required value="${v.smtpPort || 465}" /></label>
            <label class="narrow">${t('field.security')}
                <select name="smtpSecurity">
                    <option value="tls" ${v.smtpSecurity !== 'starttls' ? 'selected' : ''}>TLS</option>
                    <option value="starttls" ${v.smtpSecurity === 'starttls' ? 'selected' : ''}>STARTTLS</option>
                </select>
            </label>
        </div>
        <div class="konto-row">
            <label>${t('field.username')}<input name="smtpUsername" placeholder="${t('field.usernamePlaceholderImap')}" value="${escapeAttr(v.smtpUsername || '')}" /></label>
            <label>${t('field.password')}<input name="smtpPassword" type="password" placeholder="${editing ? escapeAttr(t('field.passwordKeepPlaceholder')) : t('field.usernamePlaceholderImap')}" /></label>
        </div>
        <button type="button" class="secondary-btn" id="test-smtp-btn">${t('button.testSmtp')}</button>
    `
}

function accountRow(a) {
    return `
        <div class="account-row" data-id="${a.id}">
            <div class="account-row-info">
                <div class="account-row-label">${escapeHtml(a.label)}</div>
                <div class="account-row-meta">${escapeHtml(a.email)}${a.displayName ? ` · ${escapeHtml(a.displayName)}` : ''}</div>
            </div>
            <div class="account-row-actions">
                <button type="button" class="secondary-btn account-edit-btn">${t('account.edit')}</button>
                <button type="button" class="secondary-btn danger-btn account-delete-btn">${t('account.delete')}</button>
            </div>
        </div>
    `
}

async function onUpdateAccount(e) {
    e.preventDefault()
    const statusEl = document.getElementById('konto-status')
    const input = readAccountInput()
    statusEl.textContent = t('status.savingChanges')
    let updated
    try {
        updated = await AccountService.UpdateAccount(state.accountModal.id, input)
    } catch (err) {
        statusEl.textContent = t('status.failedPrefix') + err
        return
    }
    const acc = state.accounts.find(a => a.id === updated.id)
    if (acc) {
        acc.label = updated.label
        acc.email = updated.email
        acc.displayName = updated.displayName
        acc.syncIntervalMinutes = updated.syncIntervalMinutes
    }
    closeAccountModal()
    toast(t('toast.accountUpdated'))
    await loadAll()
}

function onDeleteAccount(id) {
    const acc = state.accounts.find(a => a.id === id)
    const label = acc ? acc.label : '?'
    openConfirmModal({
        title: t('confirmDeleteAccount.title'),
        message: t('confirmDeleteAccount.message', { label }),
        onConfirm: async () => {
            try {
                await AccountService.DeleteAccount(id)
            } catch (err) {
                console.error('Konto konnte nicht gelöscht werden:', err)
                alert(t('alert.accountDeleteFailedPrefix') + err)
                return
            }
            state.accounts = state.accounts.filter(a => a.id !== id)
            delete lastSyncedAt[id]
            toast(t('toast.accountDeleted'))
            await loadAll()
        },
    })
}

function readAccountInput() {
    const form = document.getElementById('konto-form')
    const data = new FormData(form)
    return {
        label: data.get('label')?.trim() || '',
        email: data.get('email')?.trim() || '',
        displayName: data.get('displayName')?.trim() || '',
        syncIntervalMinutes: Number(data.get('syncIntervalMinutes')) || 0,
        imapHost: data.get('imapHost')?.trim() || '',
        imapPort: Number(data.get('imapPort')) || 0,
        imapSecurity: data.get('imapSecurity'),
        imapUsername: data.get('imapUsername')?.trim() || '',
        imapPassword: data.get('imapPassword') || '',
        smtpHost: data.get('smtpHost')?.trim() || '',
        smtpPort: Number(data.get('smtpPort')) || 0,
        smtpSecurity: data.get('smtpSecurity'),
        smtpUsername: data.get('smtpUsername')?.trim() || '',
        smtpPassword: data.get('smtpPassword') || '',
    }
}

async function testConnection(kind) {
    const statusEl = document.getElementById('konto-status')
    const input = readAccountInput()
    statusEl.textContent = t('status.testing')
    try {
        if (kind === 'imap') await AccountService.TestIMAP(input)
        else await AccountService.TestSMTP(input)
        statusEl.textContent = kind.toUpperCase() + ': ' + t('status.testSuccessSuffix')
    } catch (err) {
        statusEl.textContent = kind.toUpperCase() + ' ' + t('status.testFailedSuffix') + err
    }
}

async function onSaveAccount(e) {
    e.preventDefault()
    const statusEl = document.getElementById('konto-status')
    const input = readAccountInput()
    statusEl.textContent = t('status.savingAccount')
    let account
    try {
        account = await AccountService.CreateAccount(input)
    } catch (err) {
        statusEl.textContent = t('status.failedPrefix') + err
        return
    }
    state.accounts.push(account)
    lastSyncedAt[account.id] = Date.now()
    startAutoSync()
    statusEl.textContent = t('status.syncingAfterCreate')
    try {
        await AccountService.SyncNow(account.id)
    } catch (err) {
        console.error('Erster Sync fehlgeschlagen:', err)
    }
    closeAccountModal()
    await loadAll()
    setView('post')
}

// ---------- Hilfsfunktionen ----------

function emptyState(title, sub) {
    return `<div class="empty"><div class="empty-title">${title}</div><div class="empty-sub">${sub}</div></div>`
}

function iconCheck() {
    return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"></polyline></svg>`
}

function iconCross() {
    return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg>`
}

function iconBack() {
    return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="15 18 9 12 15 6"></polyline></svg>`
}

function iconClip() {
    return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21.44 11.05l-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48"></path></svg>`
}

function iconLink() {
    return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"></path><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"></path></svg>`
}

function iconImage() {
    return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="2"></rect><circle cx="8.5" cy="8.5" r="1.5"></circle><path d="M21 15l-5-5L5 21"></path></svg>`
}

function iconReply() {
    return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="9 17 4 12 9 7"></polyline><path d="M20 18v-2a4 4 0 0 0-4-4H4"></path></svg>`
}

function iconForward() {
    return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="15 17 20 12 15 7"></polyline><path d="M4 18v-2a4 4 0 0 1 4-4h12"></path></svg>`
}

function iconMove() {
    return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="21 8 21 21 3 21 3 8"></polyline><rect x="1" y="3" width="22" height="5"></rect><line x1="10" y1="12" x2="14" y2="12"></line></svg>`
}

function iconTrash() {
    return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"></polyline><path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"></path><path d="M10 11v6"></path><path d="M14 11v6"></path><path d="M9 6V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2"></path></svg>`
}

function iconPencil() {
    return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 20h9"></path><path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4Z"></path></svg>`
}

function iconSync() {
    return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="23 4 23 10 17 10"></polyline><polyline points="1 20 1 14 7 14"></polyline><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10"></path><path d="M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path></svg>`
}

function iconPlus() {
    return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="12" y1="5" x2="12" y2="19"></line><line x1="5" y1="12" x2="19" y2="12"></line></svg>`
}

// avatarMarkup zeigt für erkannte Absender-Domains (github.com,
// cloudflare.com, ...) das echte Markenlogo statt der generischen
// Initialen — item.brandIcon kommt vom Backend (mailservice.go,
// knownBrandDomains), die SVGs selbst liegen in brandIcons.js.
function avatarMarkup(item) {
    const brand = item.brandIcon && brandIcons[item.brandIcon]
    if (brand) {
        return `<div class="avatar avatar-brand"><svg viewBox="0 0 24 24" fill="${brand.color}">${brand.svg}</svg></div>`
    }
    return `<div class="avatar ${item.avatarColor}">${escapeHtml(item.initials)}</div>`
}

// escapeHtml: alle Mailinhalte (Absender, Betreff, Preview) laufen IMMER
// hierüber, nie über innerHTML mit rohem String. Das ist die Frontend-Hälfte
// der XSS-Absicherung — die Backend-Hälfte degradiert HTML-Mails serverseitig
// bereits zu Klartext (siehe internal/mail/sanitize.go).
function escapeHtml(str) {
    const div = document.createElement('div')
    div.textContent = str ?? ''
    return div.innerHTML
}

function escapeAttr(str) {
    return escapeHtml(str).replaceAll('"', '&quot;')
}

function cssEscape(str) {
    return (window.CSS && window.CSS.escape) ? window.CSS.escape(str) : String(str).replace(/[^a-zA-Z0-9_-]/g, '_')
}

init()
