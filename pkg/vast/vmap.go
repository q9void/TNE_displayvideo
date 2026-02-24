package vast

import (
	"encoding/xml"
	"fmt"
	"net/url"
)

// VMAPNamespace is the IAB VMAP 1.0 XML namespace URI.
const VMAPNamespace = "http://www.iab.net/videosuite/vmap"

// VMAP represents an IAB VMAP 1.0 document.
// SSAI systems fetch this once at stream start; each AdBreak's AdTagURI
// is resolved just-in-time when the player reaches that cue point.
type VMAP struct {
	XMLName  xml.Name  `xml:"vmap:VMAP"`
	XMLNS    string    `xml:"xmlns:vmap,attr"`
	Version  string    `xml:"version,attr"`
	AdBreaks []AdBreak `xml:"vmap:AdBreak"`
}

// AdBreak represents a single ad insertion point in the content timeline.
type AdBreak struct {
	// TimeOffset is one of: "start", "end", "HH:MM:SS[.mmm]", or "n%"
	TimeOffset     string              `xml:"timeOffset,attr"`
	BreakType      string              `xml:"breakType,attr"`
	BreakID        string              `xml:"breakId,attr,omitempty"`
	AdSource       *VMAPAdSource       `xml:"vmap:AdSource,omitempty"`
	TrackingEvents *VMAPTrackingEvents `xml:"vmap:TrackingEvents,omitempty"`
	Extensions     *VMAPExtensions     `xml:"vmap:Extensions,omitempty"`
}

// BreakType values (IAB VMAP 1.0 §3.1).
const (
	BreakTypeLinear    = "linear"
	BreakTypeNonLinear = "nonlinear"
	BreakTypeDisplay   = "display"
)

// VMAPAdSource defines the source of ads for a break.
// Either AdTagURI (external VAST URL) or VASTAdData (inline VAST) must be set.
type VMAPAdSource struct {
	ID               string          `xml:"id,attr,omitempty"`
	AllowMultipleAds bool            `xml:"allowMultipleAds,attr,omitempty"`
	FollowRedirects  bool            `xml:"followRedirects,attr,omitempty"`
	AdTagURI         *VMAPAdTagURI   `xml:"vmap:AdTagURI,omitempty"`
	VASTAdData       *VMAPVASTAdData `xml:"vmap:VASTAdData,omitempty"`
}

// VMAPAdTagURI is an external VAST tag URL (CDATA to preserve & in query strings).
type VMAPAdTagURI struct {
	TemplateType string `xml:"templateType,attr,omitempty"`
	Value        string `xml:",cdata"`
}

// VMAPVASTAdData embeds an inline VAST document inside a VMAP break.
type VMAPVASTAdData struct {
	VAST *VAST `xml:"VAST,omitempty"`
}

// VMAPTrackingEvents holds break-level tracking pixels.
type VMAPTrackingEvents struct {
	Tracking []VMAPTracking `xml:"vmap:Tracking"`
}

// VMAPTracking is a single break tracking event URL (CDATA).
type VMAPTracking struct {
	Event string `xml:"event,attr"`
	Value string `xml:",cdata"`
}

// Break tracking event names (IAB VMAP 1.0 §3.3).
const (
	VMAPEventBreakStart = "breakStart"
	VMAPEventBreakEnd   = "breakEnd"
	VMAPEventError      = "error"
)

// VMAPExtensions holds vendor-specific VMAP extensions.
type VMAPExtensions struct {
	Extension []VMAPExtension `xml:"vmap:Extension"`
}

// VMAPExtension is a single vendor extension block.
type VMAPExtension struct {
	Type  string `xml:"type,attr,omitempty"`
	Value string `xml:",innerxml"`
}

// NewVMAP creates an empty VMAP 1.0 document.
func NewVMAP() *VMAP {
	return &VMAP{
		XMLNS:    VMAPNamespace,
		Version:  "1.0",
		AdBreaks: []AdBreak{},
	}
}

// AddAdTagBreak appends an ad break whose ads are resolved from an external VAST URL.
// trackingBaseURL is used to generate breakStart/breakEnd pixels; pass "" to omit tracking.
func (v *VMAP) AddAdTagBreak(timeOffset, breakID, adTagURI string, allowMultipleAds bool, trackingBaseURL string) {
	sourceID := fmt.Sprintf("%s-source", breakID)

	ab := AdBreak{
		TimeOffset: timeOffset,
		BreakType:  BreakTypeLinear,
		BreakID:    breakID,
		AdSource: &VMAPAdSource{
			ID:               sourceID,
			AllowMultipleAds: allowMultipleAds,
			FollowRedirects:  true,
			AdTagURI: &VMAPAdTagURI{
				TemplateType: "vast4",
				Value:        adTagURI,
			},
		},
	}

	if trackingBaseURL != "" {
		base := trackingBaseURL + "?event="
		ab.TrackingEvents = &VMAPTrackingEvents{
			Tracking: []VMAPTracking{
				{Event: VMAPEventBreakStart, Value: base + VMAPEventBreakStart + "&break=" + url.QueryEscape(breakID)},
				{Event: VMAPEventBreakEnd, Value: base + VMAPEventBreakEnd + "&break=" + url.QueryEscape(breakID)},
			},
		}
	}

	v.AdBreaks = append(v.AdBreaks, ab)
}

// Marshal serializes the VMAP document to XML with an XML declaration header.
func (v *VMAP) Marshal() ([]byte, error) {
	data, err := xml.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal VMAP: %w", err)
	}
	return append([]byte(xml.Header), data...), nil
}
