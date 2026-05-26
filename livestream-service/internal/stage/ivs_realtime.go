package stage

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awsrt "github.com/aws/aws-sdk-go-v2/service/ivsrealtime"
	rttypes "github.com/aws/aws-sdk-go-v2/service/ivsrealtime/types"
	"github.com/aws/smithy-go"
)

// awsStageName sanitises a human-readable title so it satisfies the IVS
// Real-Time Name constraint: ^[a-zA-Z0-9-_]*$, max 128 chars.
var reInvalidName = regexp.MustCompile(`[^a-zA-Z0-9\-_]+`)

func awsStageName(title string) string {
	name := reInvalidName.ReplaceAllString(strings.TrimSpace(title), "_")
	if len(name) > 128 {
		name = name[:128]
	}
	return name
}

// wrapAWSErr enriches AWS API errors with their type code and message so
// "ValidationException:" (empty) becomes "ValidationException: <detail>".
func wrapAWSErr(op string, err error) error {
	if err == nil {
		return nil
	}
	var ae smithy.APIError
	if errors.As(err, &ae) {
		return fmt.Errorf("ivs-realtime %s: %s: %s", op, ae.ErrorCode(), ae.ErrorMessage())
	}
	return fmt.Errorf("ivs-realtime %s: %w", op, err)
}

// IVSRealTimeClient is the interface used by Service to talk to AWS IVS Real-Time.
// Both AWSIVSRealTime (real) and MockIVSRealTime (test/local) implement it.
type IVSRealTimeClient interface {
	CreateStage(ctx context.Context, title string) (arn string, err error)
	DeleteStage(ctx context.Context, stageARN string) error
	CreateParticipantToken(ctx context.Context, stageARN, userID string, caps []rttypes.ParticipantTokenCapability, durationMin int32) (*ParticipantToken, error)
	DisconnectParticipant(ctx context.Context, stageARN, participantID, reason string) error
}

// ──────────────────────────────────────────────────────────────────────────────
// Real AWS implementation
// ──────────────────────────────────────────────────────────────────────────────

// AWSIVSRealTime calls the real AWS IVS Real-Time API.
type AWSIVSRealTime struct {
	client *awsrt.Client
}

func NewAWSIVSRealTime(ctx context.Context, region string) (*AWSIVSRealTime, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load aws config for ivs-realtime: %w", err)
	}
	return &AWSIVSRealTime{client: awsrt.NewFromConfig(cfg)}, nil
}

func (a *AWSIVSRealTime) CreateStage(ctx context.Context, title string) (string, error) {
	name := awsStageName(title)
	input := &awsrt.CreateStageInput{}
	if name != "" {
		input.Name = &name
	}
	out, err := a.client.CreateStage(ctx, input)
	if err != nil {
		return "", wrapAWSErr("CreateStage", err)
	}
	return *out.Stage.Arn, nil
}

func (a *AWSIVSRealTime) DeleteStage(ctx context.Context, stageARN string) error {
	_, err := a.client.DeleteStage(ctx, &awsrt.DeleteStageInput{Arn: &stageARN})
	if err != nil {
		return wrapAWSErr("DeleteStage", err)
	}
	return nil
}

func (a *AWSIVSRealTime) CreateParticipantToken(
	ctx context.Context,
	stageARN, userID string,
	caps []rttypes.ParticipantTokenCapability,
	durationMin int32,
) (*ParticipantToken, error) {
	out, err := a.client.CreateParticipantToken(ctx, &awsrt.CreateParticipantTokenInput{
		StageArn:     &stageARN,
		UserId:       &userID,
		Capabilities: caps,
		Duration:     &durationMin,
	})
	if err != nil {
		return nil, wrapAWSErr("CreateParticipantToken", err)
	}
	tok := out.ParticipantToken
	capsStr := make([]string, len(caps))
	for i, c := range caps {
		capsStr[i] = string(c)
	}
	return &ParticipantToken{
		StageARN:      stageARN,
		ParticipantID: *tok.ParticipantId,
		Token:         *tok.Token,
		ExpiresAt:     *tok.ExpirationTime,
		Capabilities:  capsStr,
	}, nil
}

func (a *AWSIVSRealTime) DisconnectParticipant(ctx context.Context, stageARN, participantID, reason string) error {
	_, err := a.client.DisconnectParticipant(ctx, &awsrt.DisconnectParticipantInput{
		StageArn:      &stageARN,
		ParticipantId: &participantID,
		Reason:        &reason,
	})
	if err != nil {
		return wrapAWSErr("DisconnectParticipant", err)
	}
	return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Mock implementation (local dev / unit tests)
// ──────────────────────────────────────────────────────────────────────────────

type MockIVSRealTime struct{}

func (m *MockIVSRealTime) CreateStage(_ context.Context, title string) (string, error) {
	return "arn:aws:ivs:us-east-1:000000000000:stage/mock-" + title, nil
}

func (m *MockIVSRealTime) DeleteStage(_ context.Context, _ string) error { return nil }

func (m *MockIVSRealTime) CreateParticipantToken(
	_ context.Context,
	stageARN, userID string,
	caps []rttypes.ParticipantTokenCapability,
	_ int32,
) (*ParticipantToken, error) {
	capsStr := make([]string, len(caps))
	for i, c := range caps {
		capsStr[i] = string(c)
	}
	return &ParticipantToken{
		StageARN:      stageARN,
		ParticipantID: "mock-pid-" + userID,
		Token:         "mock-token-for-" + userID,
		ExpiresAt:     time.Now().UTC().Add(60 * time.Minute),
		Capabilities:  capsStr,
	}, nil
}

func (m *MockIVSRealTime) DisconnectParticipant(_ context.Context, _, _, _ string) error {
	return nil
}
