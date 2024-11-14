package fakes

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/altinn/altinn-k8s-operator/internal/config"
	"github.com/altinn/altinn-k8s-operator/internal/maskinporten"
	"github.com/go-jose/go-jose/v4"
)

type State struct {
	Db   map[string]*Db
	cfg  *config.Config
	lock sync.Mutex
}

func (s *State) GetAll() map[string][]ClientRecord {
	s.lock.Lock()
	defer s.lock.Unlock()
	res := make(map[string][]ClientRecord)
	for runId, db := range s.Db {
		records := db.Query(func(ocr *ClientRecord) bool {
			return true
		})
		res[runId] = records
	}
	return res
}

func (s *State) GetDb(req *http.Request) *Db {
	runId := req.Header.Get("X-Altinn-Operator-RunId")
	if runId == "" {
		log.Fatalf("Missing X-Altinn-Operator-RunId header in request: %v", req)
	}

	s.lock.Lock()
	defer s.lock.Unlock()
	var db *Db
	if existingDb, ok := s.Db[runId]; !ok {
		db = s.initDb()
		s.Db[runId] = db
	} else {
		db = existingDb
	}

	return db
}

func (s *State) initDb() *Db {
	db := NewDb()
	jwk := jose.JSONWebKey{}
	if err := json.Unmarshal([]byte(s.cfg.MaskinportenApi.Jwk), &jwk); err != nil {
		log.Fatalf("couldn't unmarshal JWK: %v", err)
	}
	publicJwk := jwk.Public()

	integrationType := maskinporten.OidcClientRequestIntegrationTypeMaskinporten
	appType := maskinporten.OidcClientRequestApplicationTypeWeb
	tokenEndpointMethod := maskinporten.OidcClientRequestTokenEndpointAuthMethodPrivateKeyJwt
	orgNo := "123456789"
	_, err := db.Insert(&maskinporten.OidcClientRequest{
		ClientName:  s.cfg.MaskinportenApi.ClientId,
		ClientOrgno: &orgNo,
		GrantTypes: []maskinporten.OidcClientRequestGrantTypesElem{
			maskinporten.OidcClientRequestGrantTypesElemUrnIetfParamsOauthGrantTypeJwtBearer,
		},
		Scopes:                  []string{s.cfg.MaskinportenApi.Scope},
		IntegrationType:         &integrationType,
		ApplicationType:         &appType,
		TokenEndpointAuthMethod: &tokenEndpointMethod,
	}, &jose.JSONWebKeySet{Keys: []jose.JSONWebKey{publicJwk}}, s.cfg.MaskinportenApi.ClientId)
	if err != nil {
		log.Fatalf("couldn't insert supplier client: %v", err)
	}

	return db
}

func NewState(cfg *config.Config) *State {
	return &State{
		Db:   make(map[string]*Db),
		cfg:  cfg,
		lock: sync.Mutex{},
	}
}
