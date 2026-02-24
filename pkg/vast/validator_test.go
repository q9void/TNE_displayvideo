package vast

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVASTValidation(t *testing.T) {
	t.Run("Valid_inline_VAST", func(t *testing.T) {
		v, err := NewBuilder("4.0").
			AddAd("test-ad").
			WithInLine("TestSystem", "Test Ad").
			WithImpression("https://example.com/imp").
			WithLinearCreative("creative-1", 30*time.Second).
			WithMediaFile("https://example.com/video.mp4", "video/mp4", 1920, 1080).
			EndLinear().
			Done().
			Build()

		require.NoError(t, err)

		result := v.Validate()
		assert.True(t, result.Valid, "Valid VAST should pass validation")
		assert.Empty(t, result.Errors)
	})

	t.Run("Missing_version", func(t *testing.T) {
		v := &VAST{
			Version: "",
			Ads:     []Ad{},
		}

		result := v.Validate()
		assert.False(t, result.Valid)
		assert.NotEmpty(t, result.Errors)
		assertHasError(t, result, "VAST.version")
	})

	t.Run("Invalid_version", func(t *testing.T) {
		v := &VAST{
			Version: "5.0", // Unsupported version
			Ads:     []Ad{},
		}

		result := v.Validate()
		assert.False(t, result.Valid)
		assertHasError(t, result, "VAST.version")
	})

	t.Run("Empty_VAST_without_error", func(t *testing.T) {
		v := &VAST{
			Version: "4.0",
			Ads:     []Ad{},
			Error:   "", // No error element
		}

		result := v.Validate()
		assert.False(t, result.Valid)
		assertHasError(t, result, "VAST.Ad")
	})

	t.Run("Empty_VAST_with_error", func(t *testing.T) {
		v := CreateErrorVAST("https://example.com/error")

		result := v.Validate()
		// Empty VAST with error is valid
		assert.True(t, result.Valid)
	})

	t.Run("Missing_InLine_and_Wrapper", func(t *testing.T) {
		v := &VAST{
			Version: "4.0",
			Ads: []Ad{
				{
					ID:      "test",
					InLine:  nil,
					Wrapper: nil,
				},
			},
		}

		result := v.Validate()
		assert.False(t, result.Valid)
		assertHasError(t, result, "VAST.Ad[0]")
	})

	t.Run("Both_InLine_and_Wrapper", func(t *testing.T) {
		v := &VAST{
			Version: "4.0",
			Ads: []Ad{
				{
					ID:      "test",
					InLine:  &InLine{},
					Wrapper: &Wrapper{},
				},
			},
		}

		result := v.Validate()
		assert.False(t, result.Valid)
		assertHasError(t, result, "VAST.Ad[0]")
	})
}

func TestInLineValidation(t *testing.T) {
	t.Run("Missing_AdSystem", func(t *testing.T) {
		v := &VAST{
			Version: "4.0",
			Ads: []Ad{
				{
					ID: "test",
					InLine: &InLine{
						AdSystem:    AdSystem{Value: ""}, // Missing
						AdTitle:     "Test",
						Impressions: []Impression{{Value: "https://example.com/imp"}},
						Creatives: Creatives{
							Creative: []Creative{
								{
									Linear: &Linear{
										Duration: "00:00:30",
										MediaFiles: &MediaFiles{
											MediaFile: []MediaFile{
												{
													Delivery: "progressive",
													Type:     "video/mp4",
													Width:    1920,
													Height:   1080,
													Value:    "https://example.com/video.mp4",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		result := v.Validate()
		assert.False(t, result.Valid)
		assertHasError(t, result, "AdSystem")
	})

	t.Run("Missing_AdTitle", func(t *testing.T) {
		v := &VAST{
			Version: "4.0",
			Ads: []Ad{
				{
					InLine: &InLine{
						AdSystem:    AdSystem{Value: "Test"},
						AdTitle:     "", // Missing
						Impressions: []Impression{{Value: "https://example.com/imp"}},
						Creatives: Creatives{
							Creative: []Creative{
								{Linear: &Linear{Duration: "00:00:30", MediaFiles: &MediaFiles{MediaFile: []MediaFile{{Delivery: "progressive", Type: "video/mp4", Width: 1920, Height: 1080, Value: "https://example.com/video.mp4"}}}}},
							},
						},
					},
				},
			},
		}

		result := v.Validate()
		assert.False(t, result.Valid)
		assertHasError(t, result, "AdTitle")
	})

	t.Run("Missing_Impression", func(t *testing.T) {
		v := &VAST{
			Version: "4.0",
			Ads: []Ad{
				{
					InLine: &InLine{
						AdSystem:    AdSystem{Value: "Test"},
						AdTitle:     "Test",
						Impressions: []Impression{}, // Empty
						Creatives: Creatives{
							Creative: []Creative{
								{Linear: &Linear{Duration: "00:00:30", MediaFiles: &MediaFiles{MediaFile: []MediaFile{{Delivery: "progressive", Type: "video/mp4", Width: 1920, Height: 1080, Value: "https://example.com/video.mp4"}}}}},
							},
						},
					},
				},
			},
		}

		result := v.Validate()
		assert.False(t, result.Valid)
		assertHasError(t, result, "Impression")
	})

	t.Run("Invalid_Impression_URL", func(t *testing.T) {
		v := &VAST{
			Version: "4.0",
			Ads: []Ad{
				{
					InLine: &InLine{
						AdSystem:    AdSystem{Value: "Test"},
						AdTitle:     "Test",
						Impressions: []Impression{{Value: "not-a-url"}}, // Invalid URL
						Creatives: Creatives{
							Creative: []Creative{
								{Linear: &Linear{Duration: "00:00:30", MediaFiles: &MediaFiles{MediaFile: []MediaFile{{Delivery: "progressive", Type: "video/mp4", Width: 1920, Height: 1080, Value: "https://example.com/video.mp4"}}}}},
							},
						},
					},
				},
			},
		}

		result := v.Validate()
		assert.False(t, result.Valid)
		assertHasError(t, result, "Impression")
	})

	t.Run("Invalid_Error_URL", func(t *testing.T) {
		v := &VAST{
			Version: "4.0",
			Ads: []Ad{
				{
					InLine: &InLine{
						AdSystem:    AdSystem{Value: "Test"},
						AdTitle:     "Test",
						Impressions: []Impression{{Value: "https://example.com/imp"}},
						Error:       "not-a-url", // Invalid
						Creatives: Creatives{
							Creative: []Creative{
								{Linear: &Linear{Duration: "00:00:30", MediaFiles: &MediaFiles{MediaFile: []MediaFile{{Delivery: "progressive", Type: "video/mp4", Width: 1920, Height: 1080, Value: "https://example.com/video.mp4"}}}}},
							},
						},
					},
				},
			},
		}

		result := v.Validate()
		assert.False(t, result.Valid)
		assertHasError(t, result, "Error")
	})

	t.Run("Missing_Creatives", func(t *testing.T) {
		v := &VAST{
			Version: "4.0",
			Ads: []Ad{
				{
					InLine: &InLine{
						AdSystem:    AdSystem{Value: "Test"},
						AdTitle:     "Test",
						Impressions: []Impression{{Value: "https://example.com/imp"}},
						Creatives:   Creatives{Creative: []Creative{}}, // Empty
					},
				},
			},
		}

		result := v.Validate()
		assert.False(t, result.Valid)
		assertHasError(t, result, "Creatives")
	})
}

func TestLinearValidation(t *testing.T) {
	t.Run("Missing_Duration", func(t *testing.T) {
		linear := &Linear{
			Duration: "", // Missing
			MediaFiles: &MediaFiles{
				MediaFile: []MediaFile{
					{
						Delivery: "progressive",
						Type:     "video/mp4",
						Width:    1920,
						Height:   1080,
						Value:    "https://example.com/video.mp4",
					},
				},
			},
		}

		result := &ValidationResult{Valid: true}
		validateLinear(linear, "Linear", result)

		assert.False(t, result.Valid)
		assertHasError(t, result, "Duration")
	})

	t.Run("Invalid_Duration_Format", func(t *testing.T) {
		linear := &Linear{
			Duration: "invalid", // Bad format
			MediaFiles: &MediaFiles{
				MediaFile: []MediaFile{
					{
						Delivery: "progressive",
						Type:     "video/mp4",
						Width:    1920,
						Height:   1080,
						Value:    "https://example.com/video.mp4",
					},
				},
			},
		}

		result := &ValidationResult{Valid: true}
		validateLinear(linear, "Linear", result)

		assert.False(t, result.Valid)
		assertHasError(t, result, "Duration")
	})

	t.Run("Missing_MediaFiles", func(t *testing.T) {
		linear := &Linear{
			Duration:   "00:00:30",
			MediaFiles: &MediaFiles{MediaFile: []MediaFile{}}, // Empty
		}

		result := &ValidationResult{Valid: true}
		validateLinear(linear, "Linear", result)

		assert.False(t, result.Valid)
		assertHasError(t, result, "MediaFiles")
	})
}

func TestMediaFileValidation(t *testing.T) {
	t.Run("Missing_Delivery", func(t *testing.T) {
		mf := &MediaFile{
			Delivery: "", // Missing
			Type:     "video/mp4",
			Width:    1920,
			Height:   1080,
			Value:    "https://example.com/video.mp4",
		}

		result := &ValidationResult{Valid: true}
		validateMediaFile(mf, "MediaFile", result)

		assert.False(t, result.Valid)
		assertHasError(t, result, "delivery")
	})

	t.Run("Invalid_Delivery", func(t *testing.T) {
		mf := &MediaFile{
			Delivery: "invalid", // Must be progressive or streaming
			Type:     "video/mp4",
			Width:    1920,
			Height:   1080,
			Value:    "https://example.com/video.mp4",
		}

		result := &ValidationResult{Valid: true}
		validateMediaFile(mf, "MediaFile", result)

		assert.False(t, result.Valid)
		assertHasError(t, result, "delivery")
	})

	t.Run("Missing_Type", func(t *testing.T) {
		mf := &MediaFile{
			Delivery: "progressive",
			Type:     "", // Missing
			Width:    1920,
			Height:   1080,
			Value:    "https://example.com/video.mp4",
		}

		result := &ValidationResult{Valid: true}
		validateMediaFile(mf, "MediaFile", result)

		assert.False(t, result.Valid)
		assertHasError(t, result, "type")
	})

	t.Run("Invalid_MIME_Type", func(t *testing.T) {
		mf := &MediaFile{
			Delivery: "progressive",
			Type:     "invalid/type",
			Width:    1920,
			Height:   1080,
			Value:    "https://example.com/video.mp4",
		}

		result := &ValidationResult{Valid: true}
		validateMediaFile(mf, "MediaFile", result)

		assert.False(t, result.Valid)
		assertHasError(t, result, "type")
	})

	t.Run("Invalid_Width", func(t *testing.T) {
		mf := &MediaFile{
			Delivery: "progressive",
			Type:     "video/mp4",
			Width:    0, // Invalid
			Height:   1080,
			Value:    "https://example.com/video.mp4",
		}

		result := &ValidationResult{Valid: true}
		validateMediaFile(mf, "MediaFile", result)

		assert.False(t, result.Valid)
		assertHasError(t, result, "width")
	})

	t.Run("Invalid_Height", func(t *testing.T) {
		mf := &MediaFile{
			Delivery: "progressive",
			Type:     "video/mp4",
			Width:    1920,
			Height:   0, // Invalid
			Value:    "https://example.com/video.mp4",
		}

		result := &ValidationResult{Valid: true}
		validateMediaFile(mf, "MediaFile", result)

		assert.False(t, result.Valid)
		assertHasError(t, result, "height")
	})

	t.Run("Missing_URL", func(t *testing.T) {
		mf := &MediaFile{
			Delivery: "progressive",
			Type:     "video/mp4",
			Width:    1920,
			Height:   1080,
			Value:    "", // Missing
		}

		result := &ValidationResult{Valid: true}
		validateMediaFile(mf, "MediaFile", result)

		assert.False(t, result.Valid)
	})

	t.Run("Valid_MIME_Types", func(t *testing.T) {
		validTypes := []string{
			"video/mp4",
			"video/webm",
			"video/ogg",
			"application/javascript", // VPAID
		}

		for _, mimeType := range validTypes {
			assert.True(t, isValidMIMEType(mimeType), "Should accept %s", mimeType)
		}
	})
}

func TestTrackingValidation(t *testing.T) {
	t.Run("Missing_Event", func(t *testing.T) {
		tracking := &Tracking{
			Event: "", // Missing
			Value: "https://example.com/track",
		}

		result := &ValidationResult{Valid: true}
		validateTracking(tracking, "Tracking", result)

		assert.False(t, result.Valid)
		assertHasError(t, result, "event")
	})

	t.Run("Invalid_Event_Type", func(t *testing.T) {
		tracking := &Tracking{
			Event: "invalidEvent",
			Value: "https://example.com/track",
		}

		result := &ValidationResult{Valid: true}
		validateTracking(tracking, "Tracking", result)

		assert.False(t, result.Valid)
		assertHasError(t, result, "event")
	})

	t.Run("Valid_Event_Types", func(t *testing.T) {
		validEvents := []string{
			EventStart,
			EventFirstQuartile,
			EventMidpoint,
			EventThirdQuartile,
			EventComplete,
			EventMute,
			EventUnmute,
			EventPause,
			EventResume,
			EventSkip,
		}

		for _, event := range validEvents {
			assert.True(t, isValidEventType(event), "Should accept %s", event)
		}
	})
}

func TestWrapperValidation(t *testing.T) {
	t.Run("Valid_Wrapper", func(t *testing.T) {
		v := &VAST{
			Version: "4.0",
			Ads: []Ad{
				{
					Wrapper: &Wrapper{
						AdSystem:     AdSystem{Value: "Test"},
						VASTAdTagURI: CDATAElement{Value: "https://example.com/vast"},
						Impressions:  []Impression{{Value: "https://example.com/imp"}},
					},
				},
			},
		}

		result := v.Validate()
		assert.True(t, result.Valid)
	})

	t.Run("Missing_VASTAdTagURI", func(t *testing.T) {
		v := &VAST{
			Version: "4.0",
			Ads: []Ad{
				{
					Wrapper: &Wrapper{
						AdSystem:     AdSystem{Value: "Test"},
						VASTAdTagURI: CDATAElement{Value: ""}, // Missing
					},
				},
			},
		}

		result := v.Validate()
		assert.False(t, result.Valid)
		assertHasError(t, result, "VASTAdTagURI")
	})

	t.Run("Invalid_VASTAdTagURI", func(t *testing.T) {
		v := &VAST{
			Version: "4.0",
			Ads: []Ad{
				{
					Wrapper: &Wrapper{
						AdSystem:     AdSystem{Value: "Test"},
						VASTAdTagURI: CDATAElement{Value: "not-a-url"},
					},
				},
			},
		}

		result := v.Validate()
		assert.False(t, result.Valid)
		assertHasError(t, result, "VASTAdTagURI")
	})
}

func TestURLValidation(t *testing.T) {
	t.Run("Valid_URLs", func(t *testing.T) {
		validURLs := []string{
			"https://example.com/path",
			"http://example.com/path",
			"https://example.com/path?param=value",
			"https://example.com/path?price=${AUCTION_PRICE}", // With macro
			"https://example.com/error?code=[ERRORCODE]",      // With macro
		}

		for _, url := range validURLs {
			assert.True(t, isValidURL(url), "Should accept %s", url)
		}
	})

	t.Run("Invalid_URLs", func(t *testing.T) {
		invalidURLs := []string{
			"not-a-url",
			"ftp://example.com", // Wrong scheme
			"",
		}

		for _, url := range invalidURLs {
			assert.False(t, isValidURL(url), "Should reject %s", url)
		}
	})
}

func TestCompanionValidation(t *testing.T) {
	t.Run("Valid_Companion", func(t *testing.T) {
		companion := &Companion{
			Width:  300,
			Height: 250,
			StaticResource: &StaticResource{
				CreativeType: "image/jpeg",
				Value:        "https://example.com/banner.jpg",
			},
		}

		result := &ValidationResult{Valid: true}
		validateCompanion(companion, "Companion", result)

		assert.True(t, result.Valid)
	})

	t.Run("Invalid_Width", func(t *testing.T) {
		companion := &Companion{
			Width:  0, // Invalid
			Height: 250,
			StaticResource: &StaticResource{
				Value: "https://example.com/banner.jpg",
			},
		}

		result := &ValidationResult{Valid: true}
		validateCompanion(companion, "Companion", result)

		assert.False(t, result.Valid)
	})

	t.Run("Missing_Resource", func(t *testing.T) {
		companion := &Companion{
			Width:  300,
			Height: 250,
			// No resource
		}

		result := &ValidationResult{Valid: true}
		validateCompanion(companion, "Companion", result)

		assert.False(t, result.Valid)
	})
}

// Helper function to check if error exists for a field
func assertHasError(t *testing.T, result *ValidationResult, fieldContains string) {
	t.Helper()
	for _, err := range result.Errors {
		if containsString(err.Field, fieldContains) {
			return
		}
	}
	t.Errorf("Expected error for field containing '%s', but not found. Errors: %v", fieldContains, result.Errors)
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
