package rag

import (
	"context"
)

// VectorResult — результат поиска в векторной БД
type VectorResult struct {
	ID       string         `json:"id"`
	Score    float32        `json:"score"`
	Metadata map[string]any `json:"metadata"`
	Content  string         `json:"content,omitempty"`
}

// VectorSearch — интерфейс для векторной БД (Qdrant, Milvus, pgvector)
type VectorSearch interface {
	// Search — поиск ближайших векторов
	Search(ctx context.Context, embedding []float32, topK int, filters map[string]any) ([]VectorResult, error)

	// Upsert — добавление/обновление вектора
	Upsert(ctx context.Context, id string, embedding []float32, metadata map[string]any) error

	// Delete — удаление вектора
	Delete(ctx context.Context, id string) error
}

// MultimodalEmbedding — результат мультимодального эмбеддинга
type MultimodalEmbedding struct {
	TextEmbedding   []float32   // Эмбеддинг текста
	ImageEmbeddings [][]float32 // Эмбеддинги картинок
	JointEmbedding  []float32   // Объединённый вектор (если модель поддерживает)
}

// EmbeddingService — интерфейс для сервиса эмбеддингов (CLIP, text-embedding-ada, etc.)
type EmbeddingService interface {
	// EmbedText — получить эмбеддинг текста
	EmbedText(ctx context.Context, text string) ([]float32, error)

	// EmbedImage — получить эмбеддинг картинки (CLIP/CNN)
	EmbedImage(ctx context.Context, imageData []byte) ([]float32, error)

	// EmbedMultimodal — мультимодальный эмбеддинг (текст + картинки)
	EmbedMultimodal(ctx context.Context, text string, images [][]byte) (*MultimodalEmbedding, error)
}

// RAGPipeline — интерфейс для RAG-пайплайна
type RAGPipeline interface {
	// BuildContext — собирает контекст из запроса (текст + картинки)
	// Возвращает дополненный промпт и найденные чанки
	BuildContext(ctx context.Context, text string, images [][]byte) (*RAGContext, error)
}

// RAGContext — результат работы RAG-пайплайна
type RAGContext struct {
	// AugmentedPrompt — промпт с добавленным контекстом
	AugmentedPrompt string

	// RetrievedChunks — найденные релевантные чанки
	RetrievedChunks []VectorResult

	// HasResults — были ли найдены результаты
	HasResults bool
}
