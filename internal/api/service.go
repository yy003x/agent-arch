package api

import (
	"context"

	"github.com/young/go/agent-arch/internal/agent"
)

type Service struct {
	engine *agent.Engine
}

func NewService(engine *agent.Engine) *Service {
	return &Service{engine: engine}
}

func (s *Service) Start(ctx context.Context, req StartRequest) (SnapshotResponse, error) {
	if _, err := s.engine.CreateRun(ctx, req.RunID, req.Context, req.MaxRounds); err != nil {
		return SnapshotResponse{}, err
	}
	if err := s.engine.Start(ctx, req.RunID); err != nil {
		return SnapshotResponse{}, err
	}
	return s.GetSnapshot(ctx, req.RunID)
}

func (s *Service) Block(ctx context.Context, runID string, req ReasonRequest) (SnapshotResponse, error) {
	if err := s.engine.Block(ctx, runID, req.Reason); err != nil {
		return SnapshotResponse{}, err
	}
	return s.GetSnapshot(ctx, runID)
}

func (s *Service) Stop(ctx context.Context, runID string, req ReasonRequest) (SnapshotResponse, error) {
	if err := s.engine.Stop(ctx, runID, req.Reason); err != nil {
		return SnapshotResponse{}, err
	}
	return s.GetSnapshot(ctx, runID)
}

func (s *Service) Cancel(ctx context.Context, runID string, req ReasonRequest) (SnapshotResponse, error) {
	if err := s.engine.Cancel(ctx, runID, req.Reason); err != nil {
		return SnapshotResponse{}, err
	}
	return s.GetSnapshot(ctx, runID)
}

func (s *Service) Continue(ctx context.Context, runID string) (SnapshotResponse, error) {
	if err := s.engine.Continue(ctx, runID); err != nil {
		return SnapshotResponse{}, err
	}
	return s.GetSnapshot(ctx, runID)
}

func (s *Service) PatchContextAndResume(ctx context.Context, runID string, req PatchAndResumeRequest) (SnapshotResponse, error) {
	if err := s.engine.PatchContextAndResume(ctx, runID, req.Patch); err != nil {
		return SnapshotResponse{}, err
	}
	return s.GetSnapshot(ctx, runID)
}

func (s *Service) GetSnapshot(ctx context.Context, runID string) (SnapshotResponse, error) {
	snapshot, err := s.engine.GetSnapshot(ctx, runID)
	if err != nil {
		return SnapshotResponse{}, err
	}
	return toSnapshotResponse(snapshot), nil
}

func toSnapshotResponse(snapshot agent.Snapshot) SnapshotResponse {
	return SnapshotResponse{
		RunID:     snapshot.RunID,
		State:     snapshot.State,
		Round:     snapshot.Round,
		MaxRounds: snapshot.MaxRounds,
		LastError: snapshot.LastError,
		Context:   snapshot.Context,
		Events:    snapshot.Events,
		CreatedAt: snapshot.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: snapshot.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
