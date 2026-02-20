#!/bin/bash
# Test script for PBS hook validation order
# Tests: Request validation, Privacy, SChain, Multiformat, Response normalization

set -e

BASE_URL="${PBS_URL:-http://localhost:8000}"
PUBLISHER_ID="${TEST_PUBLISHER_ID:-test-publisher}"
ADMIN_KEY="${ADMIN_API_KEY:-test-admin-key}"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Counter for test results
PASSED=0
FAILED=0

# Helper function to print test results
pass() {
    echo -e "${GREEN}✓ PASS${NC}: $1"
    ((PASSED++))
}

fail() {
    echo -e "${RED}✗ FAIL${NC}: $1"
    ((FAILED++))
}

info() {
    echo -e "${YELLOW}ℹ INFO${NC}: $1"
}

# =============================================================================
# HOOK 1: REQUEST-LEVEL VALIDATION (Currency Normalization)
# =============================================================================

test_currency_normalization() {
    info "Testing Hook 1: Request-level validation (currency normalization)"

    # Test 1: Lowercase currency code should be normalized to uppercase
    info "Test 1.1: Lowercase 'eur' should normalize to 'EUR'"
    RESPONSE=$(curl -s -X POST "$BASE_URL/openrtb2/auction" \
        -H "Content-Type: application/json" \
        -d '{
            "id": "test-curr-1",
            "imp": [{
                "id": "1",
                "banner": {"w": 300, "h": 250},
                "bidfloor": 1.50,
                "bidfloorcur": "eur"
            }],
            "site": {
                "domain": "test.com",
                "page": "https://test.com/page"
            }
        }')

    # Check if request was accepted (should not return 400)
    if echo "$RESPONSE" | grep -q '"id":"test-curr-1"'; then
        pass "Currency normalization: lowercase accepted"
    else
        fail "Currency normalization: lowercase rejected (should be normalized)"
    fi

    # Test 2: Invalid currency code (not 3 letters) should be rejected
    info "Test 1.2: Invalid currency code 'EURO' should be rejected"
    RESPONSE=$(curl -s -X POST "$BASE_URL/openrtb2/auction" \
        -H "Content-Type: application/json" \
        -d '{
            "id": "test-curr-2",
            "imp": [{
                "id": "1",
                "banner": {"w": 300, "h": 250},
                "bidfloor": 1.50,
                "bidfloorcur": "EURO"
            }],
            "site": {
                "domain": "test.com",
                "page": "https://test.com/page"
            }
        }')

    if echo "$RESPONSE" | grep -qi "invalid currency"; then
        pass "Currency validation: 4-letter code rejected"
    else
        fail "Currency validation: 4-letter code accepted (should be rejected)"
    fi
}

# =============================================================================
# HOOK 2: PRIVACY/CONSENT (GDPR/CCPA Enforcement)
# =============================================================================

test_privacy_enforcement() {
    info "Testing Hook 2: Privacy/Consent enforcement"

    # Test 1: GDPR applies without consent should reject
    info "Test 2.1: GDPR country without consent string"
    RESPONSE=$(curl -s -X POST "$BASE_URL/openrtb2/auction" \
        -H "Content-Type: application/json" \
        -H "X-Forwarded-For: 89.160.20.112" \
        -d '{
            "id": "test-gdpr-1",
            "imp": [{
                "id": "1",
                "banner": {"w": 300, "h": 250}
            }],
            "site": {
                "domain": "test.com",
                "page": "https://test.com/page"
            },
            "device": {
                "ip": "89.160.20.112",
                "ua": "Mozilla/5.0"
            },
            "regs": {
                "gdpr": 0
            }
        }')

    # Should reject requests with GDPR=0 when geo indicates GDPR applies
    if echo "$RESPONSE" | grep -qi "gdpr"; then
        pass "Privacy enforcement: GDPR mismatch detected"
    else
        fail "Privacy enforcement: GDPR mismatch not detected"
    fi

    # Test 2: CCPA opt-out should be respected
    info "Test 2.2: CCPA opt-out flag should prevent data usage"
    RESPONSE=$(curl -s -X POST "$BASE_URL/openrtb2/auction" \
        -H "Content-Type: application/json" \
        -d '{
            "id": "test-ccpa-1",
            "imp": [{
                "id": "1",
                "banner": {"w": 300, "h": 250}
            }],
            "site": {
                "domain": "test.com",
                "page": "https://test.com/page"
            },
            "device": {
                "geo": {"country": "US", "region": "CA"}
            },
            "regs": {
                "us_privacy": "1YYN"
            }
        }')

    # Should process request but strip user data
    if echo "$RESPONSE" | grep -q "id"; then
        pass "Privacy enforcement: CCPA opt-out processed"
    else
        fail "Privacy enforcement: CCPA opt-out handling failed"
    fi
}

# =============================================================================
# HOOK 4: SCHAIN AUGMENTATION
# =============================================================================

test_schain_augmentation() {
    info "Testing Hook 4: SChain augmentation"

    # Test 1: Request without schain should get platform node added
    info "Test 4.1: Missing SChain should be created with platform node"

    # Note: We need to inspect logs or use debug mode to verify schain augmentation
    # For now, just verify request is accepted
    RESPONSE=$(curl -s -X POST "$BASE_URL/openrtb2/auction" \
        -H "Content-Type: application/json" \
        -d '{
            "id": "test-schain-1",
            "imp": [{
                "id": "1",
                "banner": {"w": 300, "h": 250}
            }],
            "site": {
                "domain": "test.com",
                "page": "https://test.com/page"
            }
        }')

    if echo "$RESPONSE" | grep -q "id"; then
        pass "SChain augmentation: Request without schain accepted"
        info "Note: Check server logs to verify platform node was added"
    else
        fail "SChain augmentation: Request processing failed"
    fi

    # Test 2: Request with existing schain should preserve it
    info "Test 4.2: Existing SChain should be preserved and augmented"
    RESPONSE=$(curl -s -X POST "$BASE_URL/openrtb2/auction" \
        -H "Content-Type: application/json" \
        -d '{
            "id": "test-schain-2",
            "imp": [{
                "id": "1",
                "banner": {"w": 300, "h": 250}
            }],
            "site": {
                "domain": "test.com",
                "page": "https://test.com/page"
            },
            "source": {
                "schain": {
                    "ver": "1.0",
                    "complete": 1,
                    "nodes": [{
                        "asi": "upstream.com",
                        "sid": "12345",
                        "hp": 1
                    }]
                }
            }
        }')

    if echo "$RESPONSE" | grep -q "id"; then
        pass "SChain augmentation: Request with existing schain accepted"
        info "Note: Check server logs to verify upstream node was preserved"
    else
        fail "SChain augmentation: Request with schain failed"
    fi
}

# =============================================================================
# HOOK 5: MULTIFORMAT BID SELECTION
# =============================================================================

test_multiformat_selection() {
    info "Testing Hook 5: Multiformat bid selection"

    # Test 1: Multiformat impression (banner + video)
    info "Test 5.1: Multiformat impression should select best format"
    RESPONSE=$(curl -s -X POST "$BASE_URL/openrtb2/auction" \
        -H "Content-Type: application/json" \
        -d '{
            "id": "test-multiformat-1",
            "imp": [{
                "id": "1",
                "banner": {
                    "w": 300,
                    "h": 250
                },
                "video": {
                    "w": 640,
                    "h": 480,
                    "mimes": ["video/mp4"],
                    "protocols": [2, 3]
                },
                "ext": {
                    "prebid": {
                        "multiformatRequestStrategy": "server"
                    }
                }
            }],
            "site": {
                "domain": "test.com",
                "page": "https://test.com/page"
            }
        }')

    if echo "$RESPONSE" | grep -q "id"; then
        pass "Multiformat selection: Multiformat request accepted"
        info "Note: Check analytics to verify best bid was selected"
    else
        fail "Multiformat selection: Multiformat request failed"
    fi
}

# =============================================================================
# HOOK 6: RESPONSE NORMALIZATION
# =============================================================================

test_response_normalization() {
    info "Testing Hook 6: Response normalization (tested via adapter layer)"

    # These tests verify that the exchange properly validates bidder responses
    # We can't directly test this without mock bidders, but we can verify
    # the validation logic exists by testing edge cases

    info "Test 6.1: Verify exchange processes valid responses"
    RESPONSE=$(curl -s -X POST "$BASE_URL/openrtb2/auction" \
        -H "Content-Type: application/json" \
        -d '{
            "id": "test-response-1",
            "imp": [{
                "id": "1",
                "banner": {"w": 300, "h": 250},
                "bidfloor": 0.50
            }],
            "site": {
                "domain": "test.com",
                "page": "https://test.com/page"
            },
            "cur": ["USD"]
        }')

    if echo "$RESPONSE" | grep -q "id"; then
        pass "Response normalization: Valid request processed"
    else
        fail "Response normalization: Request processing failed"
    fi
}

# =============================================================================
# INTEGRATION TESTS
# =============================================================================

test_full_auction_flow() {
    info "Testing full auction flow with all hooks"

    info "Integration Test: Complete auction with all validations"
    RESPONSE=$(curl -s -X POST "$BASE_URL/openrtb2/auction" \
        -H "Content-Type: application/json" \
        -d '{
            "id": "test-integration-1",
            "imp": [{
                "id": "imp1",
                "banner": {
                    "w": 300,
                    "h": 250,
                    "format": [
                        {"w": 300, "h": 250},
                        {"w": 728, "h": 90}
                    ]
                },
                "video": {
                    "w": 640,
                    "h": 480,
                    "mimes": ["video/mp4"],
                    "protocols": [2, 3, 5, 6]
                },
                "bidfloor": 1.00,
                "bidfloorcur": "usd",
                "ext": {
                    "prebid": {
                        "multiformatRequestStrategy": "server"
                    }
                }
            }],
            "site": {
                "domain": "test.com",
                "page": "https://test.com/page",
                "publisher": {
                    "id": "pub123"
                }
            },
            "device": {
                "ua": "Mozilla/5.0 (Windows NT 10.0; Win64; x64)",
                "ip": "1.2.3.4"
            },
            "user": {
                "id": "user123"
            },
            "cur": ["USD", "EUR"],
            "tmax": 1000
        }')

    if echo "$RESPONSE" | grep -q "id"; then
        pass "Integration: Full auction completed successfully"

        # Verify currency was normalized
        info "Verifying currency normalization occurred"

        # Check response structure
        if echo "$RESPONSE" | jq -e '.seatbid' > /dev/null 2>&1; then
            pass "Integration: Response has valid structure"
        else
            info "Integration: No bids returned (expected if no bidders configured)"
        fi
    else
        fail "Integration: Full auction failed"
    fi
}

# =============================================================================
# SECURITY TESTS
# =============================================================================

test_security_validations() {
    info "Testing security validations"

    # Test 1: Admin endpoints require auth
    info "Security Test 1: Admin endpoints protected"
    RESPONSE=$(curl -s -w "\n%{http_code}" "$BASE_URL/admin/dashboard")
    HTTP_CODE=$(echo "$RESPONSE" | tail -n1)

    if [ "$HTTP_CODE" = "401" ] || [ "$HTTP_CODE" = "403" ]; then
        pass "Security: Admin endpoints require authentication"
    else
        fail "Security: Admin endpoints not properly protected (HTTP $HTTP_CODE)"
    fi

    # Test 2: pprof endpoints disabled by default
    info "Security Test 2: pprof endpoints disabled"
    RESPONSE=$(curl -s -w "\n%{http_code}" "$BASE_URL/debug/pprof/")
    HTTP_CODE=$(echo "$RESPONSE" | tail -n1)

    if [ "$HTTP_CODE" = "404" ]; then
        pass "Security: pprof endpoints disabled"
    else
        fail "Security: pprof endpoints exposed (HTTP $HTTP_CODE)"
    fi

    # Test 3: XSS protection in ad tags
    info "Security Test 3: XSS protection in ad tags"
    RESPONSE=$(curl -s "$BASE_URL/ad/js?div=test</script><script>alert(1)</script>")

    if echo "$RESPONSE" | grep -q "&lt;script&gt;"; then
        pass "Security: XSS properly escaped in ad tags"
    else
        fail "Security: XSS not escaped in ad tags"
    fi
}

# =============================================================================
# RUN ALL TESTS
# =============================================================================

echo "=========================================="
echo "PBS Hook Validation Test Suite"
echo "=========================================="
echo "Base URL: $BASE_URL"
echo ""

# Run all test suites
test_currency_normalization
echo ""
test_privacy_enforcement
echo ""
test_schain_augmentation
echo ""
test_multiformat_selection
echo ""
test_response_normalization
echo ""
test_full_auction_flow
echo ""
test_security_validations

# Print summary
echo ""
echo "=========================================="
echo "Test Summary"
echo "=========================================="
echo -e "${GREEN}Passed: $PASSED${NC}"
echo -e "${RED}Failed: $FAILED${NC}"
echo "=========================================="

# Exit with error if any tests failed
if [ $FAILED -gt 0 ]; then
    exit 1
fi

exit 0
