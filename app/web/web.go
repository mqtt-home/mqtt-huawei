package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/mqtt-home/mqtt-huawei/huawei"
	"github.com/philipparndt/go-logger"
	loggerchi "github.com/philipparndt/go-logger/chi"
)

type SSEClient struct {
	ID      string
	Channel chan string
}

type WebServer struct {
	client       *huawei.Client
	router       *chi.Mux
	sseClients   map[string]*SSEClient
	sseClientsMu sync.RWMutex
	statusChan   chan huawei.InverterStatus
}

func NewWebServer(client *huawei.Client) *WebServer {
	ws := &WebServer{
		client:     client,
		router:     chi.NewRouter(),
		sseClients: make(map[string]*SSEClient),
		statusChan: make(chan huawei.InverterStatus, 10),
	}

	client.AddStatusChangeListener(ws.onStatusChange)

	ws.setupRoutes()
	go ws.broadcastLoop()

	return ws
}

func (ws *WebServer) onStatusChange(status huawei.InverterStatus) {
	select {
	case ws.statusChan <- status:
	default:
		// Channel full, skip
	}
}

func (ws *WebServer) broadcastLoop() {
	for status := range ws.statusChan {
		ws.broadcastStatus(status)
	}
}

func (ws *WebServer) setupRoutes() {
	ws.router.Use(loggerchi.Logger())
	ws.router.Use(middleware.Recoverer)

	ws.router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	ws.router.Route("/api", func(r chi.Router) {
		r.Get("/health", ws.healthCheck)
		r.Get("/status", ws.getStatus)
		r.Get("/events", ws.handleSSE)
	})

	// Serve static files (React app)
	fileServer := http.FileServer(http.Dir("./web/dist/"))
	ws.router.Handle("/*", fileServer)
}

func (ws *WebServer) healthCheck(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":     "ok",
		"goroutines": runtime.NumGoroutine(),
		"sse_clients": func() int {
			ws.sseClientsMu.RLock()
			defer ws.sseClientsMu.RUnlock()
			return len(ws.sseClients)
		}(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

func (ws *WebServer) getStatus(w http.ResponseWriter, r *http.Request) {
	status := ws.client.GetStatus()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (ws *WebServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	clientID := fmt.Sprintf("%d", time.Now().UnixNano())
	logger.Info("SSE client connected", "id", clientID)

	channel := make(chan string, 10)

	ws.sseClientsMu.Lock()
	ws.sseClients[clientID] = &SSEClient{
		ID:      clientID,
		Channel: channel,
	}
	ws.sseClientsMu.Unlock()

	// Send initial state
	status := ws.client.GetStatus()
	message, _ := json.Marshal(status)
	fmt.Fprintf(w, "data: %s\n\n", string(message))

	flusher, ok := w.(http.Flusher)
	if ok {
		flusher.Flush()
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	defer func() {
		logger.Info("SSE client disconnected", "id", clientID)
		ws.sseClientsMu.Lock()
		delete(ws.sseClients, clientID)
		close(channel)
		ws.sseClientsMu.Unlock()
	}()

	for {
		select {
		case msg := <-channel:
			_, writeErr := fmt.Fprintf(w, "data: %s\n\n", msg)
			if writeErr != nil {
				return
			}
			if ok {
				flusher.Flush()
			}
		case <-r.Context().Done():
			return
		case <-ticker.C:
			status := ws.client.GetStatus()
			message, _ := json.Marshal(status)
			_, writeErr := fmt.Fprintf(w, "data: %s\n\n", string(message))
			if writeErr != nil {
				return
			}
			if ok {
				flusher.Flush()
			}
		}
	}
}

func (ws *WebServer) broadcastStatus(status huawei.InverterStatus) {
	message, err := json.Marshal(status)
	if err != nil {
		logger.Error("Failed to marshal status", "error", err)
		return
	}
	messageStr := string(message)

	ws.sseClientsMu.RLock()
	for _, client := range ws.sseClients {
		select {
		case client.Channel <- messageStr:
		default:
			// Channel full, skip
		}
	}
	ws.sseClientsMu.RUnlock()
}

func (ws *WebServer) Start(port int) error {
	addr := ":" + strconv.Itoa(port)
	logger.Info("Starting web server", "address", addr)
	return http.ListenAndServe(addr, ws.router)
}
