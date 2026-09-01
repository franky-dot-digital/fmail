#!/bin/bash
set -e

if [ -z "$DISPLAY" ]; then
    echo "Kein DISPLAY vom Host übergeben -- starte eigenen X-Server + noVNC unter :99"
    export DISPLAY=:99
    Xvfb :99 -screen 0 1280x800x24 -ac -nolisten tcp &
    sleep 1
    fluxbox &
    sleep 1
    x11vnc -display :99 -forever -shared -nopw -quiet -bg
    websockify --web=/usr/share/novnc/ 6080 localhost:5900 &
    echo "Bereit -- im Browser öffnen: http://localhost:6080/vnc.html"
else
    echo "Nutze vom Host übergebenes DISPLAY=$DISPLAY (X11-Forwarding)"
fi

# Best-effort: ein auto-entsperrter Secret-Service-Schlüsselbund für den Test.
# Kann je nach Docker-Setup fehlschlagen -- betrifft dann nur den Schritt
# "Konto speichern" im Konto-Tab, nicht den Rest der App. Siehe README.
eval "$(dbus-launch --sh-syntax 2>/dev/null)" || true
mkdir -p "$HOME/.local/share/keyrings"
eval "$(printf '\n' | gnome-keyring-daemon --unlock --replace --daemonize --components=secrets 2>/dev/null)" || true

BIN=$(find /build/fmail/bin -maxdepth 1 -type f -executable | head -1)
if [ -z "$BIN" ]; then
    echo "Kein Binary in /build/fmail/bin gefunden -- Build-Logs prüfen." >&2
    exit 1
fi

exec "$BIN"
