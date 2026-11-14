package aipclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListTasksAndStart(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/orgs/o/projects/p/tasks":
			_, _ = w.Write([]byte(`{"data":[{"id":"1","ref":"T-1","title":"x","status":"ready"}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/orgs/o/projects/p/tasks/T-1/runs":
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"id":"r1","status":"queued","agent_type":"development"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	client, err := New(srv.URL, "o", "tok")
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := client.ListTasks(context.Background(), "p")
	if err != nil || len(tasks) != 1 || tasks[0].Ref != "T-1" {
		t.Fatalf("tasks=%v err=%v", tasks, err)
	}
	run, err := client.StartRun(context.Background(), "p", "T-1", "development")
	if err != nil || run.ID != "r1" {
		t.Fatalf("run=%v err=%v", run, err)
	}
}
