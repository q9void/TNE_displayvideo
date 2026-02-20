// Package privacy provides privacy compliance features including GPP support
package privacy

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// GPP (Global Privacy Platform) implementation
// Specification: https://github.com/InteractiveAdvertisingBureau/Global-Privacy-Platform

// GPPString represents a parsed GPP consent string
type GPPString struct {
	// Raw GPP string
	Raw string

	// GPP Version (currently 1)
	Version int

	// Sections present in the string
	Sections []GPPSection

	// Parsed section data by ID
	SectionData map[int]interface{}
}

// GPPSection represents a section in the GPP string
type GPPSection struct {
	// Section ID (e.g., 2 for TCFv2, 6 for US Privacy, 7 for US National)
	ID int

	// Section name for debugging
	Name string

	// Raw section string
	Raw string

	// Parsed data (section-specific)
	Data interface{}
}

// GPP Section IDs
const (
	GPPSectionTCFv2EU        = 2  // TCF v2 European Union
	GPPSectionGPPHeader      = 3  // GPP Header
	GPPSectionTCFv2CA        = 5  // TCF v2 Canada
	GPPSectionUSPrivacy      = 6  // US Privacy (Legacy CCPA)
	GPPSectionUSNational     = 7  // US National Privacy
	GPPSectionUSCA           = 8  // US California
	GPPSectionUSVA           = 9  // US Virginia
	GPPSectionUSCO           = 10 // US Colorado
	GPPSectionUSUT           = 11 // US Utah
	GPPSectionUSCT           = 12 // US Connecticut
)

// GPP Section Names
var gppSectionNames = map[int]string{
	GPPSectionTCFv2EU:    "TCFv2-EU",
	GPPSectionGPPHeader:  "GPP-Header",
	GPPSectionTCFv2CA:    "TCFv2-CA",
	GPPSectionUSPrivacy:  "US-Privacy",
	GPPSectionUSNational: "US-National",
	GPPSectionUSCA:       "US-CA",
	GPPSectionUSVA:       "US-VA",
	GPPSectionUSCO:       "US-CO",
	GPPSectionUSUT:       "US-UT",
	GPPSectionUSCT:       "US-CT",
}

// USNationalData represents parsed US National Privacy section
type USNationalData struct {
	// Version (1 or 2)
	Version int

	// SharingNotice: 0=Not applicable, 1=Yes, 2=No
	SharingNotice int

	// SaleOptOutNotice: 0=Not applicable, 1=Yes, 2=No
	SaleOptOutNotice int

	// TargetedAdvertisingOptOutNotice: 0=Not applicable, 1=Yes, 2=No
	TargetedAdvertisingOptOutNotice int

	// SensitiveDataProcessingOptOutNotice: 0=Not applicable, 1=Yes, 2=No
	SensitiveDataProcessingOptOutNotice int

	// SaleOptOut: 0=Not applicable, 1=Opted out, 2=Did not opt out
	SaleOptOut int

	// TargetedAdvertisingOptOut: 0=Not applicable, 1=Opted out, 2=Did not opt out
	TargetedAdvertisingOptOut int

	// SensitiveDataProcessing: Array of 12 consent values (0=Not applicable, 1=Consent, 2=No consent)
	SensitiveDataProcessing [12]int

	// KnownChildSensitiveDataConsents: Array of 2 values (0=Not applicable, 1=Consent, 2=No consent)
	KnownChildSensitiveDataConsents [2]int

	// PersonalDataConsents: 0=Not applicable, 1=Consent, 2=No consent
	PersonalDataConsents int

	// MspaCoveredTransaction: 0=Not applicable, 1=Yes, 2=No
	MspaCoveredTransaction int

	// MspaOptOutOptionMode: 0=Not applicable, 1=Yes, 2=No
	MspaOptOutOptionMode int

	// MspaServiceProviderMode: 0=Not applicable, 1=Yes, 2=No
	MspaServiceProviderMode int
}

// TCFv2Data represents parsed TCF v2 European Union section
// This wraps the existing GDPR TCF v2 implementation
type TCFv2Data struct {
	// Consent string (compatible with existing TCF parser)
	ConsentString string

	// Parsed consent (use existing parser)
	// Will integrate with existing middleware/privacy.go TCF parsing
}

// ParseGPPString parses a GPP consent string
func ParseGPPString(gppString string) (*GPPString, error) {
	if gppString == "" {
		return nil, errors.New("empty GPP string")
	}

	// GPP string format: DBABMA~CPXxRfAPXxRfAAfKABENB-CgAAAAAAAAAAYgAAAAAAAA~1YNN
	// Format: Header~Section1~Section2~...
	parts := strings.Split(gppString, "~")
	if len(parts) < 1 {
		return nil, errors.New("invalid GPP string format")
	}

	gpp := &GPPString{
		Raw:         gppString,
		Version:     1, // Default to version 1
		Sections:    make([]GPPSection, 0),
		SectionData: make(map[int]interface{}),
	}

	// Parse header (first part contains section IDs)
	header := parts[0]
	sectionIDs, err := parseGPPHeader(header)
	if err != nil {
		return nil, fmt.Errorf("invalid GPP header: %w", err)
	}

	// Parse each section
	if len(parts) > 1 {
		sectionStrings := parts[1:]
		if len(sectionStrings) != len(sectionIDs) {
			return nil, fmt.Errorf("section count mismatch: header declares %d, found %d",
				len(sectionIDs), len(sectionStrings))
		}

		for i, sectionID := range sectionIDs {
			section := GPPSection{
				ID:   sectionID,
				Name: gppSectionNames[sectionID],
				Raw:  sectionStrings[i],
			}

			// Parse section data based on type
			data, err := parseGPPSection(sectionID, sectionStrings[i])
			if err != nil {
				// Log error but don't fail - some sections may not be implemented yet
				section.Data = nil
			} else {
				section.Data = data
				gpp.SectionData[sectionID] = data
			}

			gpp.Sections = append(gpp.Sections, section)
		}
	}

	return gpp, nil
}

// parseGPPHeader extracts section IDs from the GPP header
func parseGPPHeader(header string) ([]int, error) {
	// Header is base64url encoded and contains section IDs
	// For now, use a simplified parsing approach
	// Format: DBABMA encodes type, version, and section IDs

	// This is a simplified implementation
	// In production, would use proper base64url decoding and bit parsing
	decoded, err := base64.RawURLEncoding.DecodeString(header)
	if err != nil {
		return nil, fmt.Errorf("failed to decode header: %w", err)
	}

	if len(decoded) < 3 {
		return nil, errors.New("header too short")
	}

	// Extract section IDs from decoded header
	// This is a placeholder - real implementation would parse the bitfield
	sectionIDs := []int{}

	// For now, detect common sections based on header prefix
	// TODO: Implement proper bitfield parsing per GPP spec
	if strings.HasPrefix(header, "DB") {
		// Common GPP string with TCF v2 and US sections
		sectionIDs = []int{GPPSectionTCFv2EU, GPPSectionUSNational}
	} else {
		// Default to TCF v2 only
		sectionIDs = []int{GPPSectionTCFv2EU}
	}

	return sectionIDs, nil
}

// parseGPPSection parses a specific GPP section
func parseGPPSection(sectionID int, sectionString string) (interface{}, error) {
	switch sectionID {
	case GPPSectionTCFv2EU:
		return parseTCFv2Section(sectionString)
	case GPPSectionUSNational:
		return parseUSNationalSection(sectionString)
	case GPPSectionUSPrivacy:
		return parseUSPrivacySection(sectionString)
	default:
		// Unknown section - return raw string
		return sectionString, nil
	}
}

// parseTCFv2Section parses TCF v2 section (reuses existing TCF parser)
func parseTCFv2Section(sectionString string) (*TCFv2Data, error) {
	// This would integrate with existing TCF parsing in middleware/privacy.go
	// For now, just store the raw string
	return &TCFv2Data{
		ConsentString: sectionString,
	}, nil
}

// parseUSNationalSection parses US National Privacy section
func parseUSNationalSection(sectionString string) (*USNationalData, error) {
	// Decode base64url string
	decoded, err := base64.RawURLEncoding.DecodeString(sectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to decode US National section: %w", err)
	}

	if len(decoded) < 9 {
		return nil, errors.New("US National section too short")
	}

	// Parse bitfield according to US National Privacy spec
	// This is a simplified implementation
	data := &USNationalData{
		Version: 1,
	}

	// Bit parsing would happen here
	// For now, return basic structure
	// TODO: Implement full bitfield parsing per GPP US National spec

	return data, nil
}

// parseUSPrivacySection parses legacy US Privacy (CCPA) section
func parseUSPrivacySection(sectionString string) (string, error) {
	// US Privacy is a simple 4-character string: 1YNN
	if len(sectionString) != 4 {
		return "", errors.New("invalid US Privacy string length")
	}
	return sectionString, nil
}

// HasSection checks if GPP string contains a specific section
func (g *GPPString) HasSection(sectionID int) bool {
	_, exists := g.SectionData[sectionID]
	return exists
}

// GetTCFv2Consent returns TCF v2 consent string if present
func (g *GPPString) GetTCFv2Consent() (string, bool) {
	if data, ok := g.SectionData[GPPSectionTCFv2EU].(*TCFv2Data); ok {
		return data.ConsentString, true
	}
	return "", false
}

// GetUSNationalData returns US National privacy data if present
func (g *GPPString) GetUSNationalData() (*USNationalData, bool) {
	if data, ok := g.SectionData[GPPSectionUSNational].(*USNationalData); ok {
		return data, true
	}
	return nil, false
}

// GetUSPrivacyString returns legacy US Privacy string if present
func (g *GPPString) GetUSPrivacyString() (string, bool) {
	if data, ok := g.SectionData[GPPSectionUSPrivacy].(string); ok {
		return data, true
	}
	return "", false
}

// IsOptedOutOfSale checks if user opted out of data sale (any US section)
func (g *GPPString) IsOptedOutOfSale() bool {
	// Check US National section
	if usData, ok := g.GetUSNationalData(); ok {
		if usData.SaleOptOut == 1 {
			return true
		}
	}

	// Check legacy US Privacy string
	if usPrivacy, ok := g.GetUSPrivacyString(); ok {
		// Format: 1YNN where third char is opt-out (Y=opted out)
		if len(usPrivacy) >= 3 && usPrivacy[2] == 'Y' {
			return true
		}
	}

	return false
}

// IsOptedOutOfTargeting checks if user opted out of targeted advertising
func (g *GPPString) IsOptedOutOfTargeting() bool {
	if usData, ok := g.GetUSNationalData(); ok {
		if usData.TargetedAdvertisingOptOut == 1 {
			return true
		}
	}
	return false
}

// RequiresConsent checks if any section requires consent
func (g *GPPString) RequiresConsent() bool {
	// Has TCF v2 (GDPR) section
	if g.HasSection(GPPSectionTCFv2EU) {
		return true
	}

	// Has any US privacy section
	if g.HasSection(GPPSectionUSNational) ||
		g.HasSection(GPPSectionUSCA) ||
		g.HasSection(GPPSectionUSVA) ||
		g.HasSection(GPPSectionUSCO) ||
		g.HasSection(GPPSectionUSUT) ||
		g.HasSection(GPPSectionUSCT) {
		return true
	}

	return false
}
