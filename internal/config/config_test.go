package config

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/altinn/altinn-k8s-operator/internal/operatorcontext"
	"github.com/go-playground/validator/v10"
	. "github.com/onsi/gomega"
)

func TestConfigMissingValuesFail(t *testing.T) {
	RegisterTestingT(t)

	file, err := os.CreateTemp(os.TempDir(), "*.env")
	Expect(err).NotTo(HaveOccurred())
	defer func() {
		err := file.Close()
		Expect(err).NotTo(HaveOccurred())
	}()
	defer func() {
		err := os.Remove(file.Name())
		Expect(err).NotTo(HaveOccurred())
	}()

	_, err = file.WriteString("maskinporten_api.url=https://example.com")
	Expect(err).NotTo(HaveOccurred())

	operatorContext := operatorcontext.DiscoverOrDie(context.Background())
	cfg, err := GetConfig(operatorContext, file.Name())
	Expect(cfg).To(BeNil())
	Expect(err).To(HaveOccurred())
	_, ok := err.(validator.ValidationErrors)
	errType := reflect.TypeOf(err)
	Expect(errType.String()).To(Equal("validator.ValidationErrors"))
	Expect(ok).To(BeTrue())
}

func TestConfigTestEnvLoadsOk(t *testing.T) {
	RegisterTestingT(t)

	operatorContext := operatorcontext.DiscoverOrDie(context.Background())
	cfg, err := GetConfig(operatorContext, "")
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())
	Expect(cfg.MaskinportenApi.ClientId).To(Equal("64d8055d-bf0c-4ee2-979e-d2bbe996a9f5"))
	Expect(cfg.MaskinportenApi.Url).To(Equal("https://maskinporten.dev"))
	Expect(cfg.MaskinportenApi.Jwk).NotTo(BeNil())
}
