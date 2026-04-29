package inbound

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

func TestDevAuthenticator_acceptsAllowListed(t *testing.T) {
	auth := NewDevAuthenticator([]AgentEntry{
		{AgentID: "curator.example.com", AgentType: "DSP", AuthorizedDeals: []string{"deal-1", "deal-2"}},
	})
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-aamp-agent-id", "curator.example.com"))

	id, err := auth.Verify(ctx)
	require.NoError(t, err)
	assert.Equal(t, "curator.example.com", id.AgentID)
	assert.Equal(t, "DSP", id.AgentType)
	assert.True(t, id.RegistryVerified) // DevAuth sets unconditionally
	assert.ElementsMatch(t, []string{"deal-1", "deal-2"}, id.AuthorizedDeals)
}

func TestDevAuthenticator_rejectsUnknown(t *testing.T) {
	auth := NewDevAuthenticator(nil)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-aamp-agent-id", "stranger"))

	_, err := auth.Verify(ctx)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrAuthFailed))
}

func TestDevAuthenticator_rejectsMissingHeader(t *testing.T) {
	auth := NewDevAuthenticator([]AgentEntry{{AgentID: "curator.example.com"}})

	// No metadata at all.
	_, err := auth.Verify(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrAuthFailed))
}

func TestDevAuthenticator_rejectsEmptyHeader(t *testing.T) {
	auth := NewDevAuthenticator([]AgentEntry{{AgentID: "curator.example.com"}})
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-aamp-agent-id", "  "))

	_, err := auth.Verify(ctx)
	require.Error(t, err)
}

func TestDevAuthenticator_caseInsensitiveMetadataKey(t *testing.T) {
	// gRPC metadata keys are normalised to lowercase by grpc-go; we exercise
	// the path with a mixed-case input to confirm behavior.
	auth := NewDevAuthenticator([]AgentEntry{{AgentID: "curator.example.com", AgentType: "DSP"}})
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("X-AAMP-Agent-ID", "curator.example.com"))

	id, err := auth.Verify(ctx)
	require.NoError(t, err)
	assert.Equal(t, "curator.example.com", id.AgentID)
}

func TestDevAuthenticator_defaultAgentTypeIsDSP(t *testing.T) {
	auth := NewDevAuthenticator([]AgentEntry{{AgentID: "curator.example.com"}}) // AgentType omitted
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-aamp-agent-id", "curator.example.com"))

	id, err := auth.Verify(ctx)
	require.NoError(t, err)
	assert.Equal(t, "DSP", id.AgentType)
}

func TestDevAuthenticator_AddOverridesEntry(t *testing.T) {
	auth := NewDevAuthenticator([]AgentEntry{{AgentID: "x", AgentType: "DSP"}})
	auth.Add(AgentEntry{AgentID: "x", AgentType: "PUBLISHER"})
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-aamp-agent-id", "x"))

	id, err := auth.Verify(ctx)
	require.NoError(t, err)
	assert.Equal(t, "PUBLISHER", id.AgentType)
}

func TestDevAuthenticator_RefreshRegistryNoOp(t *testing.T) {
	auth := NewDevAuthenticator(nil)
	assert.NoError(t, auth.RefreshRegistry(context.Background()))
}

func TestAgentIdentity_IsAuthorizedForDeal(t *testing.T) {
	id := AgentIdentity{AuthorizedDeals: []string{"a", "b"}}
	assert.True(t, id.IsAuthorizedForDeal("a"))
	assert.True(t, id.IsAuthorizedForDeal("b"))
	assert.False(t, id.IsAuthorizedForDeal("c"))

	idEmpty := AgentIdentity{}
	assert.False(t, idEmpty.IsAuthorizedForDeal("a"))
}
