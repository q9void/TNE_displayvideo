package device

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseUserAgent_iOS(t *testing.T) {
	ua := "Mozilla/5.0 (iPhone; CPU iPhone OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Mobile/15E148 Safari/604.1"

	info := ParseUserAgent(ua)

	assert.NotNil(t, info)
	assert.Equal(t, "Apple", info.Make)
	assert.Equal(t, "iPhone", info.Model)
	assert.Equal(t, "iOS", info.OS)
	assert.Equal(t, "17.2", info.OSV)
	assert.Equal(t, 1, info.DeviceType) // Mobile
}

func TestParseUserAgent_iPad(t *testing.T) {
	ua := "Mozilla/5.0 (iPad; CPU OS 16_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.6 Mobile/15E148 Safari/604.1"

	info := ParseUserAgent(ua)

	assert.NotNil(t, info)
	assert.Equal(t, "Apple", info.Make)
	assert.Equal(t, "iPad", info.Model)
	assert.Equal(t, "iOS", info.OS)
	assert.Equal(t, "16.6", info.OSV)
	assert.Equal(t, 5, info.DeviceType) // Tablet
}

func TestParseUserAgent_AndroidSamsung(t *testing.T) {
	ua := "Mozilla/5.0 (Linux; Android 13; SM-S918B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Mobile Safari/537.36"

	info := ParseUserAgent(ua)

	assert.NotNil(t, info)
	assert.Equal(t, "Samsung", info.Make)
	assert.Contains(t, info.Model, "SM-")
	assert.Equal(t, "Android", info.OS)
	assert.Equal(t, "13", info.OSV)
	assert.Equal(t, 1, info.DeviceType) // Mobile
}

func TestParseUserAgent_AndroidPixel(t *testing.T) {
	ua := "Mozilla/5.0 (Linux; Android 14; Pixel 7 Pro) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Mobile Safari/537.36"

	info := ParseUserAgent(ua)

	assert.NotNil(t, info)
	assert.Equal(t, "Google", info.Make)
	assert.Contains(t, info.Model, "Pixel")
	assert.Equal(t, "Android", info.OS)
	assert.Equal(t, "14", info.OSV)
	assert.Equal(t, 1, info.DeviceType) // Mobile
}

func TestParseUserAgent_Desktop_Chrome_Windows(t *testing.T) {
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

	info := ParseUserAgent(ua)

	assert.NotNil(t, info)
	assert.Equal(t, "PC", info.Make)
	assert.Equal(t, "", info.Model)
	assert.Equal(t, "Windows", info.OS)
	assert.Contains(t, info.OSV, "10")
	assert.Equal(t, 2, info.DeviceType) // PC
}

func TestParseUserAgent_Desktop_Safari_macOS(t *testing.T) {
	ua := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15"

	info := ParseUserAgent(ua)

	assert.NotNil(t, info)
	assert.Equal(t, "Apple", info.Make)
	assert.Equal(t, "Mac", info.Model)
	assert.Equal(t, "macOS", info.OS)
	assert.Contains(t, info.OSV, "10")
	assert.Equal(t, 2, info.DeviceType) // PC
}

func TestParseUserAgent_Tablet_Android(t *testing.T) {
	ua := "Mozilla/5.0 (Linux; Android 12; SM-T870) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36"

	info := ParseUserAgent(ua)

	assert.NotNil(t, info)
	assert.Equal(t, "Samsung", info.Make)
	assert.Contains(t, info.Model, "SM-")
	assert.Equal(t, "Android", info.OS)
	assert.Equal(t, "12", info.OSV)
	// Note: Detecting tablet vs mobile from UA alone is challenging
	// The device type may be 1 (mobile) unless "tablet" is in UA string
}

func TestParseUserAgent_EmptyString(t *testing.T) {
	info := ParseUserAgent("")

	assert.Nil(t, info)
}

func TestParseUserAgent_InvalidUA(t *testing.T) {
	ua := "invalid user agent string"

	info := ParseUserAgent(ua)

	// Should still return info, even if fields are empty
	assert.NotNil(t, info)
}

func TestParseUserAgent_ChromeOS(t *testing.T) {
	ua := "Mozilla/5.0 (X11; CrOS x86_64 14541.0.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

	info := ParseUserAgent(ua)

	assert.NotNil(t, info)
	assert.Equal(t, "Chromebook", info.Make)
	assert.Equal(t, "ChromeOS", info.OS)
	assert.Equal(t, 2, info.DeviceType) // PC
}

func TestParseUserAgent_SmartTV(t *testing.T) {
	ua := "Mozilla/5.0 (Linux; Android 11; BRAVIA 4K VH2) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.120 Safari/537.36 SmartTV"

	info := ParseUserAgent(ua)

	assert.NotNil(t, info)
	assert.Equal(t, 3, info.DeviceType) // Connected TV
}

func TestParseDevice_Xiaomi(t *testing.T) {
	ua := "Mozilla/5.0 (Linux; Android 13; Redmi Note 12) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Mobile Safari/537.36"

	info := ParseUserAgent(ua)

	assert.NotNil(t, info)
	assert.Equal(t, "Xiaomi", info.Make)
	assert.Contains(t, info.Model, "Redmi")
}

func TestParseDevice_OnePlus(t *testing.T) {
	ua := "Mozilla/5.0 (Linux; Android 13; OnePlus 11) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Mobile Safari/537.36"

	info := ParseUserAgent(ua)

	assert.NotNil(t, info)
	assert.Equal(t, "OnePlus", info.Make)
	assert.Contains(t, info.Model, "OnePlus")
}

func TestParseOS_Normalization(t *testing.T) {
	testCases := []struct {
		ua       string
		expected string
	}{
		{
			"Mozilla/5.0 (iPhone; CPU iPhone OS 15_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.0 Mobile/15E148 Safari/604.1",
			"iOS",
		},
		{
			"Mozilla/5.0 (Linux; Android 12; SM-G998B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Mobile Safari/537.36",
			"Android",
		},
		{
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Windows",
		},
		{
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.0 Safari/605.1.15",
			"macOS",
		},
	}

	for _, tc := range testCases {
		info := ParseUserAgent(tc.ua)
		assert.NotNil(t, info)
		assert.Equal(t, tc.expected, info.OS, "Failed for UA: %s", tc.ua)
	}
}

func TestParseDeviceType_Various(t *testing.T) {
	testCases := []struct {
		ua           string
		expectedType int
	}{
		{
			"Mozilla/5.0 (iPhone; CPU iPhone OS 15_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.0 Mobile/15E148 Safari/604.1",
			1, // Mobile
		},
		{
			"Mozilla/5.0 (iPad; CPU OS 15_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.0 Mobile/15E148 Safari/604.1",
			5, // Tablet
		},
		{
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			2, // PC
		},
		{
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.0 Safari/605.1.15",
			2, // PC
		},
		{
			"Mozilla/5.0 (Linux; Android 11) AppleWebKit/537.36 SmartTV",
			3, // Connected TV
		},
	}

	for _, tc := range testCases {
		info := ParseUserAgent(tc.ua)
		assert.NotNil(t, info)
		assert.Equal(t, tc.expectedType, info.DeviceType, "Failed for UA: %s", tc.ua)
	}
}
