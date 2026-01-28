package api

import (
    "context"
    "encoding/json"
    "log"
    "net/http"
    "sync"
    "time"

    "github.com/gorilla/websocket"
    "github.com/securazion/api/internal/kafka"
)

type WebSocketManager struct {
    clients    map[*websocket.Conn]ClientInfo
    mu         sync.RWMutex
    broadcast  chan []byte
    register   chan *websocket.Conn
    unregister chan *websocket.Conn
    kafka      kafka.Consumer
}

type ClientInfo struct {
    UserID    string
    TenantID  string
    Filters   EventFilters
    Connected time.Time
}

type EventFilters struct {
    EventTypes []string `json:"event_types"`
    Severity   string   `json:"severity"` // "high", "medium", "low", "all"
    Providers  []string `json:"providers"`
}

var upgrader = websocket.Upgrader{
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
    CheckOrigin: func(r *http.Request) bool {
        // In production, validate origin
        return true
    },
}

func NewWebSocketManager(kafkaConsumer kafka.Consumer) *WebSocketManager {
    manager := &WebSocketManager{
        clients:    make(map[*websocket.Conn]ClientInfo),
        broadcast:  make(chan []byte),
        register:   make(chan *websocket.Conn),
        unregister: make(chan *websocket.Conn),
        kafka:      kafkaConsumer,
    }
    
    go manager.run()
    go manager.consumeKafkaEvents()
    
    return manager
}

func (wsm *WebSocketManager) HandleConnection(w http.ResponseWriter, r *http.Request) {
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Printf("Failed to upgrade WebSocket: %v", err)
        return
    }
    
    // Extract user info from JWT token
    userID := r.Context().Value("user_id").(string)
    tenantID := r.Context().Value("tenant_id").(string)
    
    wsm.register <- conn
    
    // Send initial connection confirmation
    conn.WriteJSON(map[string]interface{}{
        "type": "connection_established",
        "data": map[string]interface{}{
            "user_id":   userID,
            "tenant_id": tenantID,
            "timestamp": time.Now().Unix(),
        },
    })
    
    // Handle client messages (e.g., filter updates)
    go wsm.handleClientMessages(conn, userID, tenantID)
}

func (wsm *WebSocketManager) run() {
    for {
        select {
        case client := <-wsm.register:
            wsm.mu.Lock()
            wsm.clients[client] = ClientInfo{
                UserID:    "anonymous", // Will be updated when client sends auth
                Connected: time.Now(),
            }
            wsm.mu.Unlock()
            log.Printf("WebSocket client connected. Total: %d", len(wsm.clients))
            
        case client := <-wsm.unregister:
            wsm.mu.Lock()
            delete(wsm.clients, client)
            wsm.mu.Unlock()
            client.Close()
            log.Printf("WebSocket client disconnected. Total: %d", len(wsm.clients))
            
        case message := <-wsm.broadcast:
            wsm.mu.RLock()
            for client, info := range wsm.clients {
                // Apply client-specific filters before sending
                if wsm.shouldSendToClient(info, message) {
                    err := client.WriteMessage(websocket.TextMessage, message)
                    if err != nil {
                        log.Printf("Error writing to WebSocket: %v", err)
                        wsm.unregister <- client
                    }
                }
            }
            wsm.mu.RUnlock()
        }
    }
}

func (wsm *WebSocketManager) consumeKafkaEvents() {
    // Subscribe to relevant Kafka topics
    topics := []string{
        "findings",
        "alerts",
        "attackpaths",
        "remediation.results",
    }
    
    wsm.kafka.Subscribe(context.Background(), topics)
    
    for {
        select {
        case msg := <-wsm.kafka.Messages():
            // Parse event and prepare for WebSocket broadcast
            wsEvent, err := wsm.prepareWebSocketEvent(msg.Value)
            if err != nil {
                log.Printf("Failed to prepare WebSocket event: %v", err)
                continue
            }
            
            eventJSON, err := json.Marshal(wsEvent)
            if err != nil {
                log.Printf("Failed to marshal WebSocket event: %v", err)
                continue
            }
            
            wsm.broadcast <- eventJSON
        }
    }
}

func (wsm *WebSocketManager) prepareWebSocketEvent(kafkaMessage []byte) (WebSocketEvent, error) {
    var event map[string]interface{}
    if err := json.Unmarshal(kafkaMessage, &event); err != nil {
        return WebSocketEvent{}, err
    }
    
    // Determine event type from Kafka topic or event data
    eventType := wsm.determineEventType(event)
    
    // Create WebSocket-friendly structure
    wsEvent := WebSocketEvent{
        Type:      eventType,
        Timestamp: time.Now().Unix(),
        Data:      event,
    }
    
    // Add metadata for filtering
    wsEvent.Metadata = wsm.extractMetadata(event)
    
    return wsEvent, nil
}

func (wsm *WebSocketManager) shouldSendToClient(info ClientInfo, message []byte) bool {
    // Parse message to check if it matches client filters
    var wsEvent WebSocketEvent
    if err := json.Unmarshal(message, &wsEvent); err != nil {
        return false
    }
    
    // Check event type filter
    if len(info.Filters.EventTypes) > 0 {
        typeMatch := false
        for _, eventType := range info.Filters.EventTypes {
            if wsEvent.Type == eventType {
                typeMatch = true
                break
            }
        }
        if !typeMatch {
            return false
        }
    }
    
    // Check severity filter
    if info.Filters.Severity != "all" {
        if severity, ok := wsEvent.Metadata["severity"].(string); ok {
            if !wsm.severityMatches(info.Filters.Severity, severity) {
                return false
            }
        }
    }
    
    // Check provider filter
    if len(info.Filters.Providers) > 0 {
        if provider, ok := wsEvent.Metadata["provider"].(string); ok {
            providerMatch := false
            for _, p := range info.Filters.Providers {
                if provider == p {
                    providerMatch = true
                    break
                }
            }
            if !providerMatch {
                return false
            }
        }
    }
    
    return true
}

// Placeholder methods for completeness; implementations should be provided elsewhere.
func (wsm *WebSocketManager) handleClientMessages(conn *websocket.Conn, userID, tenantID string) {
    // TODO: Implement handling of client messages such as filter updates.
}

func (wsm *WebSocketManager) determineEventType(event map[string]interface{}) string {
    // TODO: Determine event type based on event content.
    return ""
}

func (wsm *WebSocketManager) extractMetadata(event map[string]interface{}) map[string]interface{} {
    // TODO: Extract metadata like severity and provider for filtering.
    return map[string]interface{}{}
}

func (wsm *WebSocketManager) severityMatches(filterSeverity, eventSeverity string) bool {
    // TODO: Implement severity matching logic.
    return filterSeverity == eventSeverity
}

// WebSocketEvent represents the structure sent to clients.
type WebSocketEvent struct {
    Type      string                 `json:"type"`
    Timestamp int64                  `json:"timestamp"`
    Data      map[string]interface{} `json:"data"`
    Metadata  map[string]interface{} `json:"metadata"`
}
