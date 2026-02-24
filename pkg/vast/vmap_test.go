package vast

import (
	"strings"
	"testing"
)

func TestNewVMAP(t *testing.T) {
	v := NewVMAP()
	if v.Version != "1.0" {
		t.Errorf("expected version 1.0, got %s", v.Version)
	}
	if v.XMLNS != VMAPNamespace {
		t.Errorf("expected xmlns %s, got %s", VMAPNamespace, v.XMLNS)
	}
	if len(v.AdBreaks) != 0 {
		t.Errorf("expected empty AdBreaks, got %d", len(v.AdBreaks))
	}
}

func TestAddAdTagBreak(t *testing.T) {
	v := NewVMAP()
	v.AddAdTagBreak("start", "preroll", "https://ads.example.com/vast?id=1&w=1920", false, "")

	if len(v.AdBreaks) != 1 {
		t.Fatalf("expected 1 AdBreak, got %d", len(v.AdBreaks))
	}
	ab := v.AdBreaks[0]
	if ab.TimeOffset != "start" {
		t.Errorf("expected timeOffset 'start', got %s", ab.TimeOffset)
	}
	if ab.BreakID != "preroll" {
		t.Errorf("expected breakId 'preroll', got %s", ab.BreakID)
	}
	if ab.BreakType != BreakTypeLinear {
		t.Errorf("expected breakType 'linear', got %s", ab.BreakType)
	}
	if ab.AdSource == nil {
		t.Fatal("expected AdSource, got nil")
	}
	if ab.AdSource.AdTagURI == nil {
		t.Fatal("expected AdTagURI, got nil")
	}
	if ab.AdSource.AdTagURI.Value != "https://ads.example.com/vast?id=1&w=1920" {
		t.Errorf("unexpected AdTagURI value: %s", ab.AdSource.AdTagURI.Value)
	}
	if ab.TrackingEvents != nil {
		t.Error("expected no TrackingEvents when trackingBaseURL is empty")
	}
}

func TestAddAdTagBreak_WithTracking(t *testing.T) {
	v := NewVMAP()
	v.AddAdTagBreak("end", "postroll", "https://ads.example.com/vast", false, "https://ads.example.com/video/event")

	ab := v.AdBreaks[0]
	if ab.TrackingEvents == nil {
		t.Fatal("expected TrackingEvents, got nil")
	}
	if len(ab.TrackingEvents.Tracking) != 2 {
		t.Fatalf("expected 2 tracking events, got %d", len(ab.TrackingEvents.Tracking))
	}
	events := map[string]string{}
	for _, tr := range ab.TrackingEvents.Tracking {
		events[tr.Event] = tr.Value
	}
	if _, ok := events[VMAPEventBreakStart]; !ok {
		t.Error("missing breakStart tracking event")
	}
	if _, ok := events[VMAPEventBreakEnd]; !ok {
		t.Error("missing breakEnd tracking event")
	}
}

func TestAddAdTagBreak_AllowMultiple(t *testing.T) {
	v := NewVMAP()
	v.AddAdTagBreak("00:05:00", "midroll-1", "https://ads.example.com/vast", true, "")
	if !v.AdBreaks[0].AdSource.AllowMultipleAds {
		t.Error("expected AllowMultipleAds=true")
	}
}

func TestVMAPMarshal_CDATAPreservesAmpersand(t *testing.T) {
	v := NewVMAP()
	// URL with multiple query params — & must NOT become &amp; in output
	v.AddAdTagBreak("start", "preroll", "https://ads.example.com/vast?id=abc&w=1920&h=1080", false, "")

	data, err := v.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	xml := string(data)

	// The & in the URL must be preserved as-is inside CDATA
	if strings.Contains(xml, "&amp;") {
		t.Error("AdTagURI contains &amp; — URL is not CDATA-wrapped correctly")
	}
	if !strings.Contains(xml, "id=abc&w=1920&h=1080") {
		t.Error("AdTagURI value not found intact in marshaled XML")
	}
}

func TestVMAPMarshal_Structure(t *testing.T) {
	v := NewVMAP()
	v.AddAdTagBreak("start", "preroll", "https://ads.example.com/vast?id=1", false, "")
	v.AddAdTagBreak("00:05:00", "midroll-1", "https://ads.example.com/vast?id=2", true, "")
	v.AddAdTagBreak("end", "postroll", "https://ads.example.com/vast?id=3", false, "")

	data, err := v.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	xml := string(data)

	checks := []string{
		`vmap:VMAP`,
		`version="1.0"`,
		`xmlns:vmap="http://www.iab.net/videosuite/vmap"`,
		`vmap:AdBreak`,
		`timeOffset="start"`,
		`breakId="preroll"`,
		`timeOffset="00:05:00"`,
		`breakId="midroll-1"`,
		`timeOffset="end"`,
		`breakId="postroll"`,
		`vmap:AdTagURI`,
		`templateType="vast4"`,
		`breakType="linear"`,
	}
	for _, check := range checks {
		if !strings.Contains(xml, check) {
			t.Errorf("marshaled VMAP missing expected content: %q", check)
		}
	}
}

func TestVMAPMarshal_XMLDeclaration(t *testing.T) {
	v := NewVMAP()
	data, err := v.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if !strings.HasPrefix(string(data), "<?xml version=") {
		t.Error("VMAP output missing XML declaration")
	}
}
