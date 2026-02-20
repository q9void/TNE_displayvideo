// Package openrtb provides DSA (Digital Services Act) compliance models
package openrtb

// ValidationError represents a validation error with field context
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

// DSA represents Digital Services Act transparency requirements
// Specification: IAB Tech Lab OpenRTB 2.6 DSA Transparency Extension
type DSA struct {
	// DSARequired indicates if DSA transparency is required
	// 0 = Not required
	// 1 = Supported, bid responses with or without DSA object will be accepted
	// 2 = Required, bid responses without DSA object will not be accepted
	// 3 = Required, bid responses without DSA object will not be accepted,
	//     Publisher will render DSA transparency info
	DSARequired int `json:"dsarequired,omitempty"`

	// PubRender indicates if the publisher will render DSA transparency info
	// 0 = Publisher will not render
	// 1 = Publisher will render
	// 2 = Publisher may render depending on other factors
	PubRender int `json:"pubrender,omitempty"`

	// DataToPub indicates what DSA data should be passed to publisher
	// 0 = Do not send
	// 1 = Send only if present in bid response
	// 2 = Send even if not present (use defaults)
	DataToPub int `json:"datatopub,omitempty"`

	// Transparency contains DSA transparency parameters (in bid response)
	Transparency []DSATransparency `json:"transparency,omitempty"`
}

// DSATransparency represents advertiser transparency information
type DSATransparency struct {
	// Domain of the entity that paid for the ad
	Domain string `json:"domain,omitempty"`

	// DSAParams are parameters reflecting DSA declarations
	DSAParams []int `json:"dsaparams,omitempty"`
}

// DSA Param values (for DSAParams array)
const (
	// DSAParamNotApplicable - DSA does not apply
	DSAParamNotApplicable = 0

	// DSAParamPaidForBy - Establish who paid for the ad
	DSAParamPaidForBy = 1

	// DSAParamOnBehalfOf - Establish on whose behalf the ad was published
	DSAParamOnBehalfOf = 2

	// DSAParamPaidForByAndOnBehalfOf - Both paid for by and on behalf of
	DSAParamPaidForByAndOnBehalfOf = 3
)

// DSARequired values
const (
	// DSANotRequired - DSA transparency is not required
	DSANotRequired = 0

	// DSASupported - Supported, responses with or without DSA accepted
	DSASupported = 1

	// DSARequired - Required, responses without DSA will be rejected
	DSARequired = 2

	// DSARequiredPubRender - Required, publisher will render transparency info
	DSARequiredPubRender = 3
)

// PubRender values
const (
	// PubRenderNo - Publisher will not render
	PubRenderNo = 0

	// PubRenderYes - Publisher will render
	PubRenderYes = 1

	// PubRenderMaybe - Publisher may render depending on other factors
	PubRenderMaybe = 2
)

// DataToPub values
const (
	// DataToPubNo - Do not send DSA data to publisher
	DataToPubNo = 0

	// DataToPubIfPresent - Send only if present in bid response
	DataToPubIfPresent = 1

	// DataToPubAlways - Send even if not present (use defaults)
	DataToPubAlways = 2
)

// ValidateDSA validates DSA object
func (d *DSA) ValidateDSA() error {
	if d == nil {
		return nil
	}

	// Validate DSARequired range
	if d.DSARequired < 0 || d.DSARequired > 3 {
		return ErrInvalidDSARequired
	}

	// Validate PubRender range
	if d.PubRender < 0 || d.PubRender > 2 {
		return ErrInvalidPubRender
	}

	// Validate DataToPub range
	if d.DataToPub < 0 || d.DataToPub > 2 {
		return ErrInvalidDataToPub
	}

	// Validate transparency entries
	for _, trans := range d.Transparency {
		if trans.Domain == "" {
			return ErrMissingDSADomain
		}
	}

	return nil
}

// IsDSARequired checks if DSA transparency is required
func (d *DSA) IsDSARequired() bool {
	if d == nil {
		return false
	}
	return d.DSARequired >= DSARequired
}

// ShouldPublisherRender checks if publisher should render DSA info
func (d *DSA) ShouldPublisherRender() bool {
	if d == nil {
		return false
	}
	return d.PubRender == PubRenderYes ||
		d.DSARequired == DSARequiredPubRender
}

// ShouldSendDataToPub checks if DSA data should be sent to publisher
func (d *DSA) ShouldSendDataToPub(hasTransparency bool) bool {
	if d == nil {
		return false
	}

	switch d.DataToPub {
	case DataToPubNo:
		return false
	case DataToPubIfPresent:
		return hasTransparency
	case DataToPubAlways:
		return true
	default:
		return false
	}
}

// DSA Errors
var (
	ErrInvalidDSARequired = &ValidationError{Field: "dsa.dsarequired", Message: "must be 0-3"}
	ErrInvalidPubRender   = &ValidationError{Field: "dsa.pubrender", Message: "must be 0-2"}
	ErrInvalidDataToPub   = &ValidationError{Field: "dsa.datatopub", Message: "must be 0-2"}
	ErrMissingDSADomain   = &ValidationError{Field: "dsa.transparency.domain", Message: "required"}
	ErrDSARequired        = &ValidationError{Field: "dsa", Message: "DSA transparency required but not provided"}
)
