package config

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	operatorcontext "github.com/altinn/altinn-k8s-operator/internal/operator_context"
)

func loadFromAzureKeyVault(operatorContext *operatorcontext.Context) (*Config, error) {
	context := context.Background()

	var cred azcore.TokenCredential
	var err error

	if operatorContext.IsLocal() {
		cred, err = azidentity.NewDefaultAzureCredential(nil)
	} else {
		cred, err = azidentity.NewWorkloadIdentityCredential(nil)
	}

	if err != nil {
		return nil, fmt.Errorf("error getting credentials for loading config: %w", err)
	}

	url := fmt.Sprintf("https://altinn-%s-operator-kv.vault.azure.net", operatorContext.Env)
	client, err := azsecrets.NewClient(url, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("error building client for Azure KV: %w", err)
	}

	secretKeys := []string{"ClientId", "Url", "Jwk", "Scope"}

	config := &Config{}
	for _, secretKey := range secretKeys {
		secret, err := client.GetSecret(context, secretKey, "", nil)
		if err != nil {
			return nil, fmt.Errorf("error getting secret: %s, %w", secretKey, err)
		}

		switch secretKey {
		case "ClientId":
			config.MaskinportenApi.ClientId = *secret.Value
		case "Url":
			config.MaskinportenApi.Url = *secret.Value
		case "Jwk":
			config.MaskinportenApi.Jwk = *secret.Value
		case "Scope":
			config.MaskinportenApi.Scope = *secret.Value
		}
	}

	return config, nil
}
