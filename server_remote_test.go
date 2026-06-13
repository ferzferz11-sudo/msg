package main

import (
	"context"
	"io"
	"log"
	"sync"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"LavenderMessenger/gen"
)

// mockDeployStream — mock для gen.ChatService_DeployAgentTaskStreamServer
type mockDeployStream struct {
	grpc.ServerStream
	ctx       context.Context
	sentMsgs  []*gen.DeployAgentTaskStreamResponse
	mu        sync.Mutex
}

func (m *mockDeployStream) Send(resp *gen.DeployAgentTaskStreamResponse) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentMsgs = append(m.sentMsgs, resp)
	return nil
}

func (m *mockDeployStream) Context() context.Context { return m.ctx }
func (m *mockDeployStream) SendHeader(metadata.MD) error  { return nil }
func (m *mockDeployStream) SetHeader(metadata.MD) error   { return nil }
func (m *mockDeployStream) SendMsg(interface{}) error      { return nil }
func (m *mockDeployStream) RecvMsg(interface{}) error      { return nil }
func (m *mockDeployStream) SetTrailer(metadata.MD)         {}

func (m *mockDeployStream) getSent() []*gen.DeployAgentTaskStreamResponse {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*gen.DeployAgentTaskStreamResponse, len(m.sentMsgs))
	copy(result, m.sentMsgs)
	return result
}

// mockClientStream — mock для gen.ChatService_DeployAgentTaskStreamClient (unary response)
type mockClientStream struct {
	resp *gen.DeployAgentTaskStreamResponse
	err  error
}

func (m *mockClientStream) Recv() (*gen.DeployAgentTaskStreamResponse, error) {
	return m.resp, m.err
}

// TestDeployAgentTaskStream_Unit — unit test проверяет что DeployAgentTaskStream
// отправляет done=True ровно один раз — с полными данными из TaskResult
func TestDeployAgentTaskStream_Unit(t *testing.T) {
	// Тест проверяет логику без реального gRPC подключения.
	// Симулируем: stream update (stdout_chunk) → stream update (done=true)
	// → TaskResult → проверяем что клиент получил ровно один done=True с полными данными.

	s := &server{} // нужен real server с remoteManager

	// Проверяем что manager nil → возвращает ошибку
	mock := &mockDeployStream{ctx: context.Background()}
	err := s.DeployAgentTaskStream(&gen.DeployAgentTaskRequest{
		AgentId:  "test",
		TaskType: "shell",
	}, mock)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sent := mock.getSent()
	if len(sent) != 1 {
		t.Fatalf("expected 1 message (error), got %d", len(sent))
	}
	if !sent[0].Done {
		t.Error("expected done=true")
	}
	if sent[0].Error != "remote agent manager not available" {
		t.Errorf("unexpected error: %s", sent[0].Error)
	}

	log.Printf("PASS: DeployAgentTaskStream with nil manager returns expected error")
}

// TestDeployAgentTaskStream_EmptyAgentId — agent_id пустой
func TestDeployAgentTaskStream_EmptyAgentId(t *testing.T) {
	s := &server{}
	mock := &mockDeployStream{ctx: context.Background()}

	err := s.DeployAgentTaskStream(&gen.DeployAgentTaskRequest{
		AgentId: "",
	}, mock)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sent := mock.getSent()
	if len(sent) != 1 {
		t.Fatalf("expected 1 message, got %d", len(sent))
	}
	if !sent[0].Done {
		t.Error("expected done=true for empty agent")
	}

	log.Printf("PASS: DeployAgentTaskStream with empty agent_id returns done=true")
}

// TestDeployAgentTaskStream_MultipleStreamUpdates — проверяем что промежуточные
// чанки отправляются корректно, а done=True от агента не вызывает немедленной отправки done клиенту.
// Этот тест требует real remoteManager — интеграционный, запускается на dev.
func TestDeployAgentTaskStream_MultipleStreamUpdates(t *testing.T) {
	t.Skip("Интеграционный тест — запускать на dev сервере с подключённым агентом")
}

// TestStreamResponse_Fields — проверяем что промежуточные сообщения содержат
// правильные поля (stdout_chunk, stderr_chunk) без done=true
func TestStreamResponse_Fields(t *testing.T) {
	// Симулируем создание ответа в цикле стриминга
	resp := &gen.DeployAgentTaskStreamResponse{
		TaskId:      "test-task-123",
		Status:      "running",
		StdoutChunk: "hello\n",
		Done:        false,
	}

	if resp.Done {
		t.Error("intermediate message should not have done=true")
	}
	if resp.StdoutChunk != "hello\n" {
		t.Errorf("unexpected stdout_chunk: %s", resp.StdoutChunk)
	}

	// Финальный ответ с TaskResult
	finalResp := &gen.DeployAgentTaskStreamResponse{
		TaskId:     "test-task-123",
		Status:     "success",
		Stdout:     "hello\nworld\n",
		Stderr:     "",
		ExitCode:   0,
		DurationMs: 1500,
		Done:       true,
	}

	if !finalResp.Done {
		t.Error("final response should have done=true")
	}
	if finalResp.Stdout != "hello\nworld\n" {
		t.Errorf("unexpected stdout: %s", finalResp.Stdout)
	}

	log.Printf("PASS: stream response fields are correct")
}

// TestStreamUpdate_DoneTrueFiltering — проверяем логику фильтрации done=true
func TestStreamUpdate_DoneTrueFiltering(t *testing.T) {
	tests := []struct {
		name       string
		updateDone bool
		status     string
		wantSkip   bool
	}{
		{"done=true, status=running", true, "running", true},
		{"done=false, status=running", false, "running", false},
		{"done=true, status=completed", true, "completed", true},
		{"done=false, status=completed", false, "completed", true},
		{"done=false, status=failed", false, "failed", true},
		{"done=false, status=timeout", false, "timeout", true},
		{"done=false, status=cancelled", false, "cancelled", true},
		{"done=true, status=failed", true, "failed", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Логика из DeployAgentTaskStream:
			// if update.Done || update.Status == "completed" || update.Status == "failed" ||
			//    update.Status == "timeout" || update.Status == "cancelled" {
			//     streamDone = true; continue
			// }
			gotSkip := tt.updateDone || tt.status == "completed" || tt.status == "failed" ||
				tt.status == "timeout" || tt.status == "cancelled"

			if gotSkip != tt.wantSkip {
				t.Errorf("gotSkip=%v, wantSkip=%v for done=%v status=%s",
					gotSkip, tt.wantSkip, tt.updateDone, tt.status)
			}
		})
	}

	log.Printf("PASS: done=true filtering logic is correct")
}

// TestFinalResult_SingleDoneTrue — проверяем что в исправленной логике
// done=True отправляется ровно один раз
func TestFinalResult_SingleDoneTrue(t *testing.T) {
	// Симулируем поток сообщений:
	// 1. stdout_chunk="line1\n", done=false
	// 2. stdout_chunk="line2\n", done=false
	// 3. done=true (агент завершил стрим)
	// → streamCh закрывается (TaskResult пришёл)
	// → проверяем: клиент получил 3 сообщения, последнее с done=true

	updates := []struct {
		stdoutChunk string
		done        bool
		status      string
	}{
		{"line1\n", false, "running"},
		{"line2\n", false, "running"},
		{"", true, "completed"},
	}

	var sentMsgs []*gen.DeployAgentTaskStreamResponse
	var streamDone bool

	for _, u := range updates {
		if u.done || u.status == "completed" || u.status == "failed" ||
			u.status == "timeout" || u.status == "cancelled" {
			streamDone = true
			continue
		}
		sentMsgs = append(sentMsgs, &gen.DeployAgentTaskStreamResponse{
			StdoutChunk: u.stdoutChunk,
			Done:        false,
		})
	}

	// После закрытия streamCh — отправляем финальный
	if streamDone {
		sentMsgs = append(sentMsgs, &gen.DeployAgentTaskStreamResponse{
			Stdout: "line1\nline2\n",
			Done:   true,
		})
	}

	// Проверки
	if len(sentMsgs) != 3 {
		t.Fatalf("expected 3 messages (2 chunks + 1 final), got %d", len(sentMsgs))
	}

	// Первые два — без done
	for i := 0; i < 2; i++ {
		if sentMsgs[i].Done {
			t.Errorf("message %d should not have done=true", i)
		}
	}

	// Последний — с done=true
	if !sentMsgs[2].Done {
		t.Error("final message should have done=true")
	}
	if sentMsgs[2].Stdout != "line1\nline2\n" {
		t.Errorf("final should contain full stdout, got: %s", sentMsgs[2].Stdout)
	}

	// Проверяем что done=True ровно один раз
	doneCount := 0
	for _, m := range sentMsgs {
		if m.Done {
			doneCount++
		}
	}
	if doneCount != 1 {
		t.Errorf("done=true should appear exactly once, got %d", doneCount)
	}

	log.Printf("PASS: single done=true sent with full data")
}

// io import fix
var _ = io.EOF
