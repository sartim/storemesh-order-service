package clients

import (
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func UnaryJWTInterceptor(secret, issuer, audience string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, request, reply any, conn *grpc.ClientConn, invoker grpc.UnaryInvoker, options ...grpc.CallOption) error {
		now := time.Now()
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"iss": issuer, "aud": audience,
			"iat": now.Unix(), "exp": now.Add(2 * time.Minute).Unix(),
		})
		signed, err := token.SignedString([]byte(secret))
		if err != nil {
			return err
		}
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+signed)
		return invoker(ctx, method, request, reply, conn, options...)
	}
}
