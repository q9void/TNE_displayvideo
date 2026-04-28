package exchange

// pbMutationT aliases the generated ARTF Mutation type so this package
// doesn't need to import the agentic/gen tree directly. The applier.Apply
// signature in the agentic package takes []*pbMutationT-equivalent input,
// and the agentic.DispatchResult.Mutations slice is exactly that type.
//
// We keep this alias in its own file so re-vendoring the protos doesn't
// touch any logic file.

import (
	mutationv1 "github.com/thenexusengine/tne_springwire/agentic/gen/iabtechlab/bidstream/mutation/v1"
)

type pbMutationT = mutationv1.Mutation
