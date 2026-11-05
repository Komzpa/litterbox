package gmailsync

import (
	"context"
	"net/http"
)

// TriggerHandler starts background account syncs for the authenticated tenant.
// The worker context outlives the request but is cancelled with the server.
type TriggerHandler struct {
	Context  context.Context
	Identity func(*http.Request) (string, bool)
	Accounts func(context.Context) ([]Account, error)
	Trigger  func(context.Context, Account) bool
}

func (h TriggerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tenant, ok := h.Identity(r)
	if !ok || tenant == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	accounts, err := h.Accounts(r.Context())
	if err != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	for _, account := range accounts {
		if account.TenantID.String() == tenant {
			h.Trigger(h.Context, account)
		}
	}
	w.WriteHeader(http.StatusAccepted)
}
