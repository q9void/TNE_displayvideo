package vast

import (
	"fmt"
	"time"
)

// Builder provides a fluent interface for constructing VAST documents
type Builder struct {
	vast    *VAST
	current *Ad
	err     error
}

// NewBuilder creates a new VAST builder
func NewBuilder(version string) *Builder {
	if version == "" {
		version = "4.0"
	}
	return &Builder{
		vast: &VAST{
			Version: version,
			Ads:     make([]Ad, 0),
		},
	}
}

// AddAd starts building a new ad
func (b *Builder) AddAd(id string) *Builder {
	if b.err != nil {
		return b
	}
	b.current = &Ad{ID: id}
	return b
}

// WithInLine sets the current ad as an inline ad
func (b *Builder) WithInLine(adSystem, adTitle string) *Builder {
	if b.err != nil || b.current == nil {
		return b
	}
	b.current.InLine = &InLine{
		AdSystem:    AdSystem{Value: adSystem},
		AdTitle:     adTitle,
		Impressions: make([]Impression, 0),
		Creatives:   Creatives{Creative: make([]Creative, 0)},
	}
	return b
}

// WithWrapper sets the current ad as a wrapper ad
func (b *Builder) WithWrapper(adSystem, vastTagURI string) *Builder {
	if b.err != nil || b.current == nil {
		return b
	}
	b.current.Wrapper = &Wrapper{
		AdSystem:     AdSystem{Value: adSystem},
		VASTAdTagURI: CDATAElement{Value: vastTagURI},
		Impressions:  make([]Impression, 0),
	}
	return b
}

// WithImpression adds an impression tracking URL
func (b *Builder) WithImpression(url string, id ...string) *Builder {
	if b.err != nil || b.current == nil {
		return b
	}
	imp := Impression{Value: url}
	if len(id) > 0 {
		imp.ID = id[0]
	}
	if b.current.InLine != nil {
		b.current.InLine.Impressions = append(b.current.InLine.Impressions, imp)
	} else if b.current.Wrapper != nil {
		b.current.Wrapper.Impressions = append(b.current.Wrapper.Impressions, imp)
	}
	return b
}

// WithError adds an error tracking URL
func (b *Builder) WithError(url string) *Builder {
	if b.err != nil || b.current == nil {
		return b
	}
	if b.current.InLine != nil {
		b.current.InLine.Error = url
	} else if b.current.Wrapper != nil {
		b.current.Wrapper.Error = url
	}
	return b
}

// WithLinearCreative adds a linear creative to the current ad
func (b *Builder) WithLinearCreative(id string, duration time.Duration) *LinearBuilder {
	if b.err != nil || b.current == nil {
		return &LinearBuilder{parent: b, err: fmt.Errorf("no current ad")}
	}

	linear := &Linear{
		Duration:       Duration(FormatDuration(duration)),
		MediaFiles:     &MediaFiles{MediaFile: make([]MediaFile, 0)},
		TrackingEvents: TrackingEvents{Tracking: make([]Tracking, 0)},
	}

	creative := Creative{
		ID:     id,
		Linear: linear,
	}

	if b.current.InLine != nil {
		b.current.InLine.Creatives.Creative = append(b.current.InLine.Creatives.Creative, creative)
	} else if b.current.Wrapper != nil {
		b.current.Wrapper.Creatives.Creative = append(b.current.Wrapper.Creatives.Creative, creative)
	}

	return &LinearBuilder{
		parent: b,
		linear: linear,
	}
}

// Done finalizes the current ad and adds it to the VAST
func (b *Builder) Done() *Builder {
	if b.err != nil {
		return b
	}
	if b.current != nil {
		b.vast.Ads = append(b.vast.Ads, *b.current)
		b.current = nil
	}
	return b
}

// Build returns the constructed VAST document
func (b *Builder) Build() (*VAST, error) {
	if b.err != nil {
		return nil, b.err
	}
	// Finalize any pending ad
	if b.current != nil {
		b.vast.Ads = append(b.vast.Ads, *b.current)
	}
	return b.vast, nil
}

// LinearBuilder provides a fluent interface for building linear creatives
type LinearBuilder struct {
	parent *Builder
	linear *Linear
	err    error
}

// WithMediaFile adds a media file to the linear creative
func (lb *LinearBuilder) WithMediaFile(url, mimeType string, width, height int, opts ...MediaFileOption) *LinearBuilder {
	if lb.err != nil {
		return lb
	}
	mf := MediaFile{
		Delivery: "progressive",
		Type:     mimeType,
		Width:    width,
		Height:   height,
		Value:    url,
	}
	for _, opt := range opts {
		opt(&mf)
	}
	if lb.linear.MediaFiles == nil {
		lb.linear.MediaFiles = &MediaFiles{}
	}
	lb.linear.MediaFiles.MediaFile = append(lb.linear.MediaFiles.MediaFile, mf)
	return lb
}

// MediaFileOption is a function that modifies a MediaFile
type MediaFileOption func(*MediaFile)

// WithBitrate sets the bitrate for a media file
func WithBitrate(bitrate int) MediaFileOption {
	return func(mf *MediaFile) {
		mf.Bitrate = bitrate
	}
}

// WithDelivery sets the delivery method for a media file
func WithDelivery(delivery string) MediaFileOption {
	return func(mf *MediaFile) {
		mf.Delivery = delivery
	}
}

// WithCodec sets the codec for a media file
func WithCodec(codec string) MediaFileOption {
	return func(mf *MediaFile) {
		mf.Codec = codec
	}
}

// WithTracking adds a tracking event to the linear creative
func (lb *LinearBuilder) WithTracking(event, url string, offset ...string) *LinearBuilder {
	if lb.err != nil {
		return lb
	}
	t := Tracking{
		Event: event,
		Value: url,
	}
	if len(offset) > 0 {
		t.Offset = offset[0]
	}
	lb.linear.TrackingEvents.Tracking = append(lb.linear.TrackingEvents.Tracking, t)
	return lb
}

// WithAllQuartileTracking adds all standard quartile tracking events
func (lb *LinearBuilder) WithAllQuartileTracking(baseURL string) *LinearBuilder {
	events := []string{
		EventStart,
		EventFirstQuartile,
		EventMidpoint,
		EventThirdQuartile,
		EventComplete,
	}
	for _, event := range events {
		lb.WithTracking(event, fmt.Sprintf("%s?event=%s", baseURL, event))
	}
	return lb
}

// WithClickThrough sets the click-through URL
func (lb *LinearBuilder) WithClickThrough(url string) *LinearBuilder {
	if lb.err != nil {
		return lb
	}
	if lb.linear.VideoClicks == nil {
		lb.linear.VideoClicks = &VideoClicks{}
	}
	lb.linear.VideoClicks.ClickThrough = &ClickThrough{Value: url}
	return lb
}

// WithClickTracking adds a click tracking URL
func (lb *LinearBuilder) WithClickTracking(url string, id ...string) *LinearBuilder {
	if lb.err != nil {
		return lb
	}
	if lb.linear.VideoClicks == nil {
		lb.linear.VideoClicks = &VideoClicks{}
	}
	ct := ClickTracking{Value: url}
	if len(id) > 0 {
		ct.ID = id[0]
	}
	lb.linear.VideoClicks.ClickTracking = append(lb.linear.VideoClicks.ClickTracking, ct)
	return lb
}

// WithSkipOffset sets the skip offset for skippable ads
func (lb *LinearBuilder) WithSkipOffset(offset string) *LinearBuilder {
	if lb.err != nil {
		return lb
	}
	lb.linear.SkipOffset = offset
	return lb
}

// EndLinear finishes the linear creative and returns to the ad builder
func (lb *LinearBuilder) EndLinear() *Builder {
	return lb.parent
}

// CreateEmptyVAST creates an empty VAST response (no ads available)
func CreateEmptyVAST() *VAST {
	return &VAST{
		Version: "4.0",
		Ads:     []Ad{},
	}
}

// CreateErrorVAST creates a VAST response with an error URL
func CreateErrorVAST(errorURL string) *VAST {
	return &VAST{
		Version: "4.0",
		Ads:     []Ad{},
		Error:   errorURL,
	}
}
