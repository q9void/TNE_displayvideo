package vast

import (
	"fmt"
	"net/url"
	"strings"
)

// ValidationError represents a VAST validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationResult contains validation results
type ValidationResult struct {
	Valid  bool
	Errors []ValidationError
}

// AddError adds a validation error
func (vr *ValidationResult) AddError(field, message string) {
	vr.Valid = false
	vr.Errors = append(vr.Errors, ValidationError{
		Field:   field,
		Message: message,
	})
}

// Validate validates a VAST document
func (v *VAST) Validate() *ValidationResult {
	result := &ValidationResult{Valid: true}

	// Validate version
	if v.Version == "" {
		result.AddError("VAST.version", "version attribute is required")
	} else if !isValidVersion(v.Version) {
		result.AddError("VAST.version", fmt.Sprintf("unsupported version: %s", v.Version))
	}

	// Validate ads
	if len(v.Ads) == 0 && v.Error == "" {
		// Empty VAST is valid if it has an Error element
		result.AddError("VAST.Ad", "VAST must contain at least one Ad or an Error element")
	}

	// Validate each ad
	for i, ad := range v.Ads {
		validateAd(&ad, i, result)
	}

	return result
}

// validateAd validates an Ad element
func validateAd(ad *Ad, index int, result *ValidationResult) {
	prefix := fmt.Sprintf("VAST.Ad[%d]", index)

	// Must have either InLine or Wrapper
	if ad.InLine == nil && ad.Wrapper == nil {
		result.AddError(prefix, "Ad must contain either InLine or Wrapper")
		return
	}

	if ad.InLine != nil && ad.Wrapper != nil {
		result.AddError(prefix, "Ad cannot contain both InLine and Wrapper")
		return
	}

	// Validate InLine
	if ad.InLine != nil {
		validateInLine(ad.InLine, prefix, result)
	}

	// Validate Wrapper
	if ad.Wrapper != nil {
		validateWrapper(ad.Wrapper, prefix, result)
	}
}

// validateInLine validates an InLine element
func validateInLine(inline *InLine, prefix string, result *ValidationResult) {
	prefix = prefix + ".InLine"

	// Required fields
	if inline.AdSystem.Value == "" {
		result.AddError(prefix+".AdSystem", "AdSystem is required")
	}

	if inline.AdTitle == "" {
		result.AddError(prefix+".AdTitle", "AdTitle is required")
	}

	if len(inline.Impressions) == 0 {
		result.AddError(prefix+".Impression", "At least one Impression is required")
	}

	// Validate impressions
	for i, imp := range inline.Impressions {
		if imp.Value == "" {
			result.AddError(fmt.Sprintf("%s.Impression[%d]", prefix, i), "Impression URL is required")
		} else if !isValidURL(imp.Value) {
			result.AddError(fmt.Sprintf("%s.Impression[%d]", prefix, i), "Invalid Impression URL")
		}
	}

	// Validate error URL if present
	if inline.Error != "" && !isValidURL(inline.Error) {
		result.AddError(prefix+".Error", "Invalid Error URL")
	}

	// Validate creatives
	if len(inline.Creatives.Creative) == 0 {
		result.AddError(prefix+".Creatives", "At least one Creative is required")
	}

	for i, creative := range inline.Creatives.Creative {
		validateCreative(&creative, fmt.Sprintf("%s.Creative[%d]", prefix, i), result)
	}
}

// validateWrapper validates a Wrapper element
func validateWrapper(wrapper *Wrapper, prefix string, result *ValidationResult) {
	prefix = prefix + ".Wrapper"

	// Required fields
	if wrapper.AdSystem.Value == "" {
		result.AddError(prefix+".AdSystem", "AdSystem is required")
	}

	if wrapper.VASTAdTagURI.Value == "" {
		result.AddError(prefix+".VASTAdTagURI", "VASTAdTagURI is required")
	} else if !isValidURL(wrapper.VASTAdTagURI.Value) {
		result.AddError(prefix+".VASTAdTagURI", "Invalid VAST Ad Tag URI")
	}

	// Validate impressions
	for i, imp := range wrapper.Impressions {
		if imp.Value == "" {
			result.AddError(fmt.Sprintf("%s.Impression[%d]", prefix, i), "Impression URL is required")
		} else if !isValidURL(imp.Value) {
			result.AddError(fmt.Sprintf("%s.Impression[%d]", prefix, i), "Invalid Impression URL")
		}
	}
}

// validateCreative validates a Creative element
func validateCreative(creative *Creative, prefix string, result *ValidationResult) {
	// Must have at least one creative type
	if creative.Linear == nil && creative.NonLinearAds == nil && creative.CompanionAds == nil {
		result.AddError(prefix, "Creative must contain Linear, NonLinearAds, or CompanionAds")
		return
	}

	// Validate Linear
	if creative.Linear != nil {
		validateLinear(creative.Linear, prefix+".Linear", result)
	}

	// Validate CompanionAds
	if creative.CompanionAds != nil {
		validateCompanionAds(creative.CompanionAds, prefix+".CompanionAds", result)
	}
}

// validateLinear validates a Linear creative
func validateLinear(linear *Linear, prefix string, result *ValidationResult) {
	// Required fields
	if linear.Duration == "" {
		result.AddError(prefix+".Duration", "Duration is required")
	} else {
		// Validate duration format
		if _, err := ParseDuration(string(linear.Duration)); err != nil {
			result.AddError(prefix+".Duration", "Invalid duration format (expected HH:MM:SS)")
		}
	}

	// Must have at least one media file
	if len(linear.MediaFiles.MediaFile) == 0 {
		result.AddError(prefix+".MediaFiles", "At least one MediaFile is required")
	}

	// Validate media files
	for i, mf := range linear.MediaFiles.MediaFile {
		validateMediaFile(&mf, fmt.Sprintf("%s.MediaFile[%d]", prefix, i), result)
	}

	// Validate tracking events
	for i, tracking := range linear.TrackingEvents.Tracking {
		validateTracking(&tracking, fmt.Sprintf("%s.Tracking[%d]", prefix, i), result)
	}

	// Validate video clicks
	if linear.VideoClicks != nil {
		if linear.VideoClicks.ClickThrough != nil {
			if linear.VideoClicks.ClickThrough.Value != "" && !isValidURL(linear.VideoClicks.ClickThrough.Value) {
				result.AddError(prefix+".VideoClicks.ClickThrough", "Invalid ClickThrough URL")
			}
		}
	}
}

// validateMediaFile validates a MediaFile element
func validateMediaFile(mf *MediaFile, prefix string, result *ValidationResult) {
	// Required fields
	if mf.Delivery == "" {
		result.AddError(prefix+".delivery", "delivery attribute is required")
	} else if mf.Delivery != "progressive" && mf.Delivery != "streaming" {
		result.AddError(prefix+".delivery", "delivery must be 'progressive' or 'streaming'")
	}

	if mf.Type == "" {
		result.AddError(prefix+".type", "type attribute is required")
	} else if !isValidMIMEType(mf.Type) {
		result.AddError(prefix+".type", "Invalid MIME type")
	}

	if mf.Width <= 0 {
		result.AddError(prefix+".width", "width must be greater than 0")
	}

	if mf.Height <= 0 {
		result.AddError(prefix+".height", "height must be greater than 0")
	}

	if mf.Value == "" {
		result.AddError(prefix, "MediaFile URL is required")
	} else if !isValidURL(mf.Value) {
		result.AddError(prefix, "Invalid MediaFile URL")
	}
}

// validateTracking validates a Tracking event
func validateTracking(tracking *Tracking, prefix string, result *ValidationResult) {
	if tracking.Event == "" {
		result.AddError(prefix+".event", "event attribute is required")
	} else if !isValidEventType(tracking.Event) {
		result.AddError(prefix+".event", fmt.Sprintf("Unknown event type: %s", tracking.Event))
	}

	if tracking.Value == "" {
		result.AddError(prefix, "Tracking URL is required")
	} else if !isValidURL(tracking.Value) {
		result.AddError(prefix, "Invalid Tracking URL")
	}
}

// validateCompanionAds validates CompanionAds
func validateCompanionAds(companionAds *CompanionAds, prefix string, result *ValidationResult) {
	if len(companionAds.Companion) == 0 {
		result.AddError(prefix, "At least one Companion is required")
	}

	for i, companion := range companionAds.Companion {
		validateCompanion(&companion, fmt.Sprintf("%s.Companion[%d]", prefix, i), result)
	}
}

// validateCompanion validates a Companion ad
func validateCompanion(companion *Companion, prefix string, result *ValidationResult) {
	if companion.Width <= 0 {
		result.AddError(prefix+".width", "width must be greater than 0")
	}

	if companion.Height <= 0 {
		result.AddError(prefix+".height", "height must be greater than 0")
	}

	// Must have at least one resource
	if companion.StaticResource == nil && companion.IFrameResource == "" && companion.HTMLResource == nil {
		result.AddError(prefix, "Companion must have StaticResource, IFrameResource, or HTMLResource")
	}

	// Validate static resource
	if companion.StaticResource != nil && companion.StaticResource.Value != "" {
		if !isValidURL(companion.StaticResource.Value) {
			result.AddError(prefix+".StaticResource", "Invalid resource URL")
		}
	}
}

// Validation helper functions

func isValidVersion(version string) bool {
	validVersions := map[string]bool{
		"1.0": true,
		"2.0": true,
		"3.0": true,
		"4.0": true,
		"4.1": true,
		"4.2": true,
	}
	return validVersions[version]
}

func isValidURL(urlStr string) bool {
	// Allow URLs with macros like ${AUCTION_PRICE} and [ERRORCODE]
	// These are valid in VAST even though they're not valid URLs until macro replacement
	if strings.Contains(urlStr, "${") || strings.Contains(urlStr, "[") {
		return true
	}

	u, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	// Must have scheme (http or https)
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}

	return true
}

func isValidMIMEType(mimeType string) bool {
	validTypes := []string{
		"video/mp4",
		"video/webm",
		"video/ogg",
		"video/3gpp",
		"video/x-flv",
		"video/x-ms-wmv",
		"application/x-shockwave-flash",
		"application/javascript", // For VPAID
	}

	for _, valid := range validTypes {
		if mimeType == valid {
			return true
		}
	}

	// Also accept any video/* type
	return strings.HasPrefix(mimeType, "video/")
}

func isValidEventType(event string) bool {
	validEvents := map[string]bool{
		EventCreativeView:     true,
		EventStart:            true,
		EventFirstQuartile:    true,
		EventMidpoint:         true,
		EventThirdQuartile:    true,
		EventComplete:         true,
		EventMute:             true,
		EventUnmute:           true,
		EventPause:            true,
		EventRewind:           true,
		EventResume:           true,
		EventFullscreen:       true,
		EventExitFullscreen:   true,
		EventExpand:           true,
		EventCollapse:         true,
		EventAcceptInvitation: true,
		EventClose:            true,
		EventSkip:             true,
		EventProgress:         true,
		EventClick:            true,
	}
	return validEvents[event]
}
