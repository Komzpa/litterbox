package gmailsync

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestTriggerHandlerTenantAndAuthentication(t *testing.T) {
	tenant, neighbor := uuid.New(), uuid.New()
	own := Account{TenantID: tenant, ID: uuid.New()}
	other := Account{TenantID: neighbor, ID: uuid.New()}
	var triggered []Account
	var identity string
	var loadErr error
	h := TriggerHandler{
		Context:  context.Background(),
		Identity: func(*http.Request) (string, bool) { return identity, identity != "" },
		Accounts: func(context.Context) ([]Account, error) { return []Account{own, other}, loadErr },
		Trigger:  func(_ context.Context, a Account) bool { triggered = append(triggered, a); return true },
	}
	request := func() int {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("POST", "/v1/gmail/sync", nil))
		return w.Code
	}
	if status := request(); status != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status=%d", status)
	}
	identity = tenant.String()
	if status := request(); status != http.StatusAccepted {
		t.Fatalf("status=%d", status)
	}
	if len(triggered) != 1 || triggered[0].ID != own.ID {
		t.Fatalf("triggered=%+v", triggered)
	}
	loadErr = errors.New("database unavailable")
	if status := request(); status != http.StatusServiceUnavailable {
		t.Fatalf("failure status=%d", status)
	}
	if len(triggered) != 1 {
		t.Fatal("triggered after failed load")
	}
}
