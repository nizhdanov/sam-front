package grpc

import (
	"context"
	"log/slog"

	"google.golang.org/grpc"
	grpchealth "google.golang.org/grpc/health"

	llmv1 "sam/gen/go/llm/v1"
	"sam/pkg/auth"
	"sam/pkg/grpcx"
	"sam/services/llm/internal/service"
)

func NewServer(log *slog.Logger, v *auth.Verifier, svc *service.Service) (*grpc.Server, *grpchealth.Server) {
	srv, hs := grpcx.NewServer(log, v)
	llmv1.RegisterLLMServiceServer(srv, &server{svc: svc})
	return srv, hs
}

type server struct {
	llmv1.UnimplementedLLMServiceServer
	svc *service.Service
}

func (s *server) MatchSoftware(ctx context.Context, req *llmv1.MatchSoftwareRequest) (*llmv1.MatchSoftwareResponse, error) {
	candidates := make([]service.Candidate, 0, len(req.GetCandidates()))
	for _, c := range req.GetCandidates() {
		candidates = append(candidates, service.Candidate{ID: c.GetId(), Name: c.GetName(), Publisher: c.GetPublisher(), Edition: c.GetEdition()})
	}
	res, err := s.svc.MatchSoftware(ctx, service.Software{
		DisplayName: req.GetDisplayName(), Publisher: req.GetPublisher(), Version: req.GetVersion(),
	}, candidates)
	if err != nil {
		return nil, err
	}
	resp := &llmv1.MatchSoftwareResponse{Status: res.Status, Confidence: res.Confidence, Reason: res.Reason, Model: res.Model}
	if res.ProductID != nil {
		resp.ProductId = *res.ProductID
	}
	return resp, nil
}

func (s *server) Chat(req *llmv1.ChatRequest, stream grpc.ServerStreamingServer[llmv1.ChatResponse]) error {
	history := make([]service.ChatMessage, 0, len(req.GetHistory()))
	for _, m := range req.GetHistory() {
		history = append(history, service.ChatMessage{Role: m.GetRole(), Content: m.GetContent()})
	}
	outcome, err := s.svc.Chat(stream.Context(), history, req.GetMessage(), func(delta string) error {
		return stream.Send(&llmv1.ChatResponse{Event: &llmv1.ChatResponse_Delta{Delta: delta}})
	})
	if err != nil {
		return err
	}
	return stream.Send(&llmv1.ChatResponse{Event: &llmv1.ChatResponse_Done{Done: &llmv1.ChatDone{
		Model:            outcome.Model,
		PromptTokens:     int32(outcome.Usage.PromptTokens),
		CompletionTokens: int32(outcome.Usage.CompletionTokens),
	}}})
}

func (s *server) AnalyzeAgreement(ctx context.Context, req *llmv1.AnalyzeAgreementRequest) (*llmv1.AnalyzeAgreementResponse, error) {
	res, err := s.svc.AnalyzeAgreement(ctx, req.GetAgreementText())
	if err != nil {
		return nil, err
	}
	return &llmv1.AnalyzeAgreementResponse{AnalysisJson: res.JSON, Model: res.Model, PromptVersion: res.PromptVersion}, nil
}
