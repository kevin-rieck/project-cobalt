package opcua

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gopcua "github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
)

func TestClientConnectReusesValidatedPrivateKeyWhenConstructingSecureClient(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	directory := t.TempDir()
	keyPath := filepath.Join(directory, "client-key.pem")
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("write RSA key: %v", err)
	}
	certificateDER, err := x509.CreateCertificate(rand.Reader, &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}, &x509.Certificate{SerialNumber: big.NewInt(1)}, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("create Client Certificate: %v", err)
	}
	certificatePath := filepath.Join(directory, "client-certificate.pem")
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER})
	if err := os.WriteFile(certificatePath, certificatePEM, 0o600); err != nil {
		t.Fatalf("write Client Certificate: %v", err)
	}

	readCount := 0
	constructed := false
	client := &gopcuaClient{
		readFile: func(filename string) ([]byte, error) {
			readCount++
			return os.ReadFile(filename)
		},
		getEndpoints: func(context.Context, string) ([]*ua.EndpointDescription, error) {
			return []*ua.EndpointDescription{{
				EndpointURL:       "opc.tcp://localhost:4840",
				SecurityPolicyURI: "http://opcfoundation.org/UA/SecurityPolicy#Basic256Sha256",
				SecurityMode:      ua.MessageSecurityModeSign,
				UserIdentityTokens: []*ua.UserTokenPolicy{{
					TokenType: ua.UserTokenTypeAnonymous,
				}},
			}}, nil
		},
		connectClient: func(client *gopcua.Client, _ context.Context) error {
			constructed = client != nil
			return nil
		},
	}

	err = client.Connect(context.Background(), ConnectRequest{
		Endpoint:              "opc.tcp://localhost:4840",
		SecurityPolicy:        "Basic256Sha256",
		SecurityMode:          "Sign",
		AuthType:              AuthAnonymous,
		ClientCertificatePath: certificatePath,
		ClientPrivateKeyPath:  keyPath,
	})

	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if !constructed {
		t.Fatal("secure OPC UA client was not constructed")
	}
	if readCount != 1 {
		t.Fatalf("Client Private Key reads = %d, want 1", readCount)
	}
}

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

	discoveryCalls := 0
	client := &gopcuaClient{getEndpoints: func(context.Context, string) ([]*ua.EndpointDescription, error) {
		discoveryCalls++
		return nil, errors.New("endpoint discovery should not run")
	}}
	err := client.Connect(context.Background(), secureConnectRequest(keyPath))

	if err == nil {
		t.Fatal("expected encrypted private key error")
	}
	if !strings.Contains(err.Error(), "Client Private Key is encrypted") || !strings.Contains(err.Error(), "unencrypted RSA private key") {
		t.Fatalf("error = %v", err)
	}
	if discoveryCalls != 0 {
		t.Fatalf("endpoint discovery calls = %d, want 0", discoveryCalls)
	}
}

func TestClientConnectRejectsMalformedAndNonRSAPrivateKeysBeforeEndpointDiscovery(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate EC key: %v", err)
	}
	ecDER, err := x509.MarshalPKCS8PrivateKey(ecKey)
	if err != nil {
		t.Fatalf("marshal EC key: %v", err)
	}

	tests := []struct {
		name     string
		contents []byte
		want     string
	}{
		{name: "malformed", contents: []byte("not a private key"), want: "unencrypted RSA private key in PKCS#1 or PKCS#8 PEM/DER format"},
		{name: "non-RSA", contents: pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: ecDER}), want: "must be an RSA private key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyPath := filepath.Join(t.TempDir(), "client-key.pem")
			if err := os.WriteFile(keyPath, tt.contents, 0o600); err != nil {
				t.Fatalf("write key: %v", err)
			}
			discoveryCalls := 0
			client := &gopcuaClient{getEndpoints: func(context.Context, string) ([]*ua.EndpointDescription, error) {
				discoveryCalls++
				return nil, errors.New("endpoint discovery should not run")
			}}

			err := client.Connect(context.Background(), secureConnectRequest(keyPath))

			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Connect() error = %v, want containing %q", err, tt.want)
			}
			if discoveryCalls != 0 {
				t.Fatalf("endpoint discovery calls = %d, want 0", discoveryCalls)
			}
		})
	}
}

func secureConnectRequest(keyPath string) ConnectRequest {
	return ConnectRequest{
		Endpoint:              "opc.tcp://localhost:4840",
		SecurityPolicy:        "Basic256Sha256",
		SecurityMode:          "SignAndEncrypt",
		AuthType:              AuthAnonymous,
		ClientCertificatePath: "C:/certs/client.pem",
		ClientPrivateKeyPath:  keyPath,
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
