package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"goshop/services/opsassistant/api/opspb"
	"goshop/services/opsassistant/internal/ollama"
	"goshop/services/opsassistant/internal/timeline"
)

type Options struct {
	Logger *slog.Logger
	LLM    *ollama.Client
	TL     *timeline.Repo
}

type OpsService struct {
	opspb.UnimplementedOpsAssistantServer
	log *slog.Logger
	llm *ollama.Client
	tl  *timeline.Repo
}

func New(opt Options) *OpsService {
	if opt.Logger == nil {
		opt.Logger = slog.Default()
	}
	return &OpsService{log: opt.Logger, llm: opt.LLM, tl: opt.TL}
}

func (s *OpsService) GetTimeline(ctx context.Context, in *opspb.GetTimelineRequest) (*opspb.GetTimelineResponse, error) {
	evs, err := s.tl.Timeline(ctx, in.OrderId)
	if err != nil {
		return nil, fmt.Errorf("timeline: %w", err)
	}
	return &opspb.GetTimelineResponse{Timeline: timeline.RenderMarkdown(evs)}, nil
}

func (s *OpsService) ExplainOrder(ctx context.Context, in *opspb.ExplainOrderRequest) (*opspb.ExplainOrderResponse, error) {
	evs, err := s.tl.Timeline(ctx, in.OrderId)
	if err != nil {
		return nil, fmt.Errorf("timeline: %w", err)
	}

	md := timeline.RenderMarkdown(evs)
	sys := "Ты — SRE помощник. Отвечай кратко и по делу. Делай выводы строго из контекста. Если данных не хватает — скажи, чего не хватает."
	user := strings.TrimSpace(fmt.Sprintf(
		"Вопрос: %s\nКонтекст по заказу %s:\n%s",
		coalesce(in.Question, "Почему заказ в текущем статусе и что произошло по шагам?"),
		in.OrderId, md,
	))

	ans, err := s.llm.Chat(ctx, sys, user)
	if err != nil {
		return nil, fmt.Errorf("llm: %w", err)
	}

	return &opspb.ExplainOrderResponse{
		Answer:   ans,
		Timeline: md,
	}, nil
}

func coalesce(s, d string) string {
	if strings.TrimSpace(s) == "" {
		return d
	}
	return s
}
