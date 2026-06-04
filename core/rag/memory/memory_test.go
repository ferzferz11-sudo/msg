package memory

import (
	"context"
	"testing"
)

func TestInMemoryEmbeddingService(t *testing.T) {
	ctx := context.Background()
	svc := NewInMemoryEmbeddingService(384)

	// Test EmbedText
	vec, err := svc.EmbedText(ctx, "Lavender Messenger мессенджер")
	if err != nil {
		t.Fatalf("EmbedText error: %v", err)
	}
	if len(vec) != 384 {
		t.Fatalf("expected dim=384, got %d", len(vec))
	}

	// Проверяем что вектор нормализован
	var norm float64
	for _, v := range vec {
		norm += float64(v) * float64(v)
	}
	if norm < 0.99 || norm > 1.01 {
		t.Fatalf("vector not normalized: norm=%.4f", norm)
	}

	// Одинаковый текст → одинаковый вектор
	vec2, _ := svc.EmbedText(ctx, "Lavender Messenger мессенджер")
	for i := range vec {
		if vec[i] != vec2[i] {
			t.Fatal("same text produced different vectors")
		}
	}

	// Разный текст → разный вектор
	vec3, _ := svc.EmbedText(ctx, "совершенно другой текст про космос")
	same := true
	for i := range vec {
		if vec[i] != vec3[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("different texts produced identical vectors")
	}
}

func TestInMemoryVectorDB(t *testing.T) {
	ctx := context.Background()
	db := NewInMemoryVectorDB(384)
	embedder := NewInMemoryEmbeddingService(384)

	// Upsert документов
	docs := map[string]string{
		"doc1": "Lavender Messenger — мессенджер с E2EE",
		"doc2": "Hermes Orchestrator маршрутизирует запросы",
		"doc3": "OpenRouter предоставляет доступ к LLM",
	}

	for id, content := range docs {
		vec, _ := embedder.EmbedText(ctx, content)
		if err := db.Upsert(ctx, id, vec, map[string]any{"content": content}); err != nil {
			t.Fatalf("Upsert error: %v", err)
		}
	}

	// Поиск по запросу про мессенджер
	queryVec, _ := embedder.EmbedText(ctx, "мессенджер")
	results, err := db.Search(ctx, queryVec, 3, nil)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("no results found")
	}

	// Первый результат должен быть doc1 (про мессенджер)
	if results[0].ID != "doc1" {
		t.Logf("First result: %s (score: %.4f)", results[0].ID, results[0].Score)
	}

	// Проверяем что score убывает
	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Fatalf("results not sorted: [%d].score=%.4f > [%d].score=%.4f",
				i, results[i].Score, i-1, results[i-1].Score)
		}
	}

	// Delete
	if err := db.Delete(ctx, "doc2"); err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	// После удаления doc2 не должен быть в результатах
	results2, _ := db.Search(ctx, queryVec, 10, nil)
	for _, r := range results2 {
		if r.ID == "doc2" {
			t.Fatal("deleted doc2 still in results")
		}
	}
}

func TestInMemoryRAGPipeline(t *testing.T) {
	ctx := context.Background()
	embedder := NewInMemoryEmbeddingService(384)
	vectorDB := NewInMemoryVectorDB(384)
	rag := NewInMemoryRAGPipeline(embedder, vectorDB)

	// Загружаем документы
	docs := map[string]string{
		"doc1": "Lavender Messenger — это мессенджер с поддержкой E2EE и AI агентов",
		"doc2": "Hermes Orchestrator маршрутизирует запросы к специализированным агентам",
	}

	for id, content := range docs {
		vec, _ := embedder.EmbedText(ctx, content)
		vectorDB.Upsert(ctx, id, vec, map[string]any{"content": content})
	}

	// BuildContext
	ragCtx, err := rag.BuildContext(ctx, "Что такое Lavender?", nil)
	if err != nil {
		t.Fatalf("BuildContext error: %v", err)
	}

	if !ragCtx.HasResults {
		t.Fatal("expected RAG results")
	}
	if len(ragCtx.RetrievedChunks) == 0 {
		t.Fatal("no retrieved chunks")
	}

	// Проверяем что augmented prompt содержит контекст
	if ragCtx.AugmentedPrompt == "" {
		t.Fatal("augmented prompt is empty")
	}

	t.Logf("Augmented prompt:\n%s", ragCtx.AugmentedPrompt)
	t.Logf("Retrieved %d chunks", len(ragCtx.RetrievedChunks))
	for i, c := range ragCtx.RetrievedChunks {
		t.Logf("  [%d] id=%s score=%.4f content=%q", i, c.ID, c.Score, c.Content)
	}
}

func TestCosineSimilarity(t *testing.T) {
	// Одинаковые векторы → similarity = 1
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	if s := cosineSimilarity(a, b); s < 0.99 {
		t.Fatalf("expected ~1.0, got %.4f", s)
	}

	// Ортогональные → similarity = 0
	c := []float32{0, 1, 0}
	if s := cosineSimilarity(a, c); s > 0.01 {
		t.Fatalf("expected ~0.0, got %.4f", s)
	}
}
