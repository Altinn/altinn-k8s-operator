package maskinporten

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/altinn/altinn-k8s-operator/internal/crypto"
	"github.com/altinn/altinn-k8s-operator/internal/operatorcontext"
	"github.com/altinn/altinn-k8s-operator/test/utils"
	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/go-jose/go-jose/v4"
	"github.com/jonboulle/clockwork"
	. "github.com/onsi/gomega"
)

func TestPublicJwksConversion(t *testing.T) {
	g := NewWithT(t)

	jwks, err := createTestJwks()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(jwks).NotTo(BeNil())
	g.Expect(jwks.Keys[0].Certificates).NotTo(BeNil())

	publicJwks, err := getPublicOnlyJwks(jwks)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(publicJwks).NotTo(BeNil())
	// Certificates is marshalled as "x5c", which Maskinporten doens't want
	g.Expect(publicJwks.Keys[0].Certificates).To(BeNil())
	g.Expect(jwks.Keys[0].Certificates).NotTo(BeNil())

	json, err := json.Marshal(publicJwks)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(json).NotTo(BeNil())
	snaps.MatchJSON(t, json)

	// TODO: assert that private key fields are not present in JWK
}

func createTestJwks() (*jose.JSONWebKeySet, error) {
	operatorContext := operatorcontext.DiscoverOrDie(context.Background())
	clock := clockwork.NewFakeClockAt(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	random := utils.NewDeterministicRand()
	service := crypto.NewService(operatorContext, clock, random)

	appId := "app1"
	jwks, err := service.CreateJwks(appId)
	if err != nil {
		return nil, err
	}

	return jwks, nil
}
