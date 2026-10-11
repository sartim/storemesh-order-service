package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestValidatorRefreshesJWKSForRotatedKey(t *testing.T) {
	initial, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.RWMutex
	activeKey, activeKID := &initial.PublicKey, "initial"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.RLock()
		defer mu.RUnlock()
		if r.URL.Path == "/.well-known/openid-configuration" {
			_ = json.NewEncoder(w).Encode(map[string]string{"issuer": serverIssuer(r), "jwks_uri": "http://" + r.Host + "/keys"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{"kid": activeKID, "kty": "RSA", "n": base64.RawURLEncoding.EncodeToString(activeKey.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(activeKey.E)).Bytes())}}})
	}))
	defer server.Close()
	issuer := server.URL
	validator, err := NewValidator(issuer, "storemesh-bff")
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	activeKey, activeKID = &rotated.PublicKey, "rotated"
	mu.Unlock()
	claims := &Claims{RegisteredClaims: jwt.RegisteredClaims{Issuer: issuer, Subject: "customer-1", Audience: jwt.ClaimStrings{"storemesh-bff"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute))}}
	claims.RealmAccess.Roles = []string{"admin"}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "rotated"
	raw, err := token.SignedString(rotated)
	if err != nil {
		t.Fatal(err)
	}
	validated, err := validator.ValidateClaims(raw)
	if err != nil {
		t.Fatalf("validator did not accept rotated signing key: %v", err)
	}
	if validated.Subject != "customer-1" || len(validated.RealmAccess.Roles) != 1 || validated.RealmAccess.Roles[0] != "admin" {
		t.Fatalf("validator claims = %#v", validated)
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+raw))
	_, err = UnaryInterceptor(validator)(ctx, nil, &grpc.UnaryServerInfo{}, func(ctx context.Context, _ any) (any, error) {
		identity, ok := IdentityFromContext(ctx)
		if !ok || identity.Subject != "customer-1" || !identity.Admin {
			t.Fatalf("middleware identity = %#v, present=%v", identity, ok)
		}
		return nil, nil
	})
	if err != nil {
		t.Fatalf("OIDC middleware failed: %v", err)
	}
}

func serverIssuer(r *http.Request) string { return "http://" + r.Host }
