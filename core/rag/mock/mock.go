package mock

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"

	"LavenderMessenger/core/rag"
)

// MockEmbeddingService — заглушка для EmbeddingService
// Генерирует псевдо-случайные векторы фиксированной размерности
type MockEmbeddingService struct {
	dim int
	mu  sync.Mutex
}

func NewMockEmbeddingService(dim int) *MockEmbeddingService {
	if dim <= 0 {
		dim = 384 // стандартная размерность для маленьких моделей
	}
	return &MockEmbeddingService{dim: dim}
}

func (m *MockEmbeddingService) EmbedText(ctx context.Context, text string) ([]float32, error) {
	// Генерируем детерминированный вектор на основе текста
	return m.textToVector(text), nil
}

func (m *MockEmbeddingService) EmbedImage(ctx context.Context, imageData []byte) ([]float32, error) {
	// Генерируем вектор на основе хеша картинки
	return m.bytesToVector(imageData), nil
}

func (m *MockEmbeddingService) EmbedMultimodal(ctx context.Context, text string, images [][]byte) (*rag.MultimodalEmbedding, error) {
	result := &rag.MultimodalEmbedding{
		TextEmbedding: m.textToVector(text),
	}

	for _, img := range images {
		result.ImageEmbeddings = append(result.ImageEmbeddings, m.bytesToVector(img))
	}

	// Joint embedding — усреднение текста и картинок
	if len(result.ImageEmbeddings) > 0 {
		result.JointEmbedding = make([]float32, m.dim)
		copy(result.JointEmbedding, result.TextEmbedding)
		for _, imgVec := range result.ImageEmbeddings {
			for i := range result.JointEmbedding {
				result.JointEmbedding[i] += imgVec[i]
			}
		}
		n := float32(1 + len(result.ImageEmbeddings))
		for i := range result.JointEmbedding {
			result.JointEmbedding[i] /= n
		}
	} else {
		result.JointEmbedding = result.TextEmbedding
	}

	return result, nil
}

func (m *MockEmbeddingService) textToVector(text string) []float32 {
	// Простой детерминированный хеш текста → вектор
	vec := make([]float32, m.dim)
	h := hashString(text)
	for i := range vec {
		h = h*1103515245 + 12345
		vec[i] = float32(h%1000) / 1000.0
	}
	return normalize(vec)
}

func (m *MockEmbeddingService) bytesToVector(data []byte) []float32 {
	vec := make([]float32, m.dim)
	h := hashBytes(data)
	for i := range vec {
		h = h*1103515245 + 12345
		vec[i] = float32(h%1000) / 1000.0
	}
	return normalize(vec)
}

func hashString(s string) int64 {
	var h int64 = 5381
	for _, c := range s {
		h = ((h << 5) + h) + int64(c)
	}
	if h < 0 {
		h = -h
	}
	return h
}

func hashBytes(b []byte) int64 {
	var h int64 = 5381
	for _, c := range b {
		h = ((h << 5) + h) + int64(c)
	}
	if h < 0 {
		h = -h
	}
	return h
}

func normalize(v []float32) []float32 {
	var sum float32
	for _, x := range v {
		sum += x * x
	}
	if sum == 0 {
		return v
	}
	inv := 1.0 / float32(math.Sqrt(float64(sum)))
	for i := range v {
		v[i] *= inv
	}
	return v
}

// MockVectorDB — заглушка для VectorSearch
// Хранит векторы в памяти
type MockVectorDB struct {
	mu      sync.RWMutex
	vectors map[string]*mockEntry
	dim     int
}

type mockEntry struct {
	ID       string
	Vector   []float32
	Metadata map[string]any
	Content  string
}

func NewMockVectorDB(dim int) *MockVectorDB {
	if dim <= 0 {
		dim = 384
	}
	return &MockVectorDB{
		vectors: make(map[string]*mockEntry),
		dim:     dim,
	}
}

func (db *MockVectorDB) Search(ctx context.Context, embedding []float32, topK int, filters map[string]any) ([]rag.VectorResult, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if topK <= 0 {
		topK = 5
	}

	type scored struct {
		entry *mockEntry
		score float32
	}

	// Вычисляем cosine similarity со всеми векторами
	results := make([]scored, 0, len(db.vectors))
	for _, e := range db.vectors {
		// Применяем фильтры
		if filters != nil && !matchesFilters(e.Metadata, filters) {
			continue
		}
		sim := cosineSimilarity(embedding, e.Vector)
		results = append(results, scored{entry: e, score: sim})
	}

	// Сортируем по score (descending) — простая сортировка вставками для маленьких N
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].score > results[j-1].score; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}

	// Берём topK
	if len(results) > topK {
		results = results[:topK]
	}

	out := make([]rag.VectorResult, 0, len(results))
	for _, r := range results {
		out = append(out, rag.VectorResult{
			ID:       r.entry.ID,
			Score:    r.score,
			Metadata: r.entry.Metadata,
			Content:  r.entry.Content,
		})
	}

	return out, nil
}

func (db *MockVectorDB) Upsert(ctx context.Context, id string, embedding []float32, metadata map[string]any) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	content, _ := metadata["content"].(string)
	db.vectors[id] = &mockEntry{
		ID:       id,
		Vector:   embedding,
		Metadata: metadata,
		Content:  content,
	}
	return nil
}

func (db *MockVectorDB) Delete(ctx context.Context, id string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	delete(db.vectors, id)
	return nil
}

func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float32
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}

func matchesFilters(metadata map[string]any, filters map[string]any) bool {
	for k, v := range filters {
		if mv, ok := metadata[k]; !ok || fmt.Sprintf("%v", mv) != fmt.Sprintf("%v", v) {
			return false
		}
	}
	return true
}

// MockRAGPipeline — заглушка для RAGPipeline
type MockRAGPipeline struct {
	embedder rag.EmbeddingService
	vectorDB rag.VectorSearch
}

func NewMockRAGPipeline(embedder rag.EmbeddingService, vectorDB rag.VectorSearch) *MockRAGPipeline {
	return &MockRAGPipeline{
		embedder: embedder,
		vectorDB: vectorDB,
	}
}

func (p *MockRAGPipeline) BuildContext(ctx context.Context, text string, images [][]byte) (*rag.RAGContext, error) {
	// Получаем мультимодальный эмбеддинг
	embedResult, err := p.embedder.EmbedMultimodal(ctx, text, images)
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}

	// Ищем в векторной БД
	searchVec := embedResult.JointEmbedding
	if searchVec == nil {
		searchVec = embedResult.TextEmbedding
	}

	results, err := p.vectorDB.Search(ctx, searchVec, 5, nil)
	if err != nil {
		log.Printf("[MockRAG] search error: %v", err)
		// Не фатально — продолжаем без RAG
		return &rag.RAGContext{
			AugmentedPrompt: text,
			HasResults:      false,
		}, nil
	}

	if len(results) == 0 {
		return &rag.RAGContext{
			AugmentedPrompt: text,
			HasResults:      false,
		}, nil
	}

	// Формируем augmented prompt
	var sb strings.Builder
	sb.WriteString("Relevant context:\n")
	for i, r := range results {
		sb.WriteString(fmt.Sprintf("[%d] (score: %.2f) %s\n", i+1, r.Score, r.Content))
	}
	sb.WriteString("\nUser query: ")
	sb.WriteString(text)

	return &rag.RAGContext{
		AugmentedPrompt: sb.String(),
		RetrievedChunks: results,
		HasResults:      true,
	}, nil
}

// Используем rand и time для seed
var _ = rand.Int
var _ = time.Now
