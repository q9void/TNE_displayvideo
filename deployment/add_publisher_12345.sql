-- Add Publisher ID '12345' for TotalProSports
-- This publisher is using accountId '12345' (yes, it looks like a test ID, but it's intentional)

-- First, check if publisher already exists
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM publishers WHERE publisher_id = '12345') THEN
        RAISE NOTICE 'Publisher 12345 already exists. Use UPDATE instead of INSERT.';
    ELSE
        -- Insert the publisher with bidder configuration
        INSERT INTO publishers (
            id,
            publisher_id,
            name,
            allowed_domains,
            bidder_params,
            bid_multiplier,
            status,
            created_at,
            updated_at,
            notes
        ) VALUES (
            gen_random_uuid(),
            '12345',
            'Total Pro Sports',
            'totalprosports.com,dev.totalprosports.com,*.totalprosports.com',
            '{
                "rubicon": {
                    "accountId": 26298,
                    "siteId": 556630,
                    "zoneId": 3767186,
                    "bidonmultiformat": false
                },
                "kargo": {
                    "placementId": "_o9n8eh8Lsw"
                },
                "sovrn": {
                    "tagid": "1294952"
                },
                "pubmatic": {
                    "publisherId": "166938",
                    "adSlot": "7079290"
                },
                "triplelift": {
                    "inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"
                }
            }'::jsonb,
            1.0,
            'active',
            CURRENT_TIMESTAMP,
            CURRENT_TIMESTAMP,
            'TotalProSports publisher using accountId 12345 (intentional, will update when going live)'
        );

        RAISE NOTICE 'Publisher 12345 added successfully';
    END IF;
END $$;

-- Verify the insert
SELECT
    publisher_id,
    name,
    allowed_domains,
    bidder_params,
    status
FROM publishers
WHERE publisher_id = '12345';
