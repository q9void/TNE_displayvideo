// Package usersync provides user ID synchronization for bidders
package usersync

import (
	"fmt"
	"net/url"
	"strings"
)

// SyncType represents the type of user sync
type SyncType string

const (
	// SyncTypeIframe uses an iframe to sync
	SyncTypeIframe SyncType = "iframe"
	// SyncTypeRedirect uses a redirect/pixel to sync
	SyncTypeRedirect SyncType = "redirect"
)

// SyncerConfig holds the sync configuration for a bidder
type SyncerConfig struct {
	// BidderCode is the bidder identifier
	BidderCode string
	// IframeSyncURL is the URL template for iframe syncs.
	// Supported placeholders: {{gdpr}}, {{gdpr_consent}}, {{us_privacy}},
	// {{redirect_url}}, {{redirect_url_base}}
	IframeSyncURL string
	// RedirectSyncURL is the URL template for redirect syncs.
	// Supported placeholders: {{gdpr}}, {{gdpr_consent}}, {{us_privacy}},
	// {{redirect_url}}, {{redirect_url_base}}
	RedirectSyncURL string
	// UserMacro is the UID placeholder the SSP substitutes in the redirect URL.
	// Defaults to "$UID" when empty. Use "#PMUID" for PubMatic, "{storage_id}" for Magnite.
	// The macro is embedded in the callback URL (via {{redirect_url}}) and gets
	// URL-encoded when passed to the SSP, so the SSP must substitute it before
	// redirecting. Macros containing "#" (e.g. #PMUID) MUST use {{redirect_url}}
	// so the "#" is percent-encoded (%23) inside the query parameter and not
	// interpreted as a URL fragment by the browser.
	UserMacro string
	// RedirectURLSuffix is an optional query string appended to the redirect callback
	// URL after the uid= macro, before the whole URL is URL-encoded into redir=.
	// Supports {{gdpr_consent}} and {{us_privacy}} placeholders (substituted with
	// actual values from the current sync request).
	// Example: "&gdpr_consent={{gdpr_consent}}&us_privacy={{us_privacy}}"
	RedirectURLSuffix string
	// SupportCORS indicates if the bidder supports CORS for the sync
	SupportCORS bool
	// Enabled indicates if syncing is enabled for this bidder
	Enabled bool
}

// Syncer handles user sync URL generation for a bidder
type Syncer struct {
	config  SyncerConfig
	hostURL string // The PBS host URL for callbacks
}

// NewSyncer creates a new syncer for a bidder
func NewSyncer(config SyncerConfig, hostURL string) *Syncer {
	return &Syncer{
		config:  config,
		hostURL: strings.TrimSuffix(hostURL, "/"),
	}
}

// SyncInfo contains the sync URL and type for a bidder
type SyncInfo struct {
	URL    string   `json:"url"`
	Type   SyncType `json:"type"`
	Bidder string   `json:"bidder"`
}

// GetSync returns the sync info for this bidder.
// fpid is the publisher first-party ID; when non-empty it is appended to the
// setuid callback URL so the handler can link the bidder UID to the right user
// without relying on the server-side uids cookie (which is third-party to the
// publisher's domain and blocked by ITP/ETP).
func (s *Syncer) GetSync(syncType SyncType, gdpr string, consent string, usPrivacy string, fpid string) (*SyncInfo, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("syncing disabled for %s", s.config.BidderCode)
	}

	var urlTemplate string
	switch syncType {
	case SyncTypeIframe:
		urlTemplate = s.config.IframeSyncURL
	case SyncTypeRedirect:
		urlTemplate = s.config.RedirectSyncURL
	default:
		// Prefer redirect, fall back to iframe
		if s.config.RedirectSyncURL != "" {
			urlTemplate = s.config.RedirectSyncURL
			syncType = SyncTypeRedirect
		} else if s.config.IframeSyncURL != "" {
			urlTemplate = s.config.IframeSyncURL
			syncType = SyncTypeIframe
		} else {
			return nil, fmt.Errorf("no sync URL configured for %s", s.config.BidderCode)
		}
	}

	if urlTemplate == "" {
		return nil, fmt.Errorf("no %s sync URL for %s", syncType, s.config.BidderCode)
	}

	// Determine the UID macro for this SSP (default: $UID).
	macro := s.config.UserMacro
	if macro == "" {
		macro = "$UID"
	}

	// Build the redirect URLs (where the bidder will send the UID back to us).
	// The FPID is embedded directly in the callback URL so /setuid can link the
	// bidder UID to the right user without reading the server-side uids cookie
	// (which is third-party to the publisher's domain and blocked by ITP/ETP).
	//
	// {{redirect_url}} — full callback URL with the SSP's UID macro embedded; the
	// entire URL is URL-encoded when substituted.  The SSP substitutes its macro
	// (e.g. $UID or %23PMUID after decoding) with the real user ID before redirecting.
	// Use this for most SSPs and for macros whose characters are safe in URLs ($UID),
	// AND for macros whose characters must be percent-encoded (#PMUID → %23PMUID)
	// to avoid being stripped as a URL fragment by the browser.
	//
	// {{redirect_url_base}} — same callback URL but WITHOUT the macro (ends at "uid=").
	// Use for SSPs (e.g. Sovrn) that append their UID macro AFTER the URL-encoded
	// redirect string: redir={{redirect_url_base}}$UID
	// The macro stays as a literal suffix outside the encoding, so the SSP can find and
	// substitute it in the raw URL string.
	redirectURL := fmt.Sprintf("%s/setuid?bidder=%s&uid=%s", s.hostURL, url.QueryEscape(s.config.BidderCode), macro)
	if fpid != "" {
		redirectURL += "&fpid=" + url.QueryEscape(fpid)
	}
	if s.config.RedirectURLSuffix != "" {
		suffix := s.config.RedirectURLSuffix
		suffix = strings.ReplaceAll(suffix, "{{gdpr_consent}}", url.QueryEscape(consent))
		suffix = strings.ReplaceAll(suffix, "{{us_privacy}}", url.QueryEscape(usPrivacy))
		redirectURL += suffix
	}
	redirectURLBase := fmt.Sprintf("%s/setuid?bidder=%s&uid=", s.hostURL, url.QueryEscape(s.config.BidderCode))

	// Replace placeholders
	syncURL := urlTemplate
	syncURL = strings.ReplaceAll(syncURL, "{{gdpr}}", gdpr)
	syncURL = strings.ReplaceAll(syncURL, "{{gdpr_consent}}", url.QueryEscape(consent))
	syncURL = strings.ReplaceAll(syncURL, "{{us_privacy}}", url.QueryEscape(usPrivacy))
	syncURL = strings.ReplaceAll(syncURL, "{{redirect_url}}", url.QueryEscape(redirectURL))
	syncURL = strings.ReplaceAll(syncURL, "{{redirect_url_base}}", url.QueryEscape(redirectURLBase))

	return &SyncInfo{
		URL:    syncURL,
		Type:   syncType,
		Bidder: s.config.BidderCode,
	}, nil
}

// BidderCode returns the bidder code
func (s *Syncer) BidderCode() string {
	return s.config.BidderCode
}

// IsEnabled returns true if syncing is enabled
func (s *Syncer) IsEnabled() bool {
	return s.config.Enabled
}

// DefaultSyncerConfigs returns the default syncer configurations for common bidders
// These URLs are from the official Prebid documentation
func DefaultSyncerConfigs() map[string]SyncerConfig {
	return map[string]SyncerConfig{
		"appnexus": {
			BidderCode:      "appnexus",
			RedirectSyncURL: "https://ib.adnxs.com/getuid?{{redirect_url}}",
			SupportCORS:     true,
			Enabled:         true,
		},
		// Magnite confirmed our redirect URL is registered with UID macro {storage_id}:
		//   https://ads.thenexusengine.com/setuid?bidder=rubicon&uid={storage_id}&gdpr_consent={{.GDPRConsent}}&us_privacy={{.USPrivacy}}
		// XAPI credentials: pb_thenexusengine (set via RUBICON_XAPI_USER/PASS env vars)
		"rubicon": {
			BidderCode:        "rubicon",
			RedirectSyncURL:   "https://pixel.rubiconproject.com/exchange/sync.php?p=prebid&gdpr={{gdpr}}&gdpr_consent={{gdpr_consent}}&us_privacy={{us_privacy}}&redir={{redirect_url}}",
			IframeSyncURL:     "https://eus.rubiconproject.com/usync.html?p=prebid&gdpr={{gdpr}}&gdpr_consent={{gdpr_consent}}&us_privacy={{us_privacy}}",
			UserMacro:         "{storage_id}",
			RedirectURLSuffix: "&gdpr_consent={{gdpr_consent}}&us_privacy={{us_privacy}}",
			SupportCORS:       true,
			Enabled:           true,
		},
		// NOTE: PubMatic's UID macro is #PMUID (not $UID).  The '#' character is a URL
		// fragment delimiter and would be stripped by the browser if placed outside a
		// percent-encoded query parameter.  We use {{redirect_url}} (which URL-encodes
		// the full callback including the macro) so '#' is encoded as '%23' inside the
		// pu= value.  PubMatic's server decodes the pu parameter, finds '#PMUID', and
		// substitutes the real user ID before issuing the redirect.
		//
		// p=159706 is the PBS partner ID recognised by PubMatic for any Prebid Server host.
		//
		// For the iframe sync, PubMatic's JS directly appends the UID to the predirect
		// URL, so {{redirect_url_base}} (ends at uid=) is correct — no explicit macro needed.
		"pubmatic": {
			BidderCode:      "pubmatic",
			UserMacro:       "#PMUID",
			RedirectSyncURL: "https://image8.pubmatic.com/AdServer/ImgSync?p=159706&gdpr={{gdpr}}&gdpr_consent={{gdpr_consent}}&us_privacy={{us_privacy}}&pu={{redirect_url}}",
			IframeSyncURL:   "https://ads.pubmatic.com/AdServer/js/user_sync.html?gdpr={{gdpr}}&gdpr_consent={{gdpr_consent}}&us_privacy={{us_privacy}}&predirect={{redirect_url_base}}",
			SupportCORS:     true,
			Enabled:         true,
		},
		"openx": {
			BidderCode:      "openx",
			RedirectSyncURL: "https://rtb.openx.net/sync/prebid?gdpr={{gdpr}}&gdpr_consent={{gdpr_consent}}&r={{redirect_url}}",
			SupportCORS:     true,
			Enabled:         true,
		},
		// NOTE: TripleLift requires redirect domains to be allowlisted before syncing works.
		// Without allowlisting, syncs return 400 with X-Error: "Unallowed sync domain".
		// Action: Email prebid@triplelift.com to allowlist ads.thenexusengine.com.
		// No code change needed once allowlisted — sync will work as-is.
		"triplelift": {
			BidderCode:      "triplelift",
			RedirectSyncURL: "https://eb2.3lift.com/sync?gdpr={{gdpr}}&cmp_cs={{gdpr_consent}}&us_privacy={{us_privacy}}&redir={{redirect_url}}",
			SupportCORS:     true,
			Enabled:         true,
		},
		"ix": {
			BidderCode:      "ix",
			RedirectSyncURL: "https://ssum.casalemedia.com/usermatchredir?s=194962&gdpr={{gdpr}}&gdpr_consent={{gdpr_consent}}&us_privacy={{us_privacy}}&cb={{redirect_url}}",
			SupportCORS:     true,
			Enabled:         true,
		},
		"criteo": {
			BidderCode:      "criteo",
			RedirectSyncURL: "https://gum.criteo.com/syncframe?origin=prebidserver&gdpr={{gdpr}}&gdpr_consent={{gdpr_consent}}&us_privacy={{us_privacy}}#{{redirect_url}}",
			SupportCORS:     true,
			Enabled:         true,
		},
		"sharethrough": {
			BidderCode:      "sharethrough",
			RedirectSyncURL: "https://match.sharethrough.com/FGMrCMMc/v1?redirectUri={{redirect_url}}",
			SupportCORS:     true,
			Enabled:         true,
		},
		// NOTE: Sovrn appends their UID macro AFTER the URL-encoded redirect string.
		// Using {{redirect_url}} would encode $UID as %24UID, which Sovrn cannot substitute.
		// {{redirect_url_base}} ends at "uid=" (no $UID), so $UID remains a literal suffix
		// that Sovrn substitutes correctly before returning the redirect to the browser.
		"sovrn": {
			BidderCode:      "sovrn",
			RedirectSyncURL: "https://ap.lijit.com/pixel?redir={{redirect_url_base}}$UID",
			SupportCORS:     true,
			Enabled:         true,
		},
		// NOTE: Kargo is an invitation-only marketplace. Confirm with Kargo account manager
		// that cookie sync is enabled and configured for ads.thenexusengine.com.
		"kargo": {
			BidderCode:      "kargo",
			RedirectSyncURL: "https://crb.kargo.com/api/v1/dsync/PrebidServer?gdpr={{gdpr}}&gdpr_consent={{gdpr_consent}}&us_privacy={{us_privacy}}&r={{redirect_url}}",
			SupportCORS:     true,
			Enabled:         true,
		},
		"33across": {
			BidderCode:      "33across",
			RedirectSyncURL: "https://ssc.33across.com/ps/?gdpr={{gdpr}}&gdpr_consent={{gdpr_consent}}&us_privacy={{us_privacy}}&redir={{redirect_url}}",
			SupportCORS:     true,
			Enabled:         true,
		},
		"gumgum": {
			BidderCode:      "gumgum",
			RedirectSyncURL: "https://rtb.gumgum.com/usync/prbds2s?gdpr={{gdpr}}&gdpr_consent={{gdpr_consent}}&us_privacy={{us_privacy}}&r={{redirect_url}}",
			SupportCORS:     true,
			Enabled:         true,
		},
		"medianet": {
			BidderCode:      "medianet",
			RedirectSyncURL: "https://csync.media.net/csync.php?gdpr={{gdpr}}&gdpr_consent={{gdpr_consent}}&us_privacy={{us_privacy}}&rurl={{redirect_url}}",
			SupportCORS:     true,
			Enabled:         true,
		},
	}
}
