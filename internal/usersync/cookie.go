// Package usersync provides user ID synchronization for bidders
package usersync

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"sync"
	"sort"
	"time"
)

// UID represents a single user ID for a bidder
type UID struct {
	UID     string    `json:"uid"`
	Expires time.Time `json:"expires"`
}

// ConsentData holds TCF consent information
type ConsentData struct {
	TCFString string `json:"tcf_string,omitempty"` // IAB TCF consent string
	Purposes  []int  `json:"purposes,omitempty"`   // Consented purpose IDs [1,2,3,4,7,10]
	GDPRApplies *bool `json:"gdpr_applies,omitempty"` // nil=unknown, true=GDPR applies, false=doesn't apply
	Updated   time.Time `json:"updated,omitempty"`  // When consent was last updated
}

// Cookie holds all bidder user IDs and consent data
type Cookie struct {
	FPID    string         `json:"fpid,omitempty"`    // First-party identifier
	UIDs    map[string]UID `json:"uids"`
	Consent *ConsentData   `json:"consent,omitempty"` // TCF consent data
	OptOut  bool           `json:"optout,omitempty"`
	Created time.Time      `json:"created"`
	mu      sync.RWMutex
}

const (
	// CookieName is the name of the user sync cookie
	CookieName = "uids"
	// DefaultTTL is the default cookie TTL (90 days)
	DefaultTTL = 90 * 24 * time.Hour
	// MaxCookieSize is the maximum cookie size in bytes
	MaxCookieSize = 4000
)

// NewCookie creates a new empty cookie
func NewCookie() *Cookie {
	return &Cookie{
		UIDs:    make(map[string]UID),
		Created: time.Now().UTC(),
	}
}

// ParseCookie parses a cookie from an HTTP request
func ParseCookie(r *http.Request) *Cookie {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return NewCookie()
	}

	// Decode base64
	decoded, err := base64.URLEncoding.DecodeString(cookie.Value)
	if err != nil {
		return NewCookie()
	}

	var c Cookie
	if err := json.Unmarshal(decoded, &c); err != nil {
		return NewCookie()
	}

	if c.UIDs == nil {
		c.UIDs = make(map[string]UID)
	}

	// Clean expired UIDs
	c.cleanExpired()

	return &c
}

// GetUID returns the UID for a bidder, or empty string if not found/expired
func (c *Cookie) GetUID(bidderCode string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	uid, ok := c.UIDs[bidderCode]
	if !ok {
		return ""
	}

	if time.Now().After(uid.Expires) {
		return ""
	}

	return uid.UID
}

// SetUID sets the UID for a bidder
func (c *Cookie) SetUID(bidderCode, uid string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.UIDs[bidderCode] = UID{
		UID:     uid,
		Expires: time.Now().Add(DefaultTTL),
	}
}

// DeleteUID removes the UID for a bidder
func (c *Cookie) DeleteUID(bidderCode string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.UIDs, bidderCode)
}

// HasUID returns true if a valid UID exists for the bidder
func (c *Cookie) HasUID(bidderCode string) bool {
	return c.GetUID(bidderCode) != ""
}

// GetFPID returns the first-party identifier
func (c *Cookie) GetFPID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.FPID
}

// SetFPID sets the first-party identifier
func (c *Cookie) SetFPID(fpid string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.FPID = fpid
}

// HasFPID returns true if FPID is set
func (c *Cookie) HasFPID() bool {
	return c.GetFPID() != ""
}

// SyncCount returns the number of synced bidders
func (c *Cookie) SyncCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	count := 0
	now := time.Now()
	for _, uid := range c.UIDs {
		if now.Before(uid.Expires) {
			count++
		}
	}
	return count
}

// SetOptOut marks the user as opted out
func (c *Cookie) SetOptOut(optOut bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.OptOut = optOut
	if optOut {
		c.FPID = ""
		c.UIDs = make(map[string]UID)
	}
}

// IsOptOut returns true if user has opted out
func (c *Cookie) IsOptOut() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.OptOut
}

// cleanExpired removes expired UIDs
func (c *Cookie) cleanExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for bidder, uid := range c.UIDs {
		if now.After(uid.Expires) {
			delete(c.UIDs, bidder)
		}
	}
}

// ToHTTPCookie converts to an http.Cookie for setting in response
// Note: Uses Lock() instead of RLock() because trimToFit() may modify c.UIDs
func (c *Cookie) ToHTTPCookie(domain string) (*http.Cookie, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}

	encoded := base64.URLEncoding.EncodeToString(data)

	// Check size limit
	if len(encoded) > MaxCookieSize {
		// Trim oldest UIDs to fit
		c.trimToFit()
		if data, err := json.Marshal(c); err == nil {
			encoded = base64.URLEncoding.EncodeToString(data)
		}
	}

	return &http.Cookie{
		Name:     CookieName,
		Value:    encoded,
		Path:     "/",
		Domain:   domain,
		Expires:  time.Now().Add(DefaultTTL),
		MaxAge:   int(DefaultTTL.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
	}, nil
}

// trimToFit removes oldest UIDs to fit within cookie size limit
// Optimized: Uses binary search approach instead of O(n²) marshaling loop
func (c *Cookie) trimToFit() {
	if len(c.UIDs) == 0 {
		return
	}

	// Quick check: marshal once to see if we need to trim at all
	data, err := json.Marshal(c)
	if err != nil {
		return // Can't check size if marshal fails
	}
	encoded := base64.URLEncoding.EncodeToString(data)
	if len(encoded) <= MaxCookieSize {
		return // Already fits
	}

	// Build sorted list of UIDs by expiry time (oldest first)
	type uidEntry struct {
		bidder  string
		expires time.Time
	}
	
	uidList := make([]uidEntry, 0, len(c.UIDs))
	for bidder, uid := range c.UIDs {
		uidList = append(uidList, uidEntry{bidder: bidder, expires: uid.Expires})
	}

	// Sort by expiry time (oldest first)
	sort.Slice(uidList, func(i, j int) bool {
		return uidList[i].expires.Before(uidList[j].expires)
	})

	// Binary search: find minimum number of UIDs to remove
	// This reduces O(n²) to O(n log n) by avoiding repeated marshaling
	left, right := 0, len(uidList)
	
	for left < right {
		mid := (left + right) / 2
		
		// Try removing first 'mid' UIDs (oldest)
		testUIDs := make(map[string]UID, len(c.UIDs)-mid)
		for i := mid; i < len(uidList); i++ {
			testUIDs[uidList[i].bidder] = c.UIDs[uidList[i].bidder]
		}
		
		// Check if this fits
		testCookie := &Cookie{
			UIDs:    testUIDs,
			OptOut:  c.OptOut,
			Created: c.Created,
		}
		
		testData, err := json.Marshal(testCookie)
		if err != nil {
			// If marshal fails, remove more
			left = mid + 1
			continue
		}
		
		testEncoded := base64.URLEncoding.EncodeToString(testData)
		if len(testEncoded) <= MaxCookieSize {
			// Fits! Try removing fewer UIDs
			right = mid
		} else {
			// Still too big, need to remove more
			left = mid + 1
		}
	}
	
	// Remove the minimum required UIDs
	for i := 0; i < left && i < len(uidList); i++ {
		delete(c.UIDs, uidList[i].bidder)
	}
}

// GetAllUIDs returns a copy of all valid UIDs
func (c *Cookie) GetAllUIDs() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]string)
	now := time.Now()
	for bidder, uid := range c.UIDs {
		if now.Before(uid.Expires) {
			result[bidder] = uid.UID
		}
	}
	return result
}

// SetConsent sets the TCF consent data
func (c *Cookie) SetConsent(tcfString string, purposes []int, gdprApplies *bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Consent = &ConsentData{
		TCFString:   tcfString,
		Purposes:    purposes,
		GDPRApplies: gdprApplies,
		Updated:     time.Now().UTC(),
	}
}

// GetConsent returns the TCF consent data
func (c *Cookie) GetConsent() *ConsentData {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Consent
}

// HasConsent returns true if consent data exists
func (c *Cookie) HasConsent() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Consent != nil && c.Consent.TCFString != ""
}

// HasConsentForPurpose checks if user consented to a specific purpose
func (c *Cookie) HasConsentForPurpose(purpose int) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.Consent == nil || c.Consent.Purposes == nil {
		return false
	}

	for _, p := range c.Consent.Purposes {
		if p == purpose {
			return true
		}
	}
	return false
}
