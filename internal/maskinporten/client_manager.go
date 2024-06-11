package maskinporten

import (
	"fmt"
	"sync"

	"altinn.studio/altinn-k8s-operator/internal/config"
	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
)

type ClientUpsertRequest struct {
	AppId string
}

type ClientManager interface {
	Get(appId string) (*ClientInfo, error)
	Reconcile(info *ClientInfo) (*ClientInfo, bool, error)
	Delete(appId string) error
}

type clientManager struct {
	mutex      sync.Mutex
	clients    map[string]*ClientInfo
	httpClient *httpApiClient
}

var _ ClientManager = (*clientManager)(nil)

func NewClientManager(config *config.MaskinportenApiConfig, clock clockwork.Clock) (ClientManager, error) {
	httpClient, err := newApiClient(config, clock)
	if err != nil {
		return nil, err
	}
	return &clientManager{
		mutex:      sync.Mutex{},
		clients:    make(map[string]*ClientInfo),
		httpClient: httpClient,
	}, nil
}

func (s *clientManager) Get(appId string) (*ClientInfo, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	client, ok := s.clients[appId]
	if !ok {
		// TODO: fetch client from API
		return nil, nil
	}

	return client, nil
}

func (s *clientManager) Reconcile(info *ClientInfo) (*ClientInfo, bool, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// TODO: sync with client from API
	client, ok := s.clients[info.AppId]
	if !ok {
		uuid, err := uuid.NewRandom()
		if err != nil {
			return nil, false, fmt.Errorf("failed to generate client ID: %w", err)
		}
		client = &ClientInfo{
			Id:     uuid.String(),
			AppId:  info.AppId,
			Scopes: info.Scopes,
		}
		s.clients[client.AppId] = client
		return client, true, nil
	} else {
		client.Scopes = info.Scopes
		// nothing to update yet
		return client, false, nil
	}
}

func (s *clientManager) Delete(appId string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, ok := s.clients[appId]; !ok {
		return fmt.Errorf("client not found")
	}

	delete(s.clients, appId)
	// TODO: sync with client API
	return nil
}
