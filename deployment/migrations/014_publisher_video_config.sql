-- Migration 014: Per-publisher video inventory configuration
--
-- video_config stores the OpenRTB video object defaults for each publisher.
-- These are applied when the corresponding URL param is absent from the ad tag,
-- so a publisher tag only needs `pub=<id>` — all video specs are resolved server-side.
--
-- URL params always take precedence over the DB config.
--
-- Field reference (OpenRTB 2.5):
--   placement      1=instream, 3=in-article, 4=in-feed, 5=interstitial/slider
--   protocols      2=VAST2, 3=VAST3, 5=VAST2Wrapper, 6=VAST3Wrapper
--   playbackmethod 1=autoplay sound on, 2=autoplay sound off, 3=click-to-play
--   api            6=VPAID2JS, 7=OMID-1
--   mimes          MIME types accepted
--   maxdur / mindur  ad duration limits in seconds

ALTER TABLE publishers_new
    ADD COLUMN IF NOT EXISTS video_config JSONB;

-- Standard web instream config (sound off autoplay, VPAID 2.0 JS + OMID)
UPDATE publishers_new
SET video_config = '{
    "placement":      1,
    "protocols":      [2, 3, 5, 6],
    "playbackmethod": [2],
    "api":            [6, 7],
    "mimes":          ["video/mp4"],
    "maxdur":         30,
    "mindur":         5
}'::jsonb
WHERE video_config IS NULL;
