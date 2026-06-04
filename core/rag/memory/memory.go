package memory

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"unicode"

	"LavenderMessenger/core/rag"
)

// InMemoryEmbeddingService — текстовые эмбеддинги через TF-IDF хеширование
// Не требует внешних зависимостей, работает полностью в памяти
type InMemoryEmbeddingService struct {
	dim     int
	mu      sync.RWMutex
	vocab   map[string]int // слово → индекс в словаре
	nextIdx int
}

func NewInMemoryEmbeddingService(dim int) *InMemoryEmbeddingService {
	if dim <= 0 {
		dim = 384
	}
	return &InMemoryEmbeddingService{
		dim:   dim,
		vocab: make(map[string]int),
	}
}

func (s *InMemoryEmbeddingService) EmbedText(ctx context.Context, text string) ([]float32, error) {
	return s.textToVector(text), nil
}

func (s *InMemoryEmbeddingService) EmbedImage(ctx context.Context, imageData []byte) ([]float32, error) {
	// Для картинок — простой хеш первых 1024 байт
	h := hashBytesLimited(imageData, 1024)
	vec := make([]float32, s.dim)
	for i := range vec {
		h = h*1103515245 + 12345 + int64(i)
		if h < 0 {
			h = -h
		}
		vec[i] = float32(h%1000) / 1000.0
	}
	return normalize(vec), nil
}

func (s *InMemoryEmbeddingService) EmbedMultimodal(ctx context.Context, text string, images [][]byte) (*rag.MultimodalEmbedding, error) {
	textVec := s.textToVector(text)
	result := &rag.MultimodalEmbedding{
		TextEmbedding: textVec,
	}

	for _, img := range images {
		imgVec, err := s.EmbedImage(ctx, img)
		if err != nil {
			continue
		}
		result.ImageEmbeddings = append(result.ImageEmbeddings, imgVec)
	}

	// Joint embedding — усреднение
	if len(result.ImageEmbeddings) > 0 {
		result.JointEmbedding = make([]float32, s.dim)
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

func (s *InMemoryEmbeddingService) textToVector(text string) []float32 {
	vec := make([]float32, s.dim)
	words := tokenize(text)

	if len(words) == 0 {
		return vec
	}

	// TF-IDF подход: каждое слово добавляет вклад в вектор
	for _, word := range words {
		s.mu.RLock()
		idx, exists := s.vocab[word]
		s.mu.RUnlock()

		if !exists {
			s.mu.Lock()
			idx, exists = s.vocab[word]
			if !exists {
				idx = s.nextIdx
				s.vocab[word] = idx
				s.nextIdx++
			}
			s.mu.Unlock()
		}

		// Распределяем вклад слова по вектору через хеширование
		h := hashString(word)
		for j := 0; j < 4 && j < s.dim; j++ {
			pos := int((h + int64(j)*31) % int64(s.dim))
			if pos < 0 {
				pos = -pos
			}
			vec[pos] += 1.0
		}
	}

	// L2 нормализация
	return normalize(vec)
}

// InMemoryVectorDB — векторное хранилище в памяти
type InMemoryVectorDB struct {
	mu      sync.RWMutex
	vectors map[string]*entry
	dim     int
}

type entry struct {
	ID       string
	Vector   []float32
	Metadata map[string]any
	Content  string
}

func NewInMemoryVectorDB(dim int) *InMemoryVectorDB {
	if dim <= 0 {
		dim = 384
	}
	return &InMemoryVectorDB{
		vectors: make(map[string]*entry),
		dim:     dim,
	}
}

func (db *InMemoryVectorDB) Search(ctx context.Context, embedding []float32, topK int, filters map[string]any) ([]rag.VectorResult, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if topK <= 0 {
		topK = 5
	}

	type scored struct {
		entry *entry
		score float32
	}

	results := make([]scored, 0, len(db.vectors))
	for _, e := range db.vectors {
		if filters != nil && !matchesFilters(e.Metadata, filters) {
			continue
		}
		sim := cosineSimilarity(embedding, e.Vector)
		results = append(results, scored{entry: e, score: sim})
	}

	// Сортировка по score (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

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

func (db *InMemoryVectorDB) Upsert(ctx context.Context, id string, embedding []float32, metadata map[string]any) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	content, _ := metadata["content"].(string)
	db.vectors[id] = &entry{
		ID:       id,
		Vector:   embedding,
		Metadata: metadata,
		Content:  content,
	}
	return nil
}

func (db *InMemoryVectorDB) Delete(ctx context.Context, id string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	delete(db.vectors, id)
	return nil
}

// InMemoryRAGPipeline — полный RAG пайплайн в памяти
type InMemoryRAGPipeline struct {
	embedder rag.EmbeddingService
	vectorDB rag.VectorSearch
}

func NewInMemoryRAGPipeline(embedder rag.EmbeddingService, vectorDB rag.VectorSearch) *InMemoryRAGPipeline {
	return &InMemoryRAGPipeline{
		embedder: embedder,
		vectorDB: vectorDB,
	}
}

func (p *InMemoryRAGPipeline) BuildContext(ctx context.Context, text string, images [][]byte) (*rag.RAGContext, error) {
	embedResult, err := p.embedder.EmbedMultimodal(ctx, text, images)
	if err != nil {
		return &rag.RAGContext{AugmentedPrompt: text, HasResults: false}, nil
	}

	searchVec := embedResult.JointEmbedding
	if searchVec == nil {
		searchVec = embedResult.TextEmbedding
	}

	results, err := p.vectorDB.Search(ctx, searchVec, 5, nil)
	if err != nil || len(results) == 0 {
		return &rag.RAGContext{AugmentedPrompt: text, HasResults: false}, nil
	}

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

// Вспомогательные функции

func tokenize(text string) []string {
	var words []string
	var current strings.Builder

	for _, r := range strings.ToLower(text) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
		}
	}
	if current.Len() > 0 {
		words = append(words, current.String())
	}

	return words
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

func hashBytesLimited(b []byte, limit int) int64 {
	if len(b) > limit {
		b = b[:limit]
	}
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
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	if sum == 0 {
		return v
	}
	inv := float32(1.0 / math.Sqrt(sum))
	for i := range v {
		v[i] *= inv
	}
	return v
}

func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(normA) * math.Sqrt(normB)))
}

func matchesFilters(metadata map[string]any, filters map[string]any) bool {
	for k, v := range filters {
		if mv, ok := metadata[k]; !ok || fmt.Sprintf("%v", mv) != fmt.Sprintf("%v", v) {
			return false
		}
	}
	return true
}
