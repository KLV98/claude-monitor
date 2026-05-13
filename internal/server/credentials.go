package server

import (
	"net/http"
	"time"

	"claude-monitor/internal/account"
	"claude-monitor/internal/keychain"
)

// CredentialsView is the JSON shape returned on GET /api/account/credentials.
// Mirrors the keychain envelope written by `claude auth login` —
// accessToken / refreshToken / expiresAt (unix milliseconds) — so a UI
// can render the same values a user would see in their OS credential
// store, plus an RFC3339 mirror of expiresAt for human display.
//
// This endpoint exposes secrets. The daemon's threat model already
// trusts anyone who can reach the API socket (they can swap accounts,
// trigger logins, etc.), so this is no broader an exposure — but it
// does make credential theft a single GET instead of requiring a swap
// dance, so callers should treat the return as sensitive.
type CredentialsView struct {
	Provider     string `json:"provider"`
	ConfigDir    string `json:"config_dir"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresAt    int64  `json:"expiresAt"`
	ExpiresAtISO string `json:"expiresAtIso,omitempty"`
}

// handleAccountCredentials returns the keychain envelope for a single
// account, identified by `ident` (name or config_dir) in the query
// string. Anthropic only — Codex stores tokens in plaintext auth.json,
// not in the OS keychain, and the user asked specifically for the
// `claude auth login` envelope shape.
func (s *Server) handleAccountCredentials(w http.ResponseWriter, r *http.Request) {
	ident := r.URL.Query().Get("ident")
	if ident == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ident query param is required"})
		return
	}
	s.mu.RLock()
	snap := s.snap
	s.mu.RUnlock()
	if snap == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "no snapshot yet"})
		return
	}
	var configDir, provider string
	for _, a := range snap.Accounts {
		if a.Name == ident || a.ConfigDir == ident {
			configDir = a.ConfigDir
			provider = a.Provider
			break
		}
	}
	if configDir == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "account not found: " + ident})
		return
	}
	if provider == string(account.ProviderOpenAI) {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "credentials view is Anthropic-only; Codex stores tokens in auth.json",
		})
		return
	}
	creds, err := keychain.LoadCredentials(configDir)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	view := CredentialsView{
		Provider:     provider,
		ConfigDir:    configDir,
		AccessToken:  creds.AccessToken,
		RefreshToken: creds.RefreshToken,
		ExpiresAt:    creds.ExpiresAt,
	}
	if creds.ExpiresAt > 0 {
		view.ExpiresAtISO = time.UnixMilli(creds.ExpiresAt).UTC().Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, view)
}
