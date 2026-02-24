// Package vast provides VAST (Video Ad Serving Template) parsing and generation
// for video ad serving in tnevideo ad exchange.
package vast

import (
	"encoding/xml"
	"fmt"
	"time"
)

// VAST represents the root VAST document
type VAST struct {
	XMLName xml.Name `xml:"VAST"`
	Version string   `xml:"version,attr"`
	Ads     []Ad     `xml:"Ad"`
	Error   string   `xml:"Error,omitempty"`
}

// Ad represents a single ad in VAST
type Ad struct {
	ID       string    `xml:"id,attr,omitempty"`
	Sequence int       `xml:"sequence,attr,omitempty"`
	InLine   *InLine   `xml:"InLine,omitempty"`
	Wrapper  *Wrapper  `xml:"Wrapper,omitempty"`
}

// InLine represents an inline ad
type InLine struct {
	AdSystem    AdSystem     `xml:"AdSystem"`
	AdTitle     string       `xml:"AdTitle"`
	Description string       `xml:"Description,omitempty"`
	Advertiser  string       `xml:"Advertiser,omitempty"`
	Pricing     *Pricing     `xml:"Pricing,omitempty"`
	Survey      string       `xml:"Survey,omitempty"`
	Error       string       `xml:"Error,omitempty"`
	Impressions []Impression `xml:"Impression"`
	Creatives   Creatives    `xml:"Creatives"`
	Extensions  *Extensions  `xml:"Extensions,omitempty"`
}

// CDATAElement wraps a string value in a CDATA section when marshaled to XML.
// Use this for URL fields (VASTAdTagURI, Error) that may contain & characters.
type CDATAElement struct {
	Value string `xml:",cdata"`
}

// Wrapper represents a wrapper ad that references another VAST.
// Fields are ordered per VAST 4.0 spec: AdSystem → Error → Impression → Creatives → VASTAdTagURI.
type Wrapper struct {
	FollowAdditionalWraps bool         `xml:"followAdditionalWrappers,attr,omitempty"`
	AllowMultipleAds      bool         `xml:"allowMultipleAds,attr,omitempty"`
	FallbackOnNoAd        bool         `xml:"fallbackOnNoAd,attr,omitempty"`
	AdSystem              AdSystem     `xml:"AdSystem"`
	Error                 string       `xml:"Error,omitempty"`
	Impressions           []Impression `xml:"Impression"`
	Creatives             Creatives    `xml:"Creatives,omitempty"`
	Extensions            *Extensions  `xml:"Extensions,omitempty"`
	VASTAdTagURI          CDATAElement `xml:"VASTAdTagURI"`
}

// AdSystem identifies the ad server
type AdSystem struct {
	Version string `xml:"version,attr,omitempty"`
	Value   string `xml:",chardata"`
}

// Pricing represents ad pricing information
type Pricing struct {
	Model    string `xml:"model,attr,omitempty"`
	Currency string `xml:"currency,attr,omitempty"`
	Value    string `xml:",chardata"`
}

// Impression represents an impression tracking URL
type Impression struct {
	ID    string `xml:"id,attr,omitempty"`
	Value string `xml:",cdata"`
}

// Creatives contains the creative elements
type Creatives struct {
	Creative []Creative `xml:"Creative"`
}

// Creative represents a single creative
type Creative struct {
	ID               string            `xml:"id,attr,omitempty"`
	AdID             string            `xml:"adId,attr,omitempty"`
	Sequence         int               `xml:"sequence,attr,omitempty"`
	APIFramework     string            `xml:"apiFramework,attr,omitempty"`
	Linear           *Linear           `xml:"Linear,omitempty"`
	NonLinearAds     *NonLinearAds     `xml:"NonLinearAds,omitempty"`
	CompanionAds     *CompanionAds     `xml:"CompanionAds,omitempty"`
	UniversalAdId    *UniversalAdId    `xml:"UniversalAdId,omitempty"`
	CreativeExtensions *CreativeExtensions `xml:"CreativeExtensions,omitempty"`
}

// UniversalAdId represents a universal ad identifier
type UniversalAdId struct {
	IdRegistry string `xml:"idRegistry,attr"`
	IdValue    string `xml:"idValue,attr,omitempty"`
	Value      string `xml:",chardata"`
}

// Linear represents a linear (video) creative
type Linear struct {
	SkipOffset     string         `xml:"skipoffset,attr,omitempty"`
	Duration       Duration       `xml:"Duration,omitempty"`
	AdParameters   *AdParameters  `xml:"AdParameters,omitempty"`
	MediaFiles     *MediaFiles    `xml:"MediaFiles,omitempty"`
	TrackingEvents TrackingEvents `xml:"TrackingEvents,omitempty"`
	VideoClicks    *VideoClicks   `xml:"VideoClicks,omitempty"`
	Icons          *Icons         `xml:"Icons,omitempty"`
}

// Duration represents a time duration in HH:MM:SS format
type Duration string

// ParseDuration parses a VAST duration string to time.Duration
func ParseDuration(s string) (time.Duration, error) {
	var h, m, sec int
	_, err := fmt.Sscanf(s, "%d:%d:%d", &h, &m, &sec)
	if err != nil {
		return 0, fmt.Errorf("invalid duration format: %s", s)
	}
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec)*time.Second, nil
}

// FormatDuration formats a time.Duration to VAST duration string
func FormatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

// AdParameters contains parameters for the ad
type AdParameters struct {
	XMLEncoded bool   `xml:"xmlEncoded,attr,omitempty"`
	Value      string `xml:",cdata"`
}

// MediaFiles contains the media file elements
type MediaFiles struct {
	MediaFile []MediaFile `xml:"MediaFile"`
}

// MediaFile represents a single media file
type MediaFile struct {
	ID                  string `xml:"id,attr,omitempty"`
	Delivery            string `xml:"delivery,attr"`
	Type                string `xml:"type,attr"`
	Bitrate             int    `xml:"bitrate,attr,omitempty"`
	MinBitrate          int    `xml:"minBitrate,attr,omitempty"`
	MaxBitrate          int    `xml:"maxBitrate,attr,omitempty"`
	Width               int    `xml:"width,attr"`
	Height              int    `xml:"height,attr"`
	Scalable            bool   `xml:"scalable,attr,omitempty"`
	MaintainAspectRatio bool   `xml:"maintainAspectRatio,attr,omitempty"`
	Codec               string `xml:"codec,attr,omitempty"`
	APIFramework        string `xml:"apiFramework,attr,omitempty"`
	Value               string `xml:",cdata"`
}

// TrackingEvents contains tracking event elements
type TrackingEvents struct {
	Tracking []Tracking `xml:"Tracking"`
}

// Tracking represents a single tracking event
type Tracking struct {
	Event  string `xml:"event,attr"`
	Offset string `xml:"offset,attr,omitempty"`
	Value  string `xml:",cdata"`
}

// TrackingEvent constants
const (
	EventCreativeView     = "creativeView"
	EventStart            = "start"
	EventFirstQuartile    = "firstQuartile"
	EventMidpoint         = "midpoint"
	EventThirdQuartile    = "thirdQuartile"
	EventComplete         = "complete"
	EventMute             = "mute"
	EventUnmute           = "unmute"
	EventPause            = "pause"
	EventRewind           = "rewind"
	EventResume           = "resume"
	EventFullscreen       = "fullscreen"
	EventExitFullscreen   = "exitFullscreen"
	EventExpand           = "expand"
	EventCollapse         = "collapse"
	EventAcceptInvitation = "acceptInvitation"
	EventClose            = "close"
	EventSkip             = "skip"
	EventProgress         = "progress"
	EventClick            = "click"
)

// VideoClicks contains click tracking elements
type VideoClicks struct {
	ClickThrough  *ClickThrough  `xml:"ClickThrough,omitempty"`
	ClickTracking []ClickTracking `xml:"ClickTracking,omitempty"`
	CustomClick   []CustomClick  `xml:"CustomClick,omitempty"`
}

// ClickThrough represents the click-through URL
type ClickThrough struct {
	ID    string `xml:"id,attr,omitempty"`
	Value string `xml:",cdata"`
}

// ClickTracking represents a click tracking URL
type ClickTracking struct {
	ID    string `xml:"id,attr,omitempty"`
	Value string `xml:",cdata"`
}

// CustomClick represents a custom click URL
type CustomClick struct {
	ID    string `xml:"id,attr,omitempty"`
	Value string `xml:",cdata"`
}

// Icons contains icon elements
type Icons struct {
	Icon []Icon `xml:"Icon"`
}

// Icon represents an icon overlay
type Icon struct {
	Program       string         `xml:"program,attr,omitempty"`
	Width         int            `xml:"width,attr,omitempty"`
	Height        int            `xml:"height,attr,omitempty"`
	XPosition     string         `xml:"xPosition,attr,omitempty"`
	YPosition     string         `xml:"yPosition,attr,omitempty"`
	Duration      string         `xml:"duration,attr,omitempty"`
	Offset        string         `xml:"offset,attr,omitempty"`
	APIFramework  string         `xml:"apiFramework,attr,omitempty"`
	PxRatio       string         `xml:"pxratio,attr,omitempty"`
	StaticResource *StaticResource `xml:"StaticResource,omitempty"`
	IFrameResource string         `xml:"IFrameResource,omitempty"`
	HTMLResource   *HTMLResource  `xml:"HTMLResource,omitempty"`
	IconClicks     *IconClicks    `xml:"IconClicks,omitempty"`
	IconViewTracking []string     `xml:"IconViewTracking,omitempty"`
}

// StaticResource represents a static resource
type StaticResource struct {
	CreativeType string `xml:"creativeType,attr,omitempty"`
	Value        string `xml:",cdata"`
}

// HTMLResource represents an HTML resource
type HTMLResource struct {
	XMLEncoded bool   `xml:"xmlEncoded,attr,omitempty"`
	Value      string `xml:",cdata"`
}

// IconClicks contains icon click elements
type IconClicks struct {
	IconClickThrough  string   `xml:"IconClickThrough,omitempty"`
	IconClickTracking []string `xml:"IconClickTracking,omitempty"`
}

// NonLinearAds contains non-linear ad elements
type NonLinearAds struct {
	TrackingEvents TrackingEvents `xml:"TrackingEvents,omitempty"`
	NonLinear      []NonLinear    `xml:"NonLinear"`
}

// NonLinear represents a non-linear ad (overlay)
type NonLinear struct {
	ID              string          `xml:"id,attr,omitempty"`
	Width           int             `xml:"width,attr"`
	Height          int             `xml:"height,attr"`
	ExpandedWidth   int             `xml:"expandedWidth,attr,omitempty"`
	ExpandedHeight  int             `xml:"expandedHeight,attr,omitempty"`
	Scalable        bool            `xml:"scalable,attr,omitempty"`
	MaintainAspect  bool            `xml:"maintainAspectRatio,attr,omitempty"`
	MinSuggestedDur string          `xml:"minSuggestedDuration,attr,omitempty"`
	APIFramework    string          `xml:"apiFramework,attr,omitempty"`
	StaticResource  *StaticResource `xml:"StaticResource,omitempty"`
	IFrameResource  string          `xml:"IFrameResource,omitempty"`
	HTMLResource    *HTMLResource   `xml:"HTMLResource,omitempty"`
	AdParameters    *AdParameters   `xml:"AdParameters,omitempty"`
	NonLinearClickThrough string    `xml:"NonLinearClickThrough,omitempty"`
	NonLinearClickTracking []string `xml:"NonLinearClickTracking,omitempty"`
}

// CompanionAds contains companion ad elements
type CompanionAds struct {
	Required  string      `xml:"required,attr,omitempty"`
	Companion []Companion `xml:"Companion"`
}

// Companion represents a companion ad
type Companion struct {
	ID                string          `xml:"id,attr,omitempty"`
	Width             int             `xml:"width,attr"`
	Height            int             `xml:"height,attr"`
	AssetWidth        int             `xml:"assetWidth,attr,omitempty"`
	AssetHeight       int             `xml:"assetHeight,attr,omitempty"`
	ExpandedWidth     int             `xml:"expandedWidth,attr,omitempty"`
	ExpandedHeight    int             `xml:"expandedHeight,attr,omitempty"`
	APIFramework      string          `xml:"apiFramework,attr,omitempty"`
	AdSlotID          string          `xml:"adSlotId,attr,omitempty"`
	PxRatio           string          `xml:"pxratio,attr,omitempty"`
	StaticResource    *StaticResource `xml:"StaticResource,omitempty"`
	IFrameResource    string          `xml:"IFrameResource,omitempty"`
	HTMLResource      *HTMLResource   `xml:"HTMLResource,omitempty"`
	AdParameters      *AdParameters   `xml:"AdParameters,omitempty"`
	AltText           string          `xml:"AltText,omitempty"`
	CompanionClickThrough  string     `xml:"CompanionClickThrough,omitempty"`
	CompanionClickTracking []string   `xml:"CompanionClickTracking,omitempty"`
	TrackingEvents    TrackingEvents  `xml:"TrackingEvents,omitempty"`
}

// Extensions contains extension elements
type Extensions struct {
	Extension []Extension `xml:"Extension"`
}

// Extension represents a custom extension
type Extension struct {
	Type  string `xml:"type,attr,omitempty"`
	Value string `xml:",innerxml"`
}

// CreativeExtensions contains creative extension elements
type CreativeExtensions struct {
	CreativeExtension []CreativeExtension `xml:"CreativeExtension"`
}

// CreativeExtension represents a creative extension
type CreativeExtension struct {
	Type  string `xml:"type,attr,omitempty"`
	Value string `xml:",innerxml"`
}

// Parse parses a VAST XML document
func Parse(data []byte) (*VAST, error) {
	var v VAST
	if err := xml.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("failed to parse VAST: %w", err)
	}
	return &v, nil
}

// Marshal serializes a VAST document to XML
func (v *VAST) Marshal() ([]byte, error) {
	data, err := xml.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal VAST: %w", err)
	}
	return append([]byte(xml.Header), data...), nil
}

// IsEmpty returns true if the VAST has no ads
func (v *VAST) IsEmpty() bool {
	return len(v.Ads) == 0
}

// GetLinearCreative returns the first linear creative from the first ad
func (v *VAST) GetLinearCreative() *Linear {
	for _, ad := range v.Ads {
		var creatives *Creatives
		if ad.InLine != nil {
			creatives = &ad.InLine.Creatives
		} else if ad.Wrapper != nil {
			creatives = &ad.Wrapper.Creatives
		}
		if creatives != nil {
			for _, c := range creatives.Creative {
				if c.Linear != nil {
					return c.Linear
				}
			}
		}
	}
	return nil
}

// GetMediaFiles returns all media files from the VAST
func (v *VAST) GetMediaFiles() []MediaFile {
	var files []MediaFile
	linear := v.GetLinearCreative()
	if linear != nil {
		files = append(files, linear.MediaFiles.MediaFile...)
	}
	return files
}
