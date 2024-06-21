package maskinporten

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/altinn/altinn-k8s-operator/internal/caching"
	"github.com/altinn/altinn-k8s-operator/internal/config"
	"github.com/altinn/altinn-k8s-operator/internal/telemetry"
	"github.com/cenkalti/backoff/v4"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type wellKnownResponse struct {
	Issuer                                     string   `json:"issuer"`
	TokenEndpoint                              string   `json:"token_endpoint"`
	JwksURI                                    string   `json:"jwks_uri"`
	TokenEndpointAuthMethodsSupported          []string `json:"token_endpoint_auth_methods_supported"`
	GrantTypesSupported                        []string `json:"grant_types_supported"`
	TokenEndpointAuthSigningAlgValuesSupported []string `json:"token_endpoint_auth_signing_alg_values_supported"`
}

type httpApiClient struct {
	config      *config.MaskinportenApiConfig
	client      http.Client
	jwk         jose.JSONWebKey
	wellKnown   caching.CachedAtom[wellKnownResponse]
	accessToken caching.CachedAtom[tokenResponse]
	tracer      trace.Tracer
}

// Docs:
// - https://docs.digdir.no/docs/Maskinporten/maskinporten_guide_apikonsument
// - https://docs.digdir.no/docs/Maskinporten/maskinporten_protocol_token
// - https://docs.digdir.no/docs/Maskinporten/maskinporten_func_wellknown

func newApiClient(config *config.MaskinportenApiConfig, clock clockwork.Clock) (*httpApiClient, error) {
	jwk := jose.JSONWebKey{}
	if err := json.Unmarshal([]byte(config.Jwk), &jwk); err != nil {
		return nil, err
	}

	client := &httpApiClient{
		config: config,
		client: http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)},
		jwk:    jwk,
		tracer: otel.Tracer(telemetry.ServiceName),
	}

	client.wellKnown = caching.NewCachedAtom(5*time.Minute, clock, client.wellKnownFetcher)
	client.accessToken = caching.NewCachedAtom(1*time.Minute, clock, client.accessTokenFetcher)

	return client, nil
}

func (c *httpApiClient) createReq(ctx context.Context, endpoint string) (*http.Request, error) {
	// Fetch the access token from the cache.
	tokenResponse, err := c.accessToken.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	// Prepare the request.
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create new request: %w", err)
	}

	// Set necessary headers.
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+tokenResponse.AccessToken)

	return req, nil
}

func (c *httpApiClient) getWellKnownConfiguration(ctx context.Context) (*wellKnownResponse, error) {
	ctx, span := c.tracer.Start(ctx, "GetWellKnownConfiguration")
	defer span.End()

	return c.wellKnown.Get(ctx)
}

func (c *httpApiClient) createGrant(ctx context.Context) (*string, error) {
	wellKnown, err := c.wellKnown.Get(ctx)
	if err != nil {
		return nil, err
	}

	exp := time.Now().Add(60 * time.Second)
	issuedAt := time.Now()

	pubClaims := jwt.Claims{
		Audience:  []string{wellKnown.Issuer},
		Issuer:    c.config.ClientId,
		IssuedAt:  jwt.NewNumericDate(issuedAt),
		NotBefore: jwt.NewNumericDate(issuedAt),
		Expiry:    jwt.NewNumericDate(exp),
		ID:        uuid.New().String(),
	}

	privClaims := struct {
		Scope string `json:"scope"`
	}{
		Scope: c.config.Scope,
	}

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: c.jwk},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	if err != nil {
		return nil, err
	}

	signedToken, err := jwt.Signed(signer).Claims(pubClaims).Claims(privClaims).Serialize()
	if err != nil {
		return nil, err
	}

	return &signedToken, nil
}

func (c *httpApiClient) accessTokenFetcher(ctx context.Context) (*tokenResponse, error) {
	grant, err := c.createGrant(ctx)
	if err != nil {
		return nil, err
	}

	endpoint, err := url.JoinPath(c.config.Url, "/token")
	if err != nil {
		return nil, err
	}

	urlEncodedContent := url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {*grant},
	}

	endpoint += "?" + urlEncodedContent.Encode()

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.retryableHTTPDo(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	tokenResp, err := deserialize[tokenResponse](resp)
	if err != nil {
		return nil, err
	}

	return &tokenResp, nil
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
}

func (c *httpApiClient) wellKnownFetcher(ctx context.Context) (*wellKnownResponse, error) {
	endpoint, err := url.JoinPath(c.config.Url, "/.well-known/oauth-authorization-server")
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	wellKnownResp, err := deserialize[wellKnownResponse](resp)
	if err != nil {
		return nil, err
	}
	return &wellKnownResp, nil
}

func deserialize[T any](resp *http.Response) (T, error) {
	defer func() { _ = resp.Body.Close() }()

	var result T
	err := json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return result, err
	}

	return result, err
}

// retryableHTTPDo performs an HTTP request with retry logic.
func (c *httpApiClient) retryableHTTPDo(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	operation := func() error {
		resp, err = c.client.Do(req)
		if err != nil {
			return err // Network error, retry.
		}
		if resp.StatusCode >= 500 { // Retrying on 5xx server errors.
			return fmt.Errorf("server error: %v", resp.Status)
		}
		return nil // No retry needed - success or client side error
	}

	backoffStrategy := backoff.NewExponentialBackOff()
	// Default setting is to 1.5x the time interval for every failure
	backoffStrategy.InitialInterval = 1 * time.Second
	backoffStrategy.MaxInterval = 30 * time.Second
	backoffStrategy.MaxElapsedTime = 2 * time.Minute

	err = backoff.Retry(operation, backoffStrategy)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
