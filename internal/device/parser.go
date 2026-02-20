// Package device provides User-Agent parsing for OpenRTB device information
package device

import (
	"regexp"
	"strings"

	"github.com/mssola/useragent"
)

// DeviceInfo contains parsed device details from User-Agent
type DeviceInfo struct {
	Make       string // Device manufacturer (Apple, Samsung, Google, etc.)
	Model      string // Device model (iPhone 14, Galaxy S23, etc.)
	OS         string // Operating system (iOS, Android, Windows, macOS)
	OSV        string // OS version (17.2, 13.0, 11, etc.)
	DeviceType int    // OpenRTB device type (1=Mobile, 2=PC, 3=TV, 5=Tablet)
}

// ParseUserAgent extracts device information from a User-Agent string
func ParseUserAgent(uaString string) *DeviceInfo {
	if uaString == "" {
		return nil
	}

	ua := useragent.New(uaString)
	info := &DeviceInfo{}

	// Parse OS and version
	info.OS, info.OSV = parseOS(ua, uaString)

	// Parse device make and model
	info.Make, info.Model = parseDevice(ua, uaString)

	// Determine device type
	info.DeviceType = parseDeviceType(ua, uaString)

	return info
}

// parseOS extracts operating system and version
func parseOS(ua *useragent.UserAgent, uaString string) (string, string) {
	osInfo := ua.OSInfo()
	os := osInfo.Name
	osv := osInfo.Version

	// Check UA string for specific patterns first (more reliable)
	uaLower := strings.ToLower(uaString)

	// iOS devices
	if strings.Contains(uaString, "iPhone") || strings.Contains(uaString, "iPad") {
		// Extract iOS version from UA string
		if match := regexp.MustCompile(`(?:iPhone )?OS (\d+(?:_\d+)*)`).FindStringSubmatch(uaString); len(match) > 1 {
			osv = strings.ReplaceAll(match[1], "_", ".")
		}
		return "iOS", osv
	}

	// Android
	if strings.Contains(uaLower, "android") {
		// Extract Android version
		if match := regexp.MustCompile(`Android (\d+(?:\.\d+)*)`).FindStringSubmatch(uaString); len(match) > 1 {
			osv = match[1]
		}
		return "Android", osv
	}

	// ChromeOS
	if strings.Contains(uaLower, "cros") {
		return "ChromeOS", osv
	}

	// Normalize OS names to common formats from library
	switch strings.ToLower(os) {
	case "iphone os", "ipad os", "ios":
		return "iOS", osv
	case "android":
		return "Android", osv
	case "windows", "windows nt":
		return "Windows", osv
	case "mac os x", "macos", "darwin":
		return "macOS", osv
	case "linux":
		return "Linux", osv
	case "chrome os", "chromeos":
		return "ChromeOS", osv
	default:
		// Return as-is for other OSes
		if os == "" {
			return "Unknown", ""
		}
		return os, osv
	}
}

// parseDevice extracts device manufacturer and model
func parseDevice(ua *useragent.UserAgent, uaString string) (string, string) {
	uaLower := strings.ToLower(uaString)

	// Check for ChromeOS first (before mobile check)
	if strings.Contains(uaLower, "cros") {
		return "Chromebook", ""
	}

	// Check for mobile devices
	if ua.Mobile() {
		// iOS devices
		if strings.Contains(uaString, "iPhone") {
			return "Apple", parseAppleModel(uaString, "iPhone")
		}
		if strings.Contains(uaString, "iPad") {
			return "Apple", parseAppleModel(uaString, "iPad")
		}

		// Android devices
		if strings.Contains(uaString, "Android") {
			return parseAndroidDevice(uaString)
		}

		// Generic mobile
		return "Mobile", ""
	}

	// Desktop/laptop devices
	// Mac
	if strings.Contains(uaString, "Macintosh") || strings.Contains(uaLower, "mac os") {
		return "Apple", "Mac"
	}

	// Windows
	if strings.Contains(uaLower, "windows") {
		return "PC", ""
	}

	// Linux
	if strings.Contains(uaLower, "linux") && !strings.Contains(uaLower, "android") {
		return "PC", ""
	}

	return "", ""
}

// parseAppleModel extracts iPhone or iPad model from UA
func parseAppleModel(uaString, deviceType string) string {
	// Look for model identifier like "iPhone14,2" or just "iPhone"
	re := regexp.MustCompile(deviceType + `\d+,\d+`)
	if match := re.FindString(uaString); match != "" {
		return match
	}

	// Fallback to device type only
	return deviceType
}

// parseAndroidDevice extracts Android device manufacturer and model
func parseAndroidDevice(uaString string) (string, string) {
	// Common patterns: "Samsung SM-G998B", "Pixel 7", "SM-S918B Build/"

	// Samsung devices
	if strings.Contains(uaString, "SAMSUNG") || strings.Contains(uaString, "SM-") {
		model := extractPattern(uaString, `SM-[A-Z0-9]+`)
		if model == "" {
			model = extractPattern(uaString, `SAMSUNG[- ]([A-Z0-9-]+)`)
		}
		return "Samsung", model
	}

	// Google Pixel
	if strings.Contains(uaString, "Pixel") {
		model := extractPattern(uaString, `Pixel( \d+)?( Pro)?( XL)?`)
		return "Google", model
	}

	// OnePlus
	if strings.Contains(uaString, "OnePlus") {
		model := extractPattern(uaString, `OnePlus[A-Z0-9 ]+`)
		return "OnePlus", model
	}

	// Xiaomi
	if strings.Contains(uaString, "Xiaomi") || strings.Contains(uaString, "Mi ") || strings.Contains(uaString, "Redmi") {
		make := "Xiaomi"
		model := extractPattern(uaString, `(Mi |Redmi |Poco )[A-Z0-9 ]+`)
		if model == "" {
			model = extractPattern(uaString, `Xiaomi[A-Z0-9 ]+`)
		}
		return make, model
	}

	// Huawei
	if strings.Contains(uaString, "Huawei") || strings.Contains(uaString, "HUAWEI") {
		model := extractPattern(uaString, `HUAWEI[- ]?[A-Z0-9-]+`)
		return "Huawei", model
	}

	// Oppo
	if strings.Contains(uaString, "OPPO") {
		model := extractPattern(uaString, `OPPO[- ]?[A-Z0-9]+`)
		return "Oppo", model
	}

	// Vivo
	if strings.Contains(uaString, "vivo") {
		model := extractPattern(uaString, `vivo[- ]?[A-Z0-9]+`)
		return "Vivo", model
	}

	// LG
	if strings.Contains(uaString, "LG-") {
		model := extractPattern(uaString, `LG-[A-Z0-9]+`)
		return "LG", model
	}

	// Motorola
	if strings.Contains(uaString, "Moto") {
		model := extractPattern(uaString, `Moto[A-Z0-9 ]+`)
		return "Motorola", model
	}

	// Generic Android device
	// Try to extract Build/model
	model := extractPattern(uaString, `Build/[A-Z0-9]+`)
	if model != "" {
		model = strings.TrimPrefix(model, "Build/")
		return "Android", model
	}

	return "Android", ""
}

// parseDeviceType determines OpenRTB device type code
func parseDeviceType(ua *useragent.UserAgent, uaString string) int {
	// Check for TV/CTV
	if strings.Contains(strings.ToLower(uaString), "tv") ||
		strings.Contains(strings.ToLower(uaString), "smarttv") ||
		strings.Contains(strings.ToLower(uaString), "appletv") {
		return 3 // Connected TV
	}

	// Check for tablet
	if ua.Mobile() && (strings.Contains(uaString, "iPad") ||
		strings.Contains(strings.ToLower(uaString), "tablet")) {
		return 5 // Tablet
	}

	// Check for mobile/phone
	if ua.Mobile() {
		return 1 // Mobile/Phone
	}

	// Desktop/PC (default)
	return 2 // Personal Computer
}

// extractPattern extracts the first match of a regex pattern
func extractPattern(text, pattern string) string {
	re := regexp.MustCompile(pattern)
	match := re.FindString(text)
	return strings.TrimSpace(match)
}
