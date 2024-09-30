// Program that contains fake APIs for Maskinporten self-service API and OAuth authority server
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/go-errors/errors"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/altinn/altinn-k8s-operator/internal/assert"
	"github.com/altinn/altinn-k8s-operator/internal/config"
	"github.com/altinn/altinn-k8s-operator/internal/fakes"
	"github.com/altinn/altinn-k8s-operator/internal/maskinporten"
	"github.com/altinn/altinn-k8s-operator/internal/operatorcontext"
)

const StateKey = "state"

func main() {
	ctx := setupSignalHandler()
	var wg sync.WaitGroup

	state := fakes.NewState()
	ctx = context.WithValue(ctx, StateKey, state)

	operatorContext := operatorcontext.DiscoverOrDie(ctx)
	cfg := config.GetConfigOrDie(operatorContext, config.ConfigSourceKoanf, "")

	wg.Add(2)
	go runMaskinportenApi(ctx, &wg, cfg)
	go runSelfServiceApi(ctx, &wg)

	log.Println("Started server threads")
	wg.Wait()
	log.Println("Shutting down..")
}

type FakeToken struct {
	Scopes   []string `json:"scopes"`
	ClientId string   `json:"client_id"`
}

func serve(ctx context.Context, name string, addr string, registerHandlers func(*http.ServeMux)) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthEndpoint)
	registerHandlers(mux)
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
		BaseContext: func(l net.Listener) context.Context {
			return ctx
		},
	}
	go func() {
		<-ctx.Done()
		if err := server.Close(); err != nil {
			log.Fatalf("[%s] HTTP server close error: %v", name, err)
		}
	}()
	log.Printf("[%s] HTTP server starting: addr=%s\n", name, server.Addr)
	err := server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		log.Printf("[%s] HTTP server shutting down\n", name)
	} else if err != nil {
		log.Fatalf("[%s] HTTP server error: %v", name, err)
	}
}

func runMaskinportenApi(ctx context.Context, wg *sync.WaitGroup, cfg *config.Config) {
	defer wg.Done()
	name := "Maskinporten API"
	addr := ":8050"

	state := ctx.Value(StateKey).(*fakes.State)
	assert.Assert(state != nil)

	jwk := jose.JSONWebKey{}
	if err := json.Unmarshal([]byte(cfg.MaskinportenApi.Jwk), &jwk); err != nil {
		log.Fatalf("couldn't unmarshal JWK: %v", err)
	}
	publicJwk := jwk.Public()

	integrationType := maskinporten.OidcClientRequestIntegrationTypeMaskinporten
	appType := maskinporten.OidcClientRequestApplicationTypeWeb
	tokenEndpointMethod := maskinporten.OidcClientRequestTokenEndpointAuthMethodPrivateKeyJwt
	orgNo := "123456789"
	err := state.Db.Insert(&maskinporten.OidcClientRequest{
		ClientName:  cfg.MaskinportenApi.ClientId,
		ClientOrgno: &orgNo,
		GrantTypes: []maskinporten.OidcClientRequestGrantTypesElem{
			maskinporten.OidcClientRequestGrantTypesElemUrnIetfParamsOauthGrantTypeJwtBearer,
		},
		Scopes:                  []string{cfg.MaskinportenApi.Scope},
		IntegrationType:         &integrationType,
		ApplicationType:         &appType,
		TokenEndpointAuthMethod: &tokenEndpointMethod,
	}, &jose.JSONWebKeySet{Keys: []jose.JSONWebKey{publicJwk}})
	if err != nil {
		log.Fatalf("couldn't insert supplier client: %v", err)
	}

	serve(ctx, name, addr, func(mux *http.ServeMux) {
		mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				w.WriteHeader(404)
				return
			}
			grantType := r.URL.Query().Get("grant_type")
			if grantType != "urn:ietf:params:oauth:grant-type:jwt-bearer" {
				w.WriteHeader(400)
				log.Printf("invalid grant_type: %s\n", grantType)
				return
			}
			assertion := r.URL.Query().Get("assertion")
			if assertion == "" {
				w.WriteHeader(400)
				log.Printf("missing assertion\n")
				return
			}

			jwtToken, err := jwt.ParseSigned(assertion, []jose.SignatureAlgorithm{jose.RS256})
			if err != nil {
				w.WriteHeader(400)
				log.Printf("couldn't parse JWT: %v\n", errors.Wrap(err, 0))
				return
			}
			if len(jwtToken.Headers) != 1 {
				w.WriteHeader(400)
				log.Printf("expected exactly one header, got %d\n", len(jwtToken.Headers))
				return
			}

			header := jwtToken.Headers[0]
			if header.KeyID == "" {
				w.WriteHeader(400)
				log.Printf("missing kid\n")
				return
			}

			clients := state.Db.Query(func(ocr *fakes.ClientRecord) bool {
				if ocr.Jwks == nil {
					return false
				}
				for _, jwk := range ocr.Jwks.Keys {
					if jwk.KeyID == header.KeyID {
						return true
					}
				}
				return false
			})
			if len(clients) != 1 {
				w.WriteHeader(400)
				log.Printf("client not found: %s\n", header.KeyID)
				return
			}
			client := clients[0]

			claims := jwt.Claims{}
			if err := jwtToken.Claims(client.Jwks.Keys[0], &claims); err != nil {
				w.WriteHeader(400)
				log.Printf("couldn't validate JWT: %v\n", errors.Wrap(err, 0))
				return
			}

			clientId := claims.Issuer
			if clientId == "" {
				w.WriteHeader(400)
				log.Printf("missing issuer\n")
				return
			}
			if clientId != client.ClientId {
				w.WriteHeader(400)
				log.Printf("invalid issuer: %s\n", clientId)
				return
			}

			w.Header().Add("Content-Type", "application/json")

			fakeToken := FakeToken{
				Scopes:   client.Client.Scopes,
				ClientId: client.ClientId,
			}
			tokenJson, err := json.Marshal(fakeToken)
			if err != nil {
				w.WriteHeader(500)
				log.Printf("couldn't encode scopes: %v\n", errors.Wrap(err, 0))
				return
			}
			base64Token := base64.StdEncoding.EncodeToString(tokenJson)

			encoder := json.NewEncoder(w)
			err = encoder.Encode(maskinporten.TokenResponse{
				AccessToken: base64Token,
				TokenType:   "Bearer",
				Scope:       strings.Join(client.Client.Scopes, " "),
				ExpiresIn:   20,
			})
			if err != nil {
				w.WriteHeader(500)
				log.Printf("couldn't write response: %v\n", errors.Wrap(err, 0))
			}
		})
		mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" {
				w.WriteHeader(404)
				return
			}
			w.Header().Add("Content-Type", "application/json")
			encoder := json.NewEncoder(w)
			err := encoder.Encode(maskinporten.WellKnownResponse{
				Issuer:                            "http://localhost:8050",
				TokenEndpoint:                     "http://localhost:8050/token",
				JwksURI:                           "http://localhost:8050/jwks",
				TokenEndpointAuthMethodsSupported: []string{"private_key_jwt"},
				GrantTypesSupported:               []string{"urn:ietf:params:oauth:grant-type:jwt-bearer"},
				TokenEndpointAuthSigningAlgValuesSupported: []string{"RS256", "RS384", "RS512"},
			})
			if err != nil {
				w.WriteHeader(500)
				log.Printf("couldn't write response: %v\n", errors.Wrap(err, 0))
			}
		})
	})
}

func runSelfServiceApi(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	name := "Self Service API"
	addr := ":8051"

	auth := func(r *http.Request) *FakeToken {
		if r.Header.Get("Authorization") == "" {
			return nil
		}

		encoded := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if encoded == "" {
			return nil
		}

		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil
		}

		var token FakeToken
		err = json.Unmarshal(decoded, &token)
		if err != nil {
			return nil
		}

		clientId := r.PathValue("clientId")
		if clientId != "" && clientId != token.ClientId && token.ClientId != "altinn_apps_supplier_client" {
			return nil
		}

		return &token
	}

	serve(ctx, name, addr, func(mux *http.ServeMux) {
		mux.HandleFunc("/clients", func(w http.ResponseWriter, r *http.Request) {
			state := r.Context().Value(StateKey).(*fakes.State)
			assert.Assert(state != nil)

			switch r.Method {
			case "GET":
				if auth(r) == nil {
					w.WriteHeader(401)
					return
				}

				w.Header().Add("Content-Type", "application/json")
				encoder := json.NewEncoder(w)
				records := state.Db.Clients
				clients := make([]*maskinporten.OidcClientResponse, len(records))
				for i, record := range records {
					clients[i] = record.Client
				}

				err := encoder.Encode(clients)
				if err != nil {
					w.WriteHeader(500)
					log.Printf("couldn't write response: %v\n", errors.Wrap(err, 0))
				}
			case "POST":
				if auth(r) == nil {
					w.WriteHeader(401)
					return
				}

				decoder := json.NewDecoder(r.Body)
				var client maskinporten.OidcClientRequest
				err := decoder.Decode(&client)
				if err != nil {
					w.WriteHeader(400)
					log.Printf("couldn't read request: %v\n", errors.Wrap(err, 0))
					return
				}
				err = state.Db.Insert(&client, nil)
				if err != nil {
					w.WriteHeader(400)
					log.Printf("couldn't insert client: %v\n", errors.Wrap(err, 0))
					return
				}

				clients := state.Db.Query(func(ocr *fakes.ClientRecord) bool {
					return ocr.ClientId == client.ClientName
				})
				if len(clients) != 1 {
					w.WriteHeader(500)
					log.Printf("couldn't find client after insert\n")
					return
				}

				w.WriteHeader(201)
				w.Header().Add("Content-Type", "application/json")
				encoder := json.NewEncoder(w)
				err = encoder.Encode(clients[0].Client)
				if err != nil {
					w.WriteHeader(500)
					log.Printf("couldn't write response: %v\n", errors.Wrap(err, 0))
					return
				}

			default:
				if auth(r) == nil {
					w.WriteHeader(401)
					return
				}

				w.WriteHeader(404)
			}
		})

		mux.HandleFunc("/clients/{clientId}", func(w http.ResponseWriter, r *http.Request) {
			state := r.Context().Value(StateKey).(*fakes.State)
			assert.Assert(state != nil)

			clientId := r.PathValue("clientId")

			switch r.Method {
			case "GET":
				if auth(r) == nil {
					w.WriteHeader(401)
					return
				}

				w.Header().Add("Content-Type", "application/json")
				encoder := json.NewEncoder(w)
				clients := state.Db.Clients
				for _, client := range clients {
					if client.ClientId == clientId {
						err := encoder.Encode(clients)
						if err != nil {
							w.WriteHeader(500)
							log.Printf("couldn't write response: %v\n", errors.Wrap(err, 0))
						}
					}
				}

				w.WriteHeader(204)
			case "PUT":
				if auth(r) == nil {
					w.WriteHeader(401)
					return
				}

				decoder := json.NewDecoder(r.Body)
				var client maskinporten.OidcClientRequest
				err := decoder.Decode(&client)
				if err != nil {
					w.WriteHeader(400)
					log.Printf("couldn't read request: %v\n", errors.Wrap(err, 0))
					return
				}
				if client.ClientName != clientId {
					w.WriteHeader(400)
					log.Printf(
						"couldn't read request: client ID did not match path=%s body=%s\n",
						clientId,
						client.ClientName,
					)
					return
				}
				deleted := state.Db.Delete(clientId)
				if !deleted {
					w.WriteHeader(400)
					log.Printf(
						"couldn't read request: client does not exist clientId=%s\n",
						clientId,
					)
					return
				}
				err = state.Db.Insert(&client, nil)
				if err != nil {
					w.WriteHeader(400)
					log.Printf("couldn't insert client: %v\n", errors.Wrap(err, 0))
					return
				}
				w.WriteHeader(200)
			case "DELETE":
				if auth(r) == nil {
					w.WriteHeader(401)
					return
				}

			default:
				if auth(r) == nil {
					w.WriteHeader(401)
					return
				}

				w.WriteHeader(404)
			}
		})

		mux.HandleFunc("/clients/{clientId}/jwks", func(w http.ResponseWriter, r *http.Request) {
			state := r.Context().Value(StateKey).(*fakes.State)
			assert.Assert(state != nil)

			clientId := r.PathValue("clientId")

			switch r.Method {
			case "GET":
				if auth(r) == nil {
					w.WriteHeader(401)
					return
				}
				w.Header().Add("Content-Type", "application/json")

				clients := state.Db.Query(func(ocr *fakes.ClientRecord) bool {
					return ocr.ClientId == clientId
				})
				if len(clients) != 1 {
					w.WriteHeader(404)
					return
				}

				encoder := json.NewEncoder(w)
				err := encoder.Encode(clients[0].Jwks)
				if err != nil {
					w.WriteHeader(500)
					log.Printf("couldn't write response: %v\n", errors.Wrap(err, 0))
				}

			case "POST":
				if auth(r) == nil {
					w.WriteHeader(401)
					return
				}

				decoder := json.NewDecoder(r.Body)
				var jwks jose.JSONWebKeySet
				err := decoder.Decode(&jwks)
				if err != nil {
					w.WriteHeader(400)
					log.Printf("couldn't read request: %v\n", errors.Wrap(err, 0))
					return
				}

				err = state.Db.UpdateJwks(clientId, &jwks)
				if err != nil {
					w.WriteHeader(400)
					log.Printf("couldn't update JWKS: %v\n", errors.Wrap(err, 0))
					return
				}
				w.WriteHeader(201)

			default:
				if auth(r) == nil {
					w.WriteHeader(401)
					return
				}
				w.WriteHeader(404)
			}
		})
	})
}

func healthEndpoint(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
}

var shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

func setupSignalHandler() context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	c := make(chan os.Signal, 2)
	signal.Notify(c, shutdownSignals...)
	go func() {
		<-c
		cancel()
		<-c
		os.Exit(1) // second signal. Exit directly.
	}()

	return ctx
}
