package maskinporten

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/altinn/altinn-k8s-operator/internal/caching"
	"github.com/altinn/altinn-k8s-operator/internal/config"
	operatorcontext "github.com/altinn/altinn-k8s-operator/internal/operator_context"
	"github.com/jonboulle/clockwork"
	. "github.com/onsi/gomega"
)

func TestWellKnownConfig(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	operatorContext, err := operatorcontext.Discover()
	g.Expect(err).NotTo(HaveOccurred())

	cfg, err := config.GetConfig(operatorContext)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(cfg).NotTo(BeNil())

	apiClient, err := newApiClient(&cfg.MaskinportenApi, clock)
	g.Expect(err).NotTo(HaveOccurred())

	config1, err := apiClient.getWellKnownConfiguration(ctx)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(config1).NotTo(BeNil())

	config2, err := apiClient.getWellKnownConfiguration(ctx)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(config2).NotTo(BeNil())
	config3 := *config1
	g.Expect(config1).To(BeIdenticalTo(config2))     // Due to cache
	g.Expect(config1).ToNot(BeIdenticalTo(&config3)) // Copied above

	clock.Advance((5 + 1) * time.Minute) // Advance the clock past cache expiration

	config4, err := apiClient.getWellKnownConfiguration(ctx)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(config4).NotTo(BeNil())
	g.Expect(config1).ToNot(BeIdenticalTo(config4)) // Due to cache expiration
}

func TestCreateGrant(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	operatorContext, err := operatorcontext.Discover()
	g.Expect(err).NotTo(HaveOccurred())

	cfg, err := config.GetConfig(operatorContext)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(cfg).NotTo(BeNil())

	client, err := newApiClient(&cfg.MaskinportenApi, clock)
	g.Expect(err).NotTo(HaveOccurred())

	grant, err := client.createGrant(ctx)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(grant).NotTo(BeNil())
}

// Integration test
// func TestFetchAccessToken(t *testing.T) {
// 	g := NewWithT(t)
// 	ctx := context.Background()
// 	clock := clockwork.NewFakeClock()

// 	operatorContext, err := operatorcontext.Discover()
// 	g.Expect(err).NotTo(HaveOccurred())

// 	cfg, err := config.GetConfig(operatorContext)
// 	g.Expect(err).NotTo(HaveOccurred())
// 	g.Expect(cfg).NotTo(BeNil())

// 	client, err := newApiClient(&cfg.MaskinportenApi, clock)
// 	g.Expect(err).NotTo(HaveOccurred())

// 	token, err := client.accessTokenFetcher(ctx)
// 	g.Expect(err).NotTo(HaveOccurred())
// 	g.Expect(token.AccessToken).NotTo(BeNil())
// }

func TestFetchAccessTokenWithHTTPTest(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{"access_token":"mock_access_token","token_type":"Bearer","expires_in":3600}`))
		g.Expect(err).NotTo(HaveOccurred())
	}))
	defer server.Close()

	operatorContext, err := operatorcontext.Discover()
	g.Expect(err).NotTo(HaveOccurred())

	cfg, err := config.GetConfig(operatorContext)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(cfg).NotTo(BeNil())

	cfg.MaskinportenApi.Url = server.URL

	client, err := newApiClient(&cfg.MaskinportenApi, clock)
	g.Expect(err).NotTo(HaveOccurred())

	token, err := client.accessTokenFetcher(ctx)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(token.AccessToken).NotTo(BeNil())
}

func mockTokenRetriever(ctx context.Context) (*tokenResponse, error) {
	// Return a mock tokenResponse
	return &tokenResponse{AccessToken: "mock_access_token"}, nil
}

func TestCreateReq(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	client := &httpApiClient{
		// Setup mock for accessToken with a custom retriever function.
		// This Cached[tokenResponse] instance will return the mock token when Get is called.
		accessToken: caching.NewCachedAtom(time.Minute*5, clock, mockTokenRetriever),
	}

	var endpoint = "http://example.com/api/endpoint"

	req, err := client.createReq(ctx, endpoint)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(req).NotTo(BeNil())
	g.Expect(req.Method).To(Equal("POST"))
	g.Expect(req.URL.String()).To(Equal(endpoint))
	g.Expect(req.Header.Get("Content-Type")).To(Equal("application/x-www-form-urlencoded"))
	g.Expect(req.Header.Get("Authorization")).To(Equal("Bearer mock_access_token"))
}
