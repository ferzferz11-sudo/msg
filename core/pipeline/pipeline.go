package pipeline

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"LavenderMessenger/core/llm"
	"LavenderMessenger/core/rag"
)

// ToolExecutor — интерфейс для выполнения tool calls на стороне Go
type ToolExecutor interface {
	// Execute — выполняет функцию и возвращает результат
	Execute(ctx context.Context, call llm.ToolCall) string

	// GetToolDefs — возвращает список доступных инструментов
	GetToolDefs() []llm.ToolDef
}

// Pipeline — оркестратор пайплайна: RAG → LLM → Tool Calls
type Pipeline struct {
	router   llm.LLMRouter
	rag      rag.RAGPipeline
	tools    ToolExecutor
}

func NewPipeline(router llm.LLMRouter, ragPipeline rag.RAGPipeline, tools ToolExecutor) *Pipeline {
	return &Pipeline{
		router: router,
		rag:    ragPipeline,
		tools:  tools,
	}
}

// ProcessRequest — полный пайплайн: от запроса до стрима ответов
func (p *Pipeline) ProcessRequest(
	ctx context.Context,
	modelHint string,
	userMessage string,
	images [][]byte,
	history []llm.Message,
	onChunk func(chunk llm.StreamChunk) error,
) error {
	// STEP 1: RAG — собираем контекст
	var augmentedPrompt string
	if p.rag != nil {
		ragCtx, err := p.rag.BuildContext(ctx, userMessage, images)
		if err != nil {
			log.Printf("[Pipeline] RAG error (continuing without): %v", err)
			augmentedPrompt = userMessage
		} else if ragCtx.HasResults {
			augmentedPrompt = ragCtx.AugmentedPrompt
			log.Printf("[Pipeline] RAG found %d chunks", len(ragCtx.RetrievedChunks))
		} else {
			augmentedPrompt = userMessage
		}
	} else {
		augmentedPrompt = userMessage
	}

	// STEP 2: Формируем messages
	messages := make([]llm.Message, 0, len(history)+2)

	// System prompt
	messages = append(messages, llm.Message{
		Role:    "system",
		Content: "You are a helpful AI assistant. Use the provided context to answer accurately.",
	})

	// History
	messages = append(messages, history...)

	// User message (augmented with RAG context)
	userMsg := llm.Message{
		Role:    "user",
		Content: augmentedPrompt,
	}
	if len(images) > 0 {
		userMsg.Images = images
	}
	messages = append(messages, userMsg)

	// STEP 3: Получаем tool definitions
	var toolDefs []llm.ToolDef
	if p.tools != nil {
		toolDefs = p.tools.GetToolDefs()
	}

	// STEP 4: Stream с tool calling loop
	return p.streamWithToolCalls(ctx, modelHint, messages, toolDefs, onChunk)
}

// streamWithToolCalls — стримит ответ, обрабатывая tool calls
func (p *Pipeline) streamWithToolCalls(
	ctx context.Context,
	modelHint string,
	messages []llm.Message,
	tools []llm.ToolDef,
	onChunk func(chunk llm.StreamChunk) error,
) error {
	maxIterations := 3 // Защита от бесконечных циклов

	for iter := 0; iter < maxIterations; iter++ {
		chunks, err := p.router.Route(ctx, modelHint, messages, tools)
		if err != nil {
			return fmt.Errorf("llm route: %w", err)
		}

		var pendingCalls []llm.ToolCall
		var contentBuf strings.Builder

		for chunk := range chunks {
			if chunk.Error != "" {
				return fmt.Errorf("llm error: %s", chunk.Error)
			}

			if chunk.ToolCall != nil {
				pendingCalls = append(pendingCalls, *chunk.ToolCall)
			}

			if chunk.Content != "" {
				contentBuf.WriteString(chunk.Content)
				if err := onChunk(chunk); err != nil {
					return fmt.Errorf("onChunk: %w", err)
				}
			}

			if chunk.Done {
				// Есть pending tool calls — выполняем и продолжаем
				if len(pendingCalls) > 0 && p.tools != nil {
					log.Printf("[Pipeline] executing %d tool calls (iter %d)", len(pendingCalls), iter)

					// Добавляем assistant message с tool calls
					messages = append(messages, llm.Message{
						Role:    "assistant",
						Content: contentBuf.String(),
					})

					// Выполняем каждый tool call
					for _, tc := range pendingCalls {
						result := p.tools.Execute(ctx, tc)
						messages = append(messages, llm.Message{
							Role:    "tool",
							Content: result,
						})
					}

					// Продолжаем цикл — отправляем результаты обратно в LLM
					break
				}

				// Нет tool calls — завершаем
				return onChunk(llm.StreamChunk{Done: true})
			}
		}
	}

	return fmt.Errorf("tool calling: max iterations (%d) reached", maxIterations)
}

// NoOpToolExecutor — заглушка для ToolExecutor (без инструментов)
type NoOpToolExecutor struct{}

func (n *NoOpToolExecutor) Execute(_ context.Context, call llm.ToolCall) string {
	return fmt.Sprintf("Tool %s not available", call.Function.Name)
}

func (n *NoOpToolExecutor) GetToolDefs() []llm.ToolDef {
	return nil
}

// Используем time для возможных future enhancements
var _ = time.Now
