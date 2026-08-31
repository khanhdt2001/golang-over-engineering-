package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"over-engineering/task/audit"
	"over-engineering/task/service"
)

type fakePublisher struct{ events []audit.Event }

func (p *fakePublisher) Publish(_ context.Context, event audit.Event) error {
	p.events = append(p.events, event)
	return nil
}
func (*fakePublisher) Close() error { return nil }

type fakeRepository struct{}

func (fakeRepository) Create(_ context.Context, task service.Task) (service.Task, error) {
	task.ID = "22222222-2222-2222-2222-222222222222"
	return task, nil
}

func TestCreateTaskPublishesInputAndOutput(t *testing.T) {
	publisher := &fakePublisher{}
	handler := NewWithPublisher(service.New(fakeRepository{}, func(context.Context, string) error { return nil }), publisher)
	request := httptest.NewRequest(http.MethodPost, "/v1/tasks", strings.NewReader(`{"name":"Ship","description":"","user_id":"11111111-1111-1111-1111-111111111111"}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if len(publisher.events) != 1 || publisher.events[0].Protocol != "http" || !strings.Contains(string(publisher.events[0].Input), `"Ship"`) || !strings.Contains(string(publisher.events[0].Output), `"22222222-2222-2222-2222-222222222222"`) {
		t.Fatalf("unexpected audit events: %#v", publisher.events)
	}
}
func (fakeRepository) FindByID(_ context.Context, id string) (service.Task, error) {
	return service.Task{ID: id, Name: "Ship", UserID: "11111111-1111-1111-1111-111111111111"}, nil
}
func (fakeRepository) FindByUserID(context.Context, string) ([]service.Task, error) {
	return []service.Task{}, nil
}
func (fakeRepository) Update(context.Context, string, service.UpdateInput) (service.Task, error) {
	return service.Task{}, nil
}

func TestCreateTask(t *testing.T) {
	handler := New(service.New(fakeRepository{}, func(context.Context, string) error { return nil }))
	request := httptest.NewRequest(http.MethodPost, "/v1/tasks", strings.NewReader(`{"name":"Ship","description":"","user_id":"11111111-1111-1111-1111-111111111111"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || !strings.Contains(response.Body.String(), `"id":"22222222-2222-2222-2222-222222222222"`) {
		t.Fatalf("unexpected response: %d %s", response.Code, response.Body.String())
	}
}
