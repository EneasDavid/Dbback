package infrastructure

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// CredentialProvider identifies the provider of a WebAuthn credential
type CredentialProvider string

const (
	ProviderApple   CredentialProvider = "apple"
	ProviderGoogle  CredentialProvider = "google"
	ProviderGeneric CredentialProvider = "webauthn"
	ProviderUnknown CredentialProvider = "unknown"
)

// Credential represents a saved WebAuthn credential
type Credential struct {
	ID          string             // Unique credential ID
	Provider    CredentialProvider // apple, google, or webauthn
	PublicKeyID string             // Hashed public key for comparision
	PublicKey   []byte             // Encoded public key
	Counter     uint32             // Attack prevention counter
	CreatedAt   time.Time          // When credential was first used
	LastUsedAt  time.Time          // When credential was last authenticated
	Nickname    string             // User-friendly name
	IsDefault   bool               // Default credential for this device
	UserSaved   bool               // User explicitly saved this credential
	DeviceInfo  string             // Device identifier (browser, OS)
}

// CredentialManager manages WebAuthn credentials with smart saving logic
type CredentialManager struct {
	store      map[string][]*Credential // userId -> credentials
	mu         sync.RWMutex
	sessionKey string // For idempotency checks
}

// NewCredentialManager creates a new manager
func NewCredentialManager() *CredentialManager {
	return &CredentialManager{
		store: make(map[string][]*Credential),
	}
}

// CredentialInfo returned after authentication
type CredentialInfo struct {
	ID          string             // Credential ID
	Provider    CredentialProvider // Detected provider
	IsKnown     bool               // Already saved before
	NeedsSaving bool               // Should ask user to save
	SaveReason  string             // Why asking to save
	LastCredID  string             // Last used credential ID (for comparison)
}

// AuthenticateAndAnalyze checks credential after successful authentication
func (cm *CredentialManager) AuthenticateAndAnalyze(
	userID string,
	credentialID string,
	publicKeyHash string,
	provider CredentialProvider,
	deviceInfo string,
) (*CredentialInfo, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	creds := cm.store[userID]
	info := &CredentialInfo{
		ID:       credentialID,
		Provider: provider,
	}

	if len(creds) == 0 {
		// First login ever
		info.IsKnown = false
		info.NeedsSaving = true
		info.SaveReason = "first_login"
		return info, nil
	}

	// Check if this credential is known
	for _, cred := range creds {
		if cred.PublicKeyID == publicKeyHash {
			// Credential exists
			info.IsKnown = true
			info.LastCredID = cred.ID

			if cred.UserSaved {
				// User already saved this one, don't ask again
				info.NeedsSaving = false
				cred.LastUsedAt = time.Now()
				cred.Counter++
				return info, nil
			}

			// Credential exists but user never explicitly saved it
			// Ask again
			info.NeedsSaving = true
			info.SaveReason = "remind_save"
			return info, nil
		}
	}

	// Different credential on same device
	info.IsKnown = false
	info.NeedsSaving = true
	info.SaveReason = "different_credential"

	if len(creds) > 0 {
		info.LastCredID = creds[0].ID
	}

	return info, nil
}

// SaveCredential saves a credential with user consent
func (cm *CredentialManager) SaveCredential(
	userID string,
	credential *Credential,
) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if userID == "" || credential.ID == "" {
		return fmt.Errorf("userID and credential ID required")
	}

	// Check if already exists
	creds := cm.store[userID]
	for _, existing := range creds {
		if existing.PublicKeyID == credential.PublicKeyID {
			// Update existing
			existing.LastUsedAt = time.Now()
			existing.UserSaved = true
			existing.Counter++
			return nil
		}
	}

	// New credential
	credential.CreatedAt = time.Now()
	credential.LastUsedAt = time.Now()
	credential.UserSaved = true

	// Auto-detect provider if not set
	if credential.Provider == ProviderUnknown || credential.Provider == "" {
		credential.Provider = DetectProvider(credential)
	}

	cm.store[userID] = append(creds, credential)
	return nil
}

// SkipSave marks a credential as explicitly skipped by user
func (cm *CredentialManager) SkipSave(userID string, credentialID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	creds, exists := cm.store[userID]
	if !exists {
		return fmt.Errorf("no credentials for user %s", userID)
	}

	for _, cred := range creds {
		if cred.ID == credentialID {
			cred.UserSaved = false
			cred.LastUsedAt = time.Now()
			return nil
		}
	}

	return fmt.Errorf("credential not found")
}

// GetCredentials returns all saved credentials for user
func (cm *CredentialManager) GetCredentials(userID string) []*Credential {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	creds := cm.store[userID]
	result := make([]*Credential, len(creds))
	copy(result, creds)
	return result
}

// GetDefault returns the default credential
func (cm *CredentialManager) GetDefault(userID string) *Credential {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	creds := cm.store[userID]
	for _, cred := range creds {
		if cred.IsDefault {
			return cred
		}
	}

	// Return most recently used
	if len(creds) > 0 {
		return creds[len(creds)-1]
	}
	return nil
}

// SetDefault marks a credential as default
func (cm *CredentialManager) SetDefault(userID string, credentialID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	creds := cm.store[userID]
	found := false

	for _, cred := range creds {
		cred.IsDefault = false
		if cred.ID == credentialID {
			cred.IsDefault = true
			found = true
		}
	}

	if !found {
		return fmt.Errorf("credential not found")
	}
	return nil
}

// DeleteCredential removes a credential
func (cm *CredentialManager) DeleteCredential(userID string, credentialID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	creds := cm.store[userID]
	for i, cred := range creds {
		if cred.ID == credentialID {
			cm.store[userID] = append(creds[:i], creds[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("credential not found")
}

// DetectProvider analyzes credential to determine provider
func DetectProvider(cred *Credential) CredentialProvider {
	// In production, this would analyze:
	// - Attestation format
	// - AAGUID
	// - Extension data
	// - Public key algorithm

	// For now, simple heuristics:
	if len(cred.PublicKey) > 0 {
		// Could check AAGUID for known values
		// Apple: D82C9643-F1F3-4341-8270-A42C46F919F0
		// Google: 00000000-0000-0000-0000-000000000001
		hash := sha256.Sum256(cred.PublicKey)
		hashStr := hex.EncodeToString(hash[:])

		// Simple detection (would be more sophisticated)
		if len(hashStr) > 0 {
			// Return based on patterns
			return ProviderGeneric
		}
	}

	return ProviderUnknown
}

// CredentialSnapshot for audit logging
type CredentialSnapshot struct {
	UserID       string
	CredentialID string
	Provider     CredentialProvider
	Action       string // "authenticated", "saved", "skipped", "deleted"
	Timestamp    time.Time
	DeviceInfo   string
	Success      bool
	ErrorMessage string
}

// AuditLog stores credential events
type AuditLog struct {
	events []*CredentialSnapshot
	mu     sync.RWMutex
	maxLen int
}

// NewAuditLog creates audit log with max entries
func NewAuditLog(maxEntries int) *AuditLog {
	return &AuditLog{
		events: make([]*CredentialSnapshot, 0, maxEntries),
		maxLen: maxEntries,
	}
}

// Log adds an event to audit log
func (al *AuditLog) Log(snapshot *CredentialSnapshot) {
	al.mu.Lock()
	defer al.mu.Unlock()

	snapshot.Timestamp = time.Now()
	al.events = append(al.events, snapshot)

	// Keep only recent events
	if len(al.events) > al.maxLen {
		al.events = al.events[len(al.events)-al.maxLen:]
	}
}

// GetEvents returns recent events
func (al *AuditLog) GetEvents(userID string, limit int) []*CredentialSnapshot {
	al.mu.RLock()
	defer al.mu.RUnlock()

	var result []*CredentialSnapshot
	for i := len(al.events) - 1; i >= 0 && len(result) < limit; i-- {
		if al.events[i].UserID == userID {
			result = append(result, al.events[i])
		}
	}
	return result
}

// Stats returns statistics about credentials
type CredentialStats struct {
	TotalUsers           int
	TotalCredentials     int
	SavedCredentials     int
	ProviderDistribution map[CredentialProvider]int
	AverageSavedPerUser  float64
}

// GetStats returns credential statistics
func (cm *CredentialManager) GetStats() *CredentialStats {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	stats := &CredentialStats{
		TotalUsers:           len(cm.store),
		ProviderDistribution: make(map[CredentialProvider]int),
	}

	totalSaved := 0
	for _, creds := range cm.store {
		stats.TotalCredentials += len(creds)
		for _, cred := range creds {
			stats.ProviderDistribution[cred.Provider]++
			if cred.UserSaved {
				totalSaved++
				stats.SavedCredentials++
			}
		}
	}

	if stats.TotalUsers > 0 {
		stats.AverageSavedPerUser = float64(stats.SavedCredentials) / float64(stats.TotalUsers)
	}

	return stats
}
