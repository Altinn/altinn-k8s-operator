package crypto

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/altinn/altinn-k8s-operator/internal/operatorcontext"
	"github.com/altinn/altinn-k8s-operator/test/utils"
	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/go-jose/go-jose/v4"
	"github.com/jonboulle/clockwork"
	. "github.com/onsi/gomega"
)

const appId string = "app1"

func TestCreateJwks(t *testing.T) {
	g := NewWithT(t)

	// We use fixed inputs and make JWKS generation deterministic
	// to enable snapshot testing. It's important that we have control
	// over the outputs of this package and notice any changes across Go versions and library updates.

	jwks, _, _, err := createTestJwks()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(jwks).NotTo(BeNil())

	json, err := json.Marshal(jwks)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(json).NotTo(BeNil())
	snaps.MatchJSON(t, json)
}

func TestRotateJwks(t *testing.T) {
	g := NewWithT(t)

	jwks, service, clock, err := createTestJwks()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(jwks).NotTo(BeNil())

	// We have only just created the cert
	clock.Advance(time.Hour * 1)
	newJwks, err := service.RotateIfNeeded(appId, jwks)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(newJwks).To(BeNil())

	// This should be before the rotation threshold
	clock.Advance(time.Hour * 24 * 18)
	newJwks, err = service.RotateIfNeeded(appId, jwks)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(newJwks).To(BeNil())

	// Now we've advanced past the treshold and should have rotated
	clock.Advance(time.Hour * 24 * 7)
	newJwks, err = service.RotateIfNeeded(appId, jwks)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(newJwks).NotTo(BeNil())
	g.Expect(newJwks.Keys).To(HaveLen(2))
	oldCert := newJwks.Keys[1].Certificates[0]
	newCert := newJwks.Keys[0].Certificates[0]
	g.Expect(newCert.NotAfter.After(oldCert.NotAfter)).To(BeTrue())

	// We should rotate again
	clock.Advance(time.Hour * 24 * 25)
	newerJwks, err := service.RotateIfNeeded(appId, newJwks)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(newerJwks).NotTo(BeNil())
	g.Expect(newerJwks.Keys).To(HaveLen(2))
	newerCert := newerJwks.Keys[0].Certificates[0]
	g.Expect(newerJwks.Keys[1].Certificates[0]).To(BeIdenticalTo(newCert))
	g.Expect(newerCert.NotAfter.After(newCert.NotAfter)).To(BeTrue())

	// Serialize the new JWKS
	newJson, err := json.Marshal(newJwks)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(newJson).NotTo(BeNil())
	snaps.MatchJSON(t, newJson)

	newerJson, err := json.Marshal(newerJwks)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(newerJson).NotTo(BeNil())
	snaps.MatchJSON(t, newerJson)
}

func TestGenerateCertSerialNumber(t *testing.T) {
	g := NewWithT(t)

	service, _ := createService()

	serial, err := service.generateCertSerialNumber()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(serial.Sign()).ToNot(BeIdenticalTo(-1))
	g.Expect(serial.Bytes()).To(HaveLen(16))

	snaps.MatchSnapshot(t, serial.String())
}

func createService() (*CryptoService, clockwork.FakeClock) {
	operatorContext := operatorcontext.DiscoverOrDie(context.Background())
	clock := clockwork.NewFakeClockAt(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	random := utils.NewDeterministicRand()
	service := NewService(operatorContext, clock, random)
	return service, clock
}

func createTestJwks() (*jose.JSONWebKeySet, *CryptoService, clockwork.FakeClock, error) {
	service, clock := createService()

	jwks, err := service.CreateJwks(appId)
	if err != nil {
		return nil, nil, nil, err
	}

	return jwks, service, clock, nil
}
