package stage

import "time"

// StageMode controls who can publish in a stage.
//   - "CALL"      – 1-to-1: both participants publish and subscribe
//   - "BROADCAST" – 1-to-many: host publishes, guests subscribe only
type StageMode string

const (
	StageModeCall      StageMode = "CALL"
	StageModeBroadcast StageMode = "BROADCAST"
)

// ParticipantCapability maps to IVS real-time capabilities.
type ParticipantCapability string

const (
	CapabilityPublish   ParticipantCapability = "PUBLISH"
	CapabilitySubscribe ParticipantCapability = "SUBSCRIBE"
)

// Stage represents an IVS Real-Time stage resource.
type Stage struct {
	StageID   string    `json:"stage_id" example:"rt-1c95b9af"`
	StageARN  string    `json:"stage_arn" example:"arn:aws:ivs:us-east-1:123456789012:stage/abc"`
	Mode      StageMode `json:"mode" example:"CALL"`
	HostID    string    `json:"host_id" example:"user_123"`
	Title     string    `json:"title" example:"1-to-1 call with Jane"`
	Status    string    `json:"status" example:"ACTIVE"`
	CreatedAt time.Time `json:"created_at" example:"2026-05-26T10:00:00Z"`
	EndedAt   time.Time `json:"ended_at,omitempty" example:"2026-05-26T11:00:00Z"`
}

// CreateStageRequest is the body for POST /stages.
type CreateStageRequest struct {
	Mode  StageMode `json:"mode" example:"CALL"`
	Title string    `json:"title" example:"Private call"`
}

// JoinStageRequest is the body for POST /stages/{id}/join.
type JoinStageRequest struct {
	// Capabilities requested by this participant.  Omit to use mode defaults.
	Capabilities []ParticipantCapability `json:"capabilities,omitempty"`
	// UserID of the participant (resolved from auth when ENABLE_AUTH=true).
	UserID string `json:"user_id,omitempty" example:"user_456"`
}

// ParticipantToken is returned to the client so it can join the stage via
// the Amazon IVS Web Broadcast SDK.
type ParticipantToken struct {
	StageID       string    `json:"stage_id"`
	StageARN      string    `json:"stage_arn"`
	ParticipantID string    `json:"participant_id" example:"p-abc123"`
	Token         string    `json:"token" example:"eyJhbGci..."`
	ExpiresAt     time.Time `json:"expires_at"`
	Capabilities  []string  `json:"capabilities"`
}

// DisconnectRequest is the body for DELETE /stages/{id}/participants/{pid}.
type DisconnectRequest struct {
	Reason string `json:"reason,omitempty" example:"host_removed"`
}
