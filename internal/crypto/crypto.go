package crypto

import (
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/altinn/altinn-k8s-operator/internal/assert"
	"github.com/altinn/altinn-k8s-operator/internal/operatorcontext"
	"github.com/go-errors/errors"
	"github.com/go-jose/go-jose/v4"
	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
)

type CryptoService struct {
	ctx    *operatorcontext.Context
	clock  clockwork.Clock
	random io.Reader
}

func NewService(
	ctx *operatorcontext.Context,
	clock clockwork.Clock,
	random io.Reader,
) *CryptoService {
	return &CryptoService{
		ctx,
		clock,
		random,
	}
}

// Creates the initial JWKS for a new app
// Constructs the JWKS from the whole RSA private/public key pair
// Uses SHA256 with RSA, 2048 bits for RSA
func (s *CryptoService) CreateJwks(appId string) (*jose.JSONWebKeySet, error) {
	cert, rsaKey, err := s.createCert(appId)
	if err != nil {
		return nil, errors.WrapPrefix(err, "error creating JWKS cert", 0)
	}

	return s.createJWKS(cert, rsaKey, 0)
}

func (s *CryptoService) createJWKS(
	cert *x509.Certificate,
	rsaKey *rsa.PrivateKey,
	index int,
) (*jose.JSONWebKeySet, error) {
	id, err := uuid.NewRandomFromReader(s.random)
	if err != nil {
		return nil, err
	}
	keyId := fmt.Sprintf("%s.%d", id.String(), index)
	return &jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			{
				Certificates: []*x509.Certificate{cert},
				Key:          rsaKey,
				KeyID:        keyId,
				Use:          "sig",
				// as of 21.08.24: only RS256 + 2048 bits is supported by Maskinporten:
				// https://docs.digdir.no/docs/idporten/oidc/oidc_api_admin.html#bruk-av-asymmetrisk-n%C3%B8kkel
				Algorithm: string(jose.RS256),
			},
		},
	}, nil
}

func (s *CryptoService) generateCertSerialNumber() (*big.Int, error) {
	// x509 serial number is a 20 bytes unsigned integer
	// source: https://www.rfc-editor.org/rfc/rfc3280#section-4.1.2.2
	// 16 bytes (128 bits) should be enough to be unique - UUID v4 (random) uses 122 bits
	serial := new(big.Int)
	serialBytes := [16]byte{}
	n, err := io.ReadFull(s.random, serialBytes[:])
	if err != nil {
		return nil, err
	}
	assert.AssertWith(n == len(serialBytes), "Read should always fill slice when err is nil")
	serial.SetBytes(serialBytes[:])
	assert.AssertWith(serial.Sign() != -1, "SetBytes should treat bytes as an unsigned integer")
	return serial, nil
}

func (s *CryptoService) createCert(appId string) (*x509.Certificate, *rsa.PrivateKey, error) {
	// as of 21.08.24: only RS256 + 2048 bits is supported by Maskinporten:
	// https://docs.digdir.no/docs/idporten/oidc/oidc_api_admin.html#bruk-av-asymmetrisk-n%C3%B8kkel
	rsaKey, err := rsa.GenerateKey(s.random, 2048)
	if err != nil {
		return nil, nil, errors.WrapPrefix(err, "error generating RSA key for jwks", 0)
	}

	serial, err := s.generateCertSerialNumber()
	if err != nil {
		return nil, nil, errors.WrapPrefix(err, "error generating serial number for jwks", 0)
	}

	now := s.clock.Now().UTC()
	certTemplate := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization: []string{s.ctx.ServiceOwnerName},
			CommonName:   appId,
		},
		Issuer:                s.getIssuer(),
		NotBefore:             now,
		NotAfter:              now.Add(time.Hour * 24 * 30),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		SignatureAlgorithm:    x509.SHA256WithRSA,
	}

	derBytes, err := x509.CreateCertificate(s.random, &certTemplate, &certTemplate, &rsaKey.PublicKey, rsaKey)
	if err != nil {
		return nil, nil, errors.WrapPrefix(err, "error generating cert for jwks", 0)
	}
	cert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		return nil, nil, errors.WrapPrefix(err, "error parsing generated cert for jwks", 0)
	}

	return cert, rsaKey, nil
}

func (s *CryptoService) RotateIfNeeded(appId string, currentJwks *jose.JSONWebKeySet) (*jose.JSONWebKeySet, error) {
	if currentJwks == nil {
		return nil, errors.New("cant rotate cert for JWKS, JWKS was null")
	}

	var activeKey *jose.JSONWebKey

	for i := 0; i < len(currentJwks.Keys); i++ {
		if activeKey == nil {
			activeKey = &currentJwks.Keys[i]
			certificateCount := len(activeKey.Certificates)
			if certificateCount != 1 {
				return nil, errors.Errorf(
					"unexpected number of certificates for key '%s': '%d'",
					activeKey.KeyID,
					certificateCount,
				)
			}

		} else {
			key := &currentJwks.Keys[i]

			certificateCount := len(key.Certificates)
			if certificateCount != 1 {
				return nil, errors.Errorf("unexpected number of certificates for key '%s': '%d'", key.KeyID, certificateCount)
			}

			cert := key.Certificates[0]
			activeCert := activeKey.Certificates[0]

			if cert.NotAfter.After(activeCert.NotAfter) {
				activeKey = key
			}
		}
	}

	rotationThreshold := s.clock.Now().UTC().Add(time.Hour * 24 * 7)
	if activeKey.Certificates[0].NotAfter.After(rotationThreshold) {
		return nil, nil
	} else {
		keyParts := strings.Split(activeKey.KeyID, ".")
		currentIndexStr := keyParts[len(keyParts)-1]
		currentIndex, err := strconv.Atoi(currentIndexStr)
		if err != nil {
			return nil, errors.Errorf("invalid key format: %s", activeKey.KeyID)
		}
		cert, rsaKey, err := s.createCert(appId)
		if err != nil {
			return nil, errors.WrapPrefix(err, "error creating JWKS cert", 0)
		}

		newJwks, err := s.createJWKS(cert, rsaKey, currentIndex+1)
		if err != nil {
			return nil, err
		}
		newJwks.Keys = append(newJwks.Keys, *activeKey) // TODO: verify that app-lib reads latest key
		return newJwks, nil
	}
}

func (s *CryptoService) getIssuer() pkix.Name {
	return pkix.Name{
		Organization: []string{"Digdir"},
		CommonName:   "Altinn Operator",
	}
}
