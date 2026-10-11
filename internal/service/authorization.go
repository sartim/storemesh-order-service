package service

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	auth "storemesh-order-service/internal/auth"
)

func requireCustomer(ctx context.Context, customerID string) error {
	identity, authenticated := auth.IdentityFromContext(ctx)
	if !authenticated || identity.Admin {
		return nil
	}
	return requireIdentityOwner(identity, customerID)
}

func requireIdentityOwner(identity auth.Identity, customerID string) error {
	if customerID == "" || customerID != identity.Subject {
		return status.Error(codes.PermissionDenied, "customer access is restricted to the authenticated user")
	}
	return nil
}

func requireCartCustomer(ctx context.Context, customerID string) error {
	identity, authenticated := auth.IdentityFromContext(ctx)
	if !authenticated {
		return nil
	}
	return requireIdentityOwner(identity, customerID)
}

func requireOrderOwner(ctx context.Context, customerID string) error {
	identity, authenticated := auth.IdentityFromContext(ctx)
	if !authenticated || identity.Admin || customerID == identity.Subject {
		return nil
	}
	// Do not disclose whether an order belonging to another customer exists.
	return status.Error(codes.NotFound, "order not found")
}

func scopedCustomerID(ctx context.Context, requested string) (string, error) {
	identity, authenticated := auth.IdentityFromContext(ctx)
	if !authenticated || identity.Admin {
		return requested, nil
	}
	if requested != "" && requested != identity.Subject {
		return "", status.Error(codes.PermissionDenied, "customer access is restricted to the authenticated user")
	}
	return identity.Subject, nil
}
