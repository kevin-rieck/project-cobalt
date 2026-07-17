package opcua

import (
	"context"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClientConnectRejectsSecureEndpointWithoutCertificateAndKey(t *testing.T) {
	client := &gopcuaClient{}

	err := client.Connect(context.Background(), ConnectRequest{
		Endpoint:       "opc.tcp://localhost:4840",
		SecurityPolicy: "Basic256Sha256",
		SecurityMode:   "Sign",
		AuthType:       AuthAnonymous,
	})

	if err == nil {
		t.Fatal("expected missing certificate/key error")
	}
	if !strings.Contains(err.Error(), "client certificate and private key") {
		t.Fatalf("error = %v", err)
	}
}

func TestClientConnectRejectsEncryptedClientPrivateKeyBeforeEndpointDiscovery(t *testing.T) {
	keyPath := filepath.Join(t.TempDir(), "encrypted_trusted_client_key.pem")
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "ENCRYPTED PRIVATE KEY", Bytes: []byte{1, 2, 3}})
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatal(err)
	}

	client := &gopcuaClient{}
	err := client.Connect(context.Background(), ConnectRequest{
		Endpoint:              "opc.tcp://localhost:4840",
		SecurityPolicy:        "Basic256Sha256",
		SecurityMode:          "SignAndEncrypt",
		AuthType:              AuthAnonymous,
		ClientCertificatePath: "C:/certs/client.pem",
		ClientPrivateKeyPath:  keyPath,
	})

	if err == nil {
		t.Fatal("expected encrypted private key error")
	}
	if !strings.Contains(err.Error(), "Client Private Key is encrypted") || !strings.Contains(err.Error(), "unencrypted RSA private key") {
		t.Fatalf("error = %v", err)
	}
}

func TestClientConnectRejectsUnsupportedAuthType(t *testing.T) {
	for _, authType := range []AuthType{AuthType("Certificate"), AuthType("IssuedToken")} {
		t.Run(string(authType), func(t *testing.T) {
			client := &gopcuaClient{}

			err := client.Connect(context.Background(), ConnectRequest{
				Endpoint:       "opc.tcp://localhost:4840",
				SecurityPolicy: "None",
				SecurityMode:   "None",
				AuthType:       authType,
			})

			if err == nil {
				t.Fatal("expected unsupported authentication error")
			}
			if !strings.Contains(err.Error(), "unsupported authentication") || !strings.Contains(err.Error(), string(authType)) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}
