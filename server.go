package main

import (
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/big"
	"net"
	"net/http"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

//go:embed web/*
var assets embed.FS

type service struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type desktop interface {
	move(x, y int) error
	scroll(amount int) error
	click(button string) error
	holdLeft(down bool) error
	key(name string) error
	typeText(value string, erase int) error
	open(url string) error
}

type command struct {
	ID     int    `json:"id"`
	Kind   string `json:"kind"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Amount int    `json:"amount"`
	Button string `json:"button"`
	Down   bool   `json:"down"`
	Value  string `json:"value"`
	Erase  int    `json:"erase"`
}

type remote struct {
	mu       sync.Mutex
	target   desktop
	pin      string
	qrSecret string
	sessions map[string]time.Time
	clients  map[*websocket.Conn]string
	drag     *websocket.Conn
	attempts []time.Time
	services map[string]string
}

func randomToken() string {
	var key [24]byte
	rand.Read(key[:])
	return base64.RawURLEncoding.EncodeToString(key[:])
}

func newRemote(target desktop) *remote {
	data, err := assets.ReadFile("web/services.json")
	if err != nil {
		panic(err)
	}
	var list []service
	if err := json.Unmarshal(data, &list); err != nil {
		panic(err)
	}
	h := &remote{target: target, clients: make(map[*websocket.Conn]string), services: make(map[string]string)}
	for _, item := range list {
		h.services[item.ID] = item.URL
	}
	h.reset()
	return h
}

func (h *remote) reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	number, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		panic(err)
	}
	h.pin = fmt.Sprintf("%06d", number.Int64())
	h.qrSecret = randomToken()
	h.sessions = make(map[string]time.Time)
	h.attempts = nil
	if h.drag != nil {
		_ = h.target.holdLeft(false)
		h.drag = nil
	}
	for client := range h.clients {
		_ = client.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(4001, "Pair again"), time.Now().Add(75*time.Millisecond))
		_ = client.Close()
		delete(h.clients, client)
	}
}

func (h *remote) status() (pin, secret string, count int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, token := range h.clients {
		if h.authorized(token) {
			count++
		}
	}
	return h.pin, h.qrSecret, count
}

const cookieName = "stream_remote"
const sessionTime = 24 * time.Hour

func tokenFrom(r *http.Request) string {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func (h *remote) authorized(token string) bool {
	return time.Now().Before(h.sessions[token])
}

func sameOrigin(r *http.Request) bool {
	return r.Header.Get("Origin") == "http://"+r.Host
}

func localRequest(r *http.Request) bool {
	local, ok := r.Context().Value(http.LocalAddrContextKey).(*net.TCPAddr)
	if !ok {
		return false
	}
	host, port, err := net.SplitHostPort(r.Host)
	if err != nil || port != strconv.Itoa(local.Port) {
		return false
	}
	if !local.IP.Equal(net.ParseIP(host)) && !(host == "localhost" && local.IP.IsLoopback()) {
		return false
	}
	sourceHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	source, err := netip.ParseAddr(sourceHost)
	if err != nil || !source.Is4() {
		return false
	}
	destination, ok := netip.AddrFromSlice(local.IP.To4())
	if !ok {
		return false
	}
	if destination.IsLoopback() {
		return source.IsLoopback()
	}
	if source == destination {
		return true
	}
	for _, adapter := range localNetworks() {
		if adapter.prefix.Addr() == destination && adapter.prefix.Contains(source) {
			return true
		}
	}
	return false
}

func (h *remote) handler() http.Handler {
	web, err := fs.Sub(assets, "web")
	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/state", h.state)
	mux.HandleFunc("POST /api/pair", h.pair)
	mux.HandleFunc("POST /api/disconnect", h.disconnect)
	mux.HandleFunc("GET /api/control", h.control)
	mux.Handle("GET /", http.FileServerFS(web))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !localRequest(r) {
			http.Error(w, "Open the address shown in Stream Remote.", http.StatusForbidden)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; font-src 'self'; img-src 'self'; connect-src 'self' ws://"+r.Host+"; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
		mux.ServeHTTP(w, r)
	})
}

func (h *remote) state(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	paired := h.authorized(tokenFrom(r))
	h.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"paired": paired})
}

func (h *remote) pair(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		http.Error(w, "Open Stream Remote directly in your browser.", http.StatusForbidden)
		return
	}
	var supplied struct {
		PIN    string `json:"pin"`
		Secret string `json:"secret"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 512)
	decoder := json.NewDecoder(r.Body)
	if decoder.Decode(&supplied) != nil || decoder.Decode(new(any)) != io.EOF {
		http.Error(w, "Enter the pairing code shown on your PC.", http.StatusBadRequest)
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	if supplied.Secret != "" {
		if subtle.ConstantTimeCompare([]byte(supplied.Secret), []byte(h.qrSecret)) != 1 {
			http.Error(w, "This QR code has changed. Scan the code on your PC again.", http.StatusUnauthorized)
			return
		}
	} else {
		for len(h.attempts) > 0 && now.Sub(h.attempts[0]) > time.Minute {
			h.attempts = h.attempts[1:]
		}
		if len(h.attempts) >= 18 {
			http.Error(w, "Too many attempts. Wait a minute or scan the QR code.", http.StatusTooManyRequests)
			return
		}
		if subtle.ConstantTimeCompare([]byte(supplied.PIN), []byte(h.pin)) != 1 {
			h.attempts = append(h.attempts, now)
			http.Error(w, "That code did not match. Check the code on your PC.", http.StatusUnauthorized)
			return
		}
	}
	if h.authorized(tokenFrom(r)) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	for token, expires := range h.sessions {
		if !now.Before(expires) {
			delete(h.sessions, token)
		}
	}
	if len(h.sessions) >= 48 {
		http.Error(w, "Too many paired browsers. Reset pairing on your PC.", http.StatusServiceUnavailable)
		return
	}
	token := randomToken()
	h.sessions[token] = now.Add(sessionTime)
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: int(sessionTime.Seconds())})
	w.WriteHeader(http.StatusNoContent)
}

func (h *remote) disconnect(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		http.Error(w, "Open Stream Remote directly in your browser.", http.StatusForbidden)
		return
	}
	token := tokenFrom(r)
	h.mu.Lock()
	delete(h.sessions, token)
	for client, owner := range h.clients {
		if owner == token {
			h.remove(client)
			_ = client.Close()
		}
	}
	h.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: cookieName, Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	w.WriteHeader(http.StatusNoContent)
}

var upgrader = websocket.Upgrader{CheckOrigin: sameOrigin, HandshakeTimeout: 4 * time.Second}

func (h *remote) control(w http.ResponseWriter, r *http.Request) {
	token := tokenFrom(r)
	h.mu.Lock()
	if !h.authorized(token) {
		h.mu.Unlock()
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if len(h.clients) >= 24 {
		h.mu.Unlock()
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.mu.Unlock()
		return
	}
	h.clients[conn] = token
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		h.remove(conn)
		h.mu.Unlock()
		_ = conn.Close()
	}()
	conn.SetReadLimit(24576)
	window := time.Now()
	count := 0
	for {
		_ = conn.SetReadDeadline(time.Now().Add(16 * time.Second))
		var cmd command
		if conn.ReadJSON(&cmd) != nil {
			return
		}
		if time.Since(window) >= time.Second {
			window, count = time.Now(), 0
		}
		count++
		if count > 180 {
			return
		}
		h.mu.Lock()
		if !h.authorized(token) {
			h.mu.Unlock()
			_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(4001, "Pair again"), time.Now().Add(time.Second))
			return
		}
		err := h.run(conn, cmd)
		h.mu.Unlock()
		if cmd.ID != 0 || cmd.Kind == "ping" {
			result := struct {
				ID    int    `json:"id"`
				Error string `json:"error,omitempty"`
			}{ID: cmd.ID}
			if err != nil {
				result.Error = err.Error()
			}
			_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
			if conn.WriteJSON(result) != nil {
				return
			}
		}
	}
}

func (h *remote) remove(conn *websocket.Conn) {
	if h.drag == conn {
		_ = h.target.holdLeft(false)
		h.drag = nil
	}
	delete(h.clients, conn)
}

func (h *remote) run(conn *websocket.Conn, c command) error {
	switch c.Kind {
	case "move":
		if h.drag == nil || h.drag == conn {
			return h.target.move(bound(c.X, 800), bound(c.Y, 800))
		}
		return nil
	case "scroll":
		return h.target.scroll(bound(c.Amount, 1200))
	case "click":
		if c.Button != "left" && c.Button != "right" {
			break
		}
		if h.drag != nil {
			return errors.New("Release Drag before clicking.")
		}
		return h.target.click(c.Button)
	case "drag":
		if h.drag != nil && h.drag != conn {
			return errors.New("Another browser is dragging.")
		}
		if c.Down != (h.drag == conn) {
			if err := h.target.holdLeft(c.Down); err != nil {
				return err
			}
			if c.Down {
				h.drag = conn
			} else {
				h.drag = nil
			}
		}
		return nil
	case "key":
		return h.target.key(c.Value)
	case "type":
		if len(c.Value) > 6144 || c.Erase < 0 || c.Erase > 1500 {
			break
		}
		return h.target.typeText(c.Value, c.Erase)
	case "open":
		if url := h.services[c.Value]; url != "" {
			return h.target.open(url)
		}
		break
	case "ping":
		return nil
	default:
		return errors.New("Unknown control.")
	}
	return errors.New("Invalid control value.")
}

func bound(value, limit int) int { return max(-limit, min(value, limit)) }

type address struct {
	prefix netip.Prefix
	name   string
}

func usableIPv4(ip netip.Addr) bool {
	return ip.Is4() && (ip.IsGlobalUnicast() || ip.IsLinkLocalUnicast())
}

func localNetworks() []address {
	ifaces, _ := net.Interfaces()
	addresses := []address{}
	seen := map[string]bool{}
	for _, i := range ifaces {
		if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagLoopback != 0 {
			continue
		}
		list, _ := i.Addrs()
		for _, item := range list {
			prefix, err := netip.ParsePrefix(item.String())
			if err != nil || !usableIPv4(prefix.Addr()) {
				continue
			}
			ip := prefix.Addr().String()
			if !seen[ip] {
				seen[ip] = true
				addresses = append(addresses, address{prefix, i.Name})
			}
		}
	}
	if len(addresses) == 0 {
		return fallbackNetworks()
	}
	return addresses
}

func addresses() []address {
	addresses := localNetworks()
	// A UDP connect asks the OS which adapter has the default route.
	primary := ""
	if conn, err := net.Dial("udp4", "192.0.2.1:80"); err == nil {
		primary = conn.LocalAddr().(*net.UDPAddr).IP.String()
		_ = conn.Close()
	}
	sort.SliceStable(addresses, func(i, j int) bool {
		rank := func(a address) int {
			name := strings.ToLower(a.name)
			if strings.Contains(name, "wi-fi") || strings.Contains(name, "wlan") || strings.Contains(name, "wireless") {
				return 0
			}
			if a.prefix.Addr().String() == primary {
				return 1
			}
			return 2
		}
		return rank(addresses[i]) < rank(addresses[j])
	})
	return addresses
}

func startServer(h *remote, preferred string) (*http.Server, net.Listener, error) {
	listener, err := net.Listen("tcp4", preferred)
	if err != nil {
		host, port, parseErr := net.SplitHostPort(preferred)
		if parseErr == nil && port != "0" {
			listener, err = net.Listen("tcp4", net.JoinHostPort(host, "0"))
		}
	}
	if err != nil {
		return nil, nil, err
	}
	server := &http.Server{
		Handler: h.handler(), ReadHeaderTimeout: 4 * time.Second, ReadTimeout: 8 * time.Second,
		WriteTimeout: 8 * time.Second, IdleTimeout: 25 * time.Second, MaxHeaderBytes: 8192,
	}
	return server, listener, nil
}
