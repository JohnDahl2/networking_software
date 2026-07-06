package proto

type Command string

const (
    CmdStart    Command = "start"
    CmdStop     Command = "stop"
    CmdShutdown Command = "shutdown"
    CmdStatus   Command = "status"
)

type Request struct {
    Command   Command `json:"command"`
    Interface string  `json:"interface,omitempty"` 
    SessionID string  `json:"session_id,omitempty"`
}

type Session struct {
    ID           string  `json:"session_id"`
    Interface    string  `json:"interface"`
    Status       string  `json:"status"`
    StartedAt    string  `json:"started_at"`
    EndedAt      *string `json:"ended_at,omitempty"`
    PacketsSaved int64   `json:"packets_saved"`
}

type Response struct {
    OK        bool      `json:"ok"`
    Error     string    `json:"error,omitempty"`
    SessionID string    `json:"session_id,omitempty"`
    Sessions  []Session `json:"sessions,omitempty"`
}