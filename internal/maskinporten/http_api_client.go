package maskinporten

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-errors/errors"

	"github.com/altinn/altinn-k8s-operator/internal/caching"
	"github.com/altinn/altinn-k8s-operator/internal/config"
	"github.com/altinn/altinn-k8s-operator/internal/operatorcontext"
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

type WellKnownResponse struct {
	Issuer                                     string   `json:"issuer"`
	TokenEndpoint                              string   `json:"token_endpoint"`
	JwksURI                                    string   `json:"jwks_uri"`
	TokenEndpointAuthMethodsSupported          []string `json:"token_endpoint_auth_methods_supported"`
	GrantTypesSupported                        []string `json:"grant_types_supported"`
	TokenEndpointAuthSigningAlgValuesSupported []string `json:"token_endpoint_auth_signing_alg_values_supported"`
}

// This client calls necessary APIs in the Maskinporten authority service
// and the self-service APIs to manage clients. It also makes sure to enforce
// rules related to naming/scoping according to the passed in config.
//
// Docs:
//   - https://docs.digdir.no/docs/Maskinporten/maskinporten_guide_apikonsument
//   - https://docs.digdir.no/docs/Maskinporten/maskinporten_protocol_token
//   - https://docs.digdir.no/docs/Maskinporten/maskinporten_func_wellknown
//   - Dev self service API: https://api.samarbeid.digdir.dev/swagger-ui/index.html
//   - Dev auth/token API: https://maskinporten.dev
type HttpApiClient struct {
	config      *config.MaskinportenApiConfig
	context     *operatorcontext.Context
	client      http.Client
	jwk         jose.JSONWebKey
	hydrated    bool
	wellKnown   caching.CachedAtom[WellKnownResponse]
	accessToken caching.CachedAtom[TokenResponse]
	tracer      trace.Tracer

	clientNamePrefix string
}

func NewHttpApiClient(
	config *config.MaskinportenApiConfig,
	context *operatorcontext.Context,
	clock clockwork.Clock,
) (*HttpApiClient, error) {
	jwk := jose.JSONWebKey{}
	if err := json.Unmarshal([]byte(config.Jwk), &jwk); err != nil {
		return nil, err
	}

	client := &HttpApiClient{
		config:   config,
		context:  context,
		client:   http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)},
		jwk:      jwk,
		hydrated: false,
		tracer:   otel.Tracer(telemetry.ServiceName),

		clientNamePrefix: getClientNamePrefix(context),
	}

	client.wellKnown = caching.NewCachedAtom(5*time.Minute, clock, client.wellKnownFetcher)
	client.accessToken = caching.NewCachedAtom(1*time.Minute, clock, client.accessTokenFetcher)

	return client, nil
}

func (c *HttpApiClient) createReq(
	ctx context.Context,
	url string,
	method string,
	body io.Reader,
) (*http.Request, error) {
	// Fetch the access token from the cache.
	tokenResponse, err := c.accessToken.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	// Prepare the request.
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create new request: %w", err)
	}

	// Set necessary headers.
	req.Header.Set("Authorization", "Bearer "+tokenResponse.AccessToken)

	return req, nil
}

func (c *HttpApiClient) GetWellKnownConfiguration(ctx context.Context) (*WellKnownResponse, error) {
	ctx, span := c.tracer.Start(ctx, "GetWellKnownConfiguration")
	defer span.End()

	return c.wellKnown.Get(ctx)
}

func (c *HttpApiClient) GetAllClients(ctx context.Context) ([]OidcClientResponse, error) {
	ctx, span := c.tracer.Start(ctx, "GetAllClients")
	defer span.End()

	url, err := url.JoinPath(c.config.SelfServiceUrl, "/clients")

	if err != nil {
		return nil, err
	}
	req, err := c.createReq(ctx, url, "GET", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	resp, err := c.retryableHTTPDo(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		defer func() { _ = resp.Body.Close() }()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, errors.WrapPrefix(err, "error reading body on unexpected status", 0)
		}
		return nil, fmt.Errorf("unexpected status code: %d, body:\n%s", resp.StatusCode, body)
	}

	dtos, err := deserialize[[]OidcClientResponse](resp)
	if err != nil {
		return nil, err
	}

	if dtos == nil {
		return nil, fmt.Errorf("no clients found")
	}

	result := make([]OidcClientResponse, 0, 16)
	for _, cl := range dtos {
		clientName := strings.TrimPrefix(cl.ClientName, c.clientNamePrefix)
		if clientName == cl.ClientName {
			continue
		}

		result = append(result, cl)
	}

	return result, nil
}

func (c *HttpApiClient) GetClient(
	ctx context.Context,
	clientId string,
) (*OidcClientResponse, *jose.JSONWebKeySet, error) {
	ctx, span := c.tracer.Start(ctx, "GetClient")
	defer span.End()

	url, err := url.JoinPath(c.config.SelfServiceUrl, "/clients", clientId)
	if err != nil {
		return nil, nil, err
	}

	req, err := c.createReq(ctx, url, "GET", nil)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("Accept", "application/json")
	resp, err := c.retryableHTTPDo(req)
	if err != nil {
		return nil, nil, err
	}

	if resp.StatusCode != 200 {
		defer func() { _ = resp.Body.Close() }()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, nil, errors.WrapPrefix(err, "error reading body on unexpected status", 0)
		}
		return nil, nil, fmt.Errorf("unexpected status code: %d, body:\n%s", resp.StatusCode, body)
	}

	dto, err := deserialize[OidcClientResponse](resp)
	if err != nil {
		return nil, nil, err
	}

	clientName := strings.TrimPrefix(dto.ClientName, c.clientNamePrefix)
	if clientName == dto.ClientName {
		return nil, nil, errors.New(fmt.Errorf("unexpected client name: %s", dto.ClientName))
	}

	jwks, err := c.getClientJwks(ctx, clientId)
	if err != nil {
		return nil, nil, err
	}

	return &dto, jwks, nil
}

func (c *HttpApiClient) getClientJwks(ctx context.Context, clientId string) (*jose.JSONWebKeySet, error) {
	ctx, span := c.tracer.Start(ctx, "GetClientJwks")
	defer span.End()

	if clientId == "" {
		return nil, errors.New("missing ID on client info")
	}

	url, err := url.JoinPath(c.config.SelfServiceUrl, "/clients", clientId, "jwks")
	if err != nil {
		return nil, err
	}

	req, err := c.createReq(ctx, url, "GET", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	resp, err := c.retryableHTTPDo(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		defer func() { _ = resp.Body.Close() }()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, errors.WrapPrefix(err, "error reading body on unexpected status", 0)
		}
		return nil, fmt.Errorf("unexpected status code: %d, body:\n%s", resp.StatusCode, body)
	}

	jwks, err := deserialize[jose.JSONWebKeySet](resp)
	if err != nil {
		return nil, err
	}

	return &jwks, nil
}

var ErrFailedToCreateJwks = errors.Errorf("Created Maskinporten client, but failed to create associated JWKS")

func (c *HttpApiClient) CreateClient(
	ctx context.Context,
	client *OidcClientRequest,
	jwks *jose.JSONWebKeySet,
) (*OidcClientResponse, error) {
	ctx, span := c.tracer.Start(ctx, "CreateClient")
	defer span.End()

	if jwks == nil {
		return nil, errors.New("can't create maskinporten client without JWKS initialized")
	}

	url, err := url.JoinPath(c.config.SelfServiceUrl, "/clients")
	if err != nil {
		return nil, err
	}

	buf, err := json.Marshal(client)
	if err != nil {
		return nil, err
	}

	req, err := c.createReq(ctx, url, "POST", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := c.retryableHTTPDo(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 201 {
		defer func() { _ = resp.Body.Close() }()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, errors.WrapPrefix(err, "error reading body on unexpected status", 0)
		}
		bodyString := string(body)
		return nil, fmt.Errorf("unexpected status code: %d, body:\n%s", resp.StatusCode, bodyString)
	}

	result, err := deserialize[OidcClientResponse](resp)
	if err != nil {
		return nil, err
	}

	err = c.CreateClientJwks(ctx, result.ClientId, jwks)
	if err != nil {
		// TODO: hmm, delete client?
		return nil, errors.Errorf("error creating client: %w, %w", ErrFailedToCreateJwks, err)
	}

	return &result, nil
}

func (c *HttpApiClient) UpdateClient(
	ctx context.Context,
	clientId string,
	client *OidcClientRequest,
) (*OidcClientResponse, error) {
	ctx, span := c.tracer.Start(ctx, "UpdateClient")
	defer span.End()

	if clientId == "" {
		return nil, errors.Errorf(
			"tried to update maskinporten client with empty ID for client name: %s",
			client.ClientName,
		)
	}

	url, err := url.JoinPath(c.config.SelfServiceUrl, "/clients", clientId)
	if err != nil {
		return nil, err
	}

	buf, err := json.Marshal(client)
	if err != nil {
		return nil, err
	}

	req, err := c.createReq(ctx, url, "PUT", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := c.retryableHTTPDo(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		defer func() { _ = resp.Body.Close() }()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, errors.WrapPrefix(err, "error reading body on unexpected status", 0)
		}
		bodyString := string(body)
		return nil, fmt.Errorf("unexpected status code: %d, body:\n%s", resp.StatusCode, bodyString)
	}

	dto, err := deserialize[OidcClientResponse](resp)
	if err != nil {
		return nil, err
	}

	return &dto, nil
}

func (c *HttpApiClient) CreateClientJwks(ctx context.Context, clientId string, jwks *jose.JSONWebKeySet) error {
	ctx, span := c.tracer.Start(ctx, "CreateClientJwks")
	defer span.End()

	if clientId == "" {
		return errors.New("missing ID on client info")
	}
	if jwks == nil {
		return errors.New("can't create maskinporten client without JWKS initialized")
	}
	for _, jwk := range jwks.Keys {
		// jwk.Certificates
		if !jwk.IsPublic() {
			return errors.Errorf("tried to upload private key JWKS to Maskinporten for: %s", clientId)
		}
	}

	url, err := url.JoinPath(c.config.SelfServiceUrl, "/clients", clientId, "jwks")
	if err != nil {
		return err
	}

	buf, err := json.Marshal(&jwks)
	if err != nil {
		return err
	}

	req, err := c.createReq(ctx, url, "POST", bytes.NewReader(buf))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := c.retryableHTTPDo(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != 201 {
		defer func() { _ = resp.Body.Close() }()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return errors.WrapPrefix(err, "error reading body on unexpected status", 0)
		}
		bodyString := string(body)
		return fmt.Errorf("unexpected status code: %d, body:\n%s", resp.StatusCode, bodyString)
	}

	return nil
}

func (c *HttpApiClient) DeleteClient(ctx context.Context, clientId string) error {
	ctx, span := c.tracer.Start(ctx, "DeleteClient")
	defer span.End()

	url, err := url.JoinPath(c.config.SelfServiceUrl, "/clients", clientId)
	if err != nil {
		return err
	}

	req, err := c.createReq(ctx, url, "DELETE", nil)
	if err != nil {
		return err
	}

	resp, err := c.retryableHTTPDo(req)
	if err != nil {
		return err
	}

	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.WrapPrefix(err, "error reading body", 0)
	}
	bodyString := string(body)

	if resp.StatusCode != 200 {
		return fmt.Errorf("unexpected status code: %d, body:\n%s", resp.StatusCode, bodyString)
	}

	return nil
}

func (c *HttpApiClient) createGrant(ctx context.Context) (*string, error) {
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

func (c *HttpApiClient) accessTokenFetcher(ctx context.Context) (*TokenResponse, error) {
	grant, err := c.createGrant(ctx)
	if err != nil {
		return nil, err
	}

	endpoint, err := url.JoinPath(c.config.AuthorityUrl, "/token")
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

	tokenResp, err := deserialize[TokenResponse](resp)
	if err != nil {
		return nil, err
	}

	return &tokenResp, nil
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
}

func (c *HttpApiClient) wellKnownFetcher(ctx context.Context) (*WellKnownResponse, error) {
	endpoint, err := url.JoinPath(c.config.AuthorityUrl, "/.well-known/oauth-authorization-server")
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.retryableHTTPDo(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	wellKnownResp, err := deserialize[WellKnownResponse](resp)
	if err != nil {
		return nil, err
	}
	return &wellKnownResp, nil
}

func deserialize[T any](resp *http.Response) (T, error) {
	// TODO: accept `result` as a pointer from outside?

	// There is not much to do about the error returned from closing the body
	// apparently this should not happen for the Closer set to the response body
	defer func() { _ = resp.Body.Close() }()

	var result T
	err := json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return result, err
	}

	return result, err
}

// retryableHTTPDo performs an HTTP request with retry logic.
func (c *HttpApiClient) retryableHTTPDo(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	req.Header.Add("X-Altinn-Operator-RunId", c.context.RunId)

	// TODO: different strategy??

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
