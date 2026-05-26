package stage

import (
	"context"
	"fmt"
	"time"

	rttypes "github.com/aws/aws-sdk-go-v2/service/ivsrealtime/types"
	"github.com/google/uuid"
)

// defaultTokenDuration is how long a participant token is valid (minutes).
const defaultTokenDuration int32 = 60

// Service contains business logic for IVS Real-Time stages.
type Service struct {
	store Store
	ivs   IVSRealTimeClient
}

func NewService(store Store, ivs IVSRealTimeClient) *Service {
	return &Service{store: store, ivs: ivs}
}

// ──────────────────────────────────────────────────────────────────────────────
// CreateStage – provision an IVS stage and persist it.
// ──────────────────────────────────────────────────────────────────────────────

func (s *Service) CreateStage(ctx context.Context, hostID string, req CreateStageRequest) (*Stage, error) {
	if req.Mode != StageModeCall && req.Mode != StageModeBroadcast {
		return nil, fmt.Errorf("invalid mode: must be CALL or BROADCAST")
	}
	if req.Title == "" {
		return nil, fmt.Errorf("title is required")
	}

	arn, err := s.ivs.CreateStage(ctx, req.Title)
	if err != nil {
		return nil, err
	}

	st := &Stage{
		StageID:   uuid.NewString(),
		StageARN:  arn,
		Mode:      req.Mode,
		HostID:    hostID,
		Title:     req.Title,
		Status:    "ACTIVE",
		CreatedAt: time.Now().UTC(),
	}
	if err := s.store.Put(ctx, st); err != nil {
		return nil, err
	}
	return st, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// GetStage – retrieve stage metadata.
// ──────────────────────────────────────────────────────────────────────────────

func (s *Service) GetStage(ctx context.Context, stageID string) (*Stage, error) {
	return s.store.Get(ctx, stageID)
}

// ──────────────────────────────────────────────────────────────────────────────
// ListMyStages – list stages created by a host.
// ──────────────────────────────────────────────────────────────────────────────

func (s *Service) ListMyStages(ctx context.Context, hostID string) ([]*Stage, error) {
	return s.store.ListByHost(ctx, hostID)
}

// ──────────────────────────────────────────────────────────────────────────────
// Join – create a participant token so a user can enter the stage.
//
// Capability rules:
//   - CALL mode:      every participant gets PUBLISH + SUBSCRIBE
//   - BROADCAST mode: host gets PUBLISH + SUBSCRIBE; guests get SUBSCRIBE only
// ──────────────────────────────────────────────────────────────────────────────

func (s *Service) Join(ctx context.Context, stageID, userID string, req JoinStageRequest) (*ParticipantToken, error) {
	st, err := s.store.Get(ctx, stageID)
	if err != nil {
		return nil, err
	}
	if st.Status != "ACTIVE" {
		return nil, fmt.Errorf("stage_ended")
	}

	caps := s.resolveCaps(st, userID, req.Capabilities)

	tok, err := s.ivs.CreateParticipantToken(ctx, st.StageARN, userID, caps, defaultTokenDuration)
	if err != nil {
		return nil, err
	}
	tok.StageID = stageID
	return tok, nil
}

// resolveCaps determines participant capabilities based on mode and role.
func (s *Service) resolveCaps(st *Stage, userID string, requested []ParticipantCapability) []rttypes.ParticipantTokenCapability {
	// If the caller specified explicit capabilities, honour them.
	if len(requested) > 0 {
		out := make([]rttypes.ParticipantTokenCapability, 0, len(requested))
		for _, c := range requested {
			out = append(out, rttypes.ParticipantTokenCapability(c))
		}
		return out
	}

	switch st.Mode {
	case StageModeCall:
		// Both sides publish and subscribe.
		return []rttypes.ParticipantTokenCapability{
			rttypes.ParticipantTokenCapabilityPublish,
			rttypes.ParticipantTokenCapabilitySubscribe,
		}
	default: // BROADCAST
		if userID == st.HostID {
			return []rttypes.ParticipantTokenCapability{
				rttypes.ParticipantTokenCapabilityPublish,
				rttypes.ParticipantTokenCapabilitySubscribe,
			}
		}
		return []rttypes.ParticipantTokenCapability{
			rttypes.ParticipantTokenCapabilitySubscribe,
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// EndStage – mark stage ended and remove the IVS resource.
// ──────────────────────────────────────────────────────────────────────────────

func (s *Service) EndStage(ctx context.Context, stageID, callerID string) error {
	st, err := s.store.Get(ctx, stageID)
	if err != nil {
		return err
	}
	if st.HostID != callerID {
		return fmt.Errorf("only_host_can_end_stage")
	}
	if err := s.ivs.DeleteStage(ctx, st.StageARN); err != nil {
		return err
	}
	st.Status = "ENDED"
	st.EndedAt = time.Now().UTC()
	return s.store.Update(ctx, st)
}

// ──────────────────────────────────────────────────────────────────────────────
// DisconnectParticipant – kick a participant (host only).
// ──────────────────────────────────────────────────────────────────────────────

func (s *Service) DisconnectParticipant(ctx context.Context, stageID, callerID, participantID, reason string) error {
	st, err := s.store.Get(ctx, stageID)
	if err != nil {
		return err
	}
	if st.HostID != callerID {
		return fmt.Errorf("only_host_can_disconnect_participant")
	}
	return s.ivs.DisconnectParticipant(ctx, st.StageARN, participantID, reason)
}

// keep linter happy if ptrTime is not used outside of store.go
var _ = ptrTime
