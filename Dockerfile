FROM ubuntu:22.04
ENV DEBIAN_FRONTEND=noninteractive

# ---------- Go (Version und offizieller SHA-256 gepinnt) ----------
RUN apt-get update && apt-get install -y --no-install-recommends \
        curl ca-certificates \
    && curl -fsSL "https://go.dev/dl/go1.25.11.linux-amd64.tar.gz" -o /tmp/go.tar.gz \
    && echo "34f14304e856893f4ba30c2cacfe93906e9de7915c5f6aaaf3a81cdccd7ba30b  /tmp/go.tar.gz" | sha256sum -c - \
    && tar -C /usr/local -xzf /tmp/go.tar.gz \
    && rm /tmp/go.tar.gz
ENV PATH="/usr/local/go/bin:/root/go/bin:${PATH}"

# ---------- Node (für den Vite-Frontend-Build, den wails3 intern anstößt) ----------
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
    && apt-get install -y --no-install-recommends nodejs

# ---------- Build- und GUI-Abhängigkeiten ----------
# libgtk-3-dev/libwebkit2gtk-4.1-dev: WebKitGTK-Legacy-Pfad (Ubuntu 22.04 hat
# noch kein WebKitGTK 6.0), passend zum -tags gtk3 unten -- gleiche Wahl wie
# im CI-Workflow und wie sie auf openSUSE Leap nötig wäre.
# xvfb/x11vnc/novnc/websockify/fluxbox: virtuelles Display + Browser-Zugriff,
# nur relevant für die noVNC-Variante (siehe docker-compose.novnc.yml).
# gnome-keyring/dbus-x11: Best-Effort-Ersatz für den fehlenden echten
# OS-Schlüsselbund im Container (siehe README, Abschnitt "Docker").
RUN apt-get update && apt-get install -y --no-install-recommends \
        build-essential pkg-config git \
        libgtk-3-dev libwebkit2gtk-4.1-dev \
        xvfb x11vnc novnc websockify fluxbox dbus-x11 gnome-keyring \
        fonts-liberation \
    && rm -rf /var/lib/apt/lists/*

RUN go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.9

# ---------- Projekt bauen ----------
WORKDIR /build
COPY fmail ./fmail

WORKDIR /build/fmail
RUN go mod verify && cd frontend && npm ci && npm run build

RUN wails3 build -tags gtk3

# ---------- Laufzeit ----------
# Software-Rendering statt GPU -- im Container/unter Xvfb gibt es keine
# echte GPU, WebKitGTK würde sonst mit Compositing-Fehlern abstürzen.
ENV LIBGL_ALWAYS_SOFTWARE=1
ENV WEBKIT_DISABLE_COMPOSITING_MODE=1
ENV HOME=/root

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 6080
ENTRYPOINT ["/entrypoint.sh"]
