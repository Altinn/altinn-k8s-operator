package fakes

import (
	"time"
	"unsafe"

	"github.com/altinn/altinn-k8s-operator/internal/maskinporten"
	"github.com/go-errors/errors"
	"github.com/go-jose/go-jose/v4"
)

var InvalidClientName = errors.Errorf("invalid client ID")
var ClientAlreadyExists = errors.Errorf("client already exists")

const SupplierOrgNo string = "11111111"

type Db struct {
	Clients       []ClientRecord
	ClientIdIndex map[string]int
}

type ClientRecord struct {
	ClientId string
	Client   *maskinporten.OidcClientResponse
	Jwks     *jose.JSONWebKeySet
}

func NewDb() Db {
	return Db{
		Clients:       make([]ClientRecord, 64),
		ClientIdIndex: make(map[string]int, 64),
	}
}

func (d *Db) Insert(req *maskinporten.OidcClientRequest, jwks *jose.JSONWebKeySet) error {
	if req.ClientName == "" {
		return errors.New(InvalidClientName)
	}
	clientId := req.ClientName
	_, ok := d.ClientIdIndex[clientId]
	if ok {
		return errors.New(ClientAlreadyExists)
	}

	supplierOrg := SupplierOrgNo
	now := time.Now()
	active := true
	jwksUri := ""
	client := &maskinporten.OidcClientResponse{
		ClientId:                          clientId,
		ClientName:                        req.ClientName,
		LogoUri:                           req.LogoUri,
		Description:                       req.Description,
		Scopes:                            req.Scopes,
		RedirectUris:                      req.RedirectUris,
		PostLogoutRedirectUris:            req.PostLogoutRedirectUris,
		AuthorizationLifetime:             req.AuthorizationLifetime,
		AccessTokenLifetime:               req.AccessTokenLifetime,
		RefreshTokenLifetime:              req.RefreshTokenLifetime,
		RefreshTokenUsage:                 (*maskinporten.OidcClientResponseRefreshTokenUsage)(req.RefreshTokenUsage),
		FrontchannelLogoutUri:             req.FrontchannelLogoutUri,
		FrontchannelLogoutSessionRequired: req.FrontchannelLogoutSessionRequired,
		TokenEndpointAuthMethod: (*maskinporten.OidcClientResponseTokenEndpointAuthMethod)(
			req.TokenEndpointAuthMethod,
		),
		GrantTypes: *(*[]maskinporten.OidcClientResponseGrantTypesElem)(
			unsafe.Pointer(&req.GrantTypes),
		),
		IntegrationType:      (*maskinporten.OidcClientResponseIntegrationType)(req.IntegrationType),
		ApplicationType:      (*maskinporten.OidcClientResponseApplicationType)(req.ApplicationType),
		SsoDisabled:          req.SsoDisabled,
		CodeChallengeMethod:  (*maskinporten.OidcClientResponseCodeChallengeMethod)(req.CodeChallengeMethod),
		ClientOrgName:        req.ClientOrgName,
		ClientOrgDescription: req.ClientOrgDescription,
		ClientLogoUri:        req.ClientLogoUri,
		LastUpdated:          &now,
		Created:              &now,
		ClientSecret:         nil,
		ClientOrgno:          *req.ClientOrgno,
		SupplierOrgno:        &supplierOrg,
		Active:               &active,
		JwksUri:              &jwksUri,
	}

	record := ClientRecord{
		ClientId: clientId,
		Client:   client,
		Jwks:     jwks,
	}

	idx := len(d.Clients)
	d.Clients = append(d.Clients, record)
	d.ClientIdIndex[clientId] = idx
	return nil
}

func (d *Db) UpdateJwks(clientId string, jwks *jose.JSONWebKeySet) error {
	i, ok := d.ClientIdIndex[clientId]
	if !ok {
		return errors.New("client not found")
	}

	d.Clients[i].Jwks = jwks
	return nil
}

func (d *Db) Delete(clientId string) bool {
	i, ok := d.ClientIdIndex[clientId]
	if !ok {
		return false
	}

	delete(d.ClientIdIndex, clientId)

	d.Clients[i] = d.Clients[len(d.Clients)-1]
	d.Clients = d.Clients[:len(d.Clients)-1]
	return true
}

func (d *Db) Query(predicate func(*ClientRecord) bool) []ClientRecord {
	result := make([]ClientRecord, 0, 4)
	for i := 0; i < len(d.Clients); i++ {
		client := &d.Clients[i]
		if predicate(client) {
			result = append(result, *client)
		}
	}
	return result
}
