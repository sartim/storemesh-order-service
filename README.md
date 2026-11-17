# StoreMesh Order Service

The Order Service owns order lifecycle, customer association, line-item
snapshots, totals, and cancellation state. It coordinates Product and
Inventory services but does not own catalog or stock data.

The protobuf contract establishes order creation, retrieval, paginated listing,
and cancellation. `ListOrders` supports optional customer/status filters and
page-token pagination. When Keycloak OIDC is configured, the gRPC interceptor
propagates the verified token subject and realm `admin` role into request
context. Customer order lists are scoped to that subject by default; attempts
to supply another customer ID are denied. Create, get, and cancel operations
enforce ownership in the service layer, and cross-customer order lookups return
`NotFound` to avoid disclosing another customer's order. Only the realm `admin`
role can perform broad order listing or access another customer's order.

The initial PostgreSQL schema is available in `migrations/001_orders.sql`.
Order lines preserve the product price snapshot used to calculate historical
order totals.

## Cart boundary

The repository now contains the versioned `CartService` protobuf contract in
`proto/storemesh/order/v1/cart.proto`. A cart is customer-owned and contains
only product IDs and quantities; it is deliberately separate from `Order`, so
cart contents do not appear in sales metrics or order history. The generated
Go stubs are committed under `gen/storemesh/order/v1`.

Persistent PostgreSQL storage, gRPC server registration, BFF REST routes, and
cart-to-order checkout orchestration are being delivered incrementally. The
PostgreSQL migration is `migrations/003_carts.sql`; when `DATABASE_URL` is not
configured, the service uses an in-memory cart store for local development.
With Keycloak OIDC enabled, cart get/update/clear operations are restricted to
the authenticated token subject, including for admin tokens.
Do not treat a cart as an order: carts are excluded from order metrics until
checkout creates an order.

Order creation also writes an `OrderCreated` record to `event_outbox` in the
same transaction as the order and its lines. The optional publisher/worker
delivers pending outbox records to Kafka and marks them published only after
Kafka acknowledges them; no request handler publishes directly.

Customer cart replacements and clears use the same transactional outbox. They
emit `CartUpdated` and `CartCleared` records to `storemesh.cart.events`; order
events continue to use `storemesh.order.events`. This keeps cart changes out
of order analytics while allowing a future cart projection to consume them.

The publisher is `cmd/outbox-publisher`. It requires `DATABASE_URL` and a
comma-separated `KAFKA_BROKERS` value, publishes order events to
`storemesh.order.events`, and marks an event published only after Kafka
acknowledges it. Each worker claims rows with a 30-second PostgreSQL lease
using `FOR UPDATE SKIP LOCKED`; an expired lease can be reclaimed after a
worker crash. Delivery remains at-least-once, so downstream consumers must be
idempotent. Set `OUTBOX_WORKER_ID` to a stable unique value when running more
than one worker; the hostname is used by default.

The publisher is optional. The standard Order Service image includes the
publisher binary, but the default Helm and Argo CD configuration does not run
it and does not install Kafka or Confluent for Kubernetes. This keeps order
creation fully functional with PostgreSQL alone while the outbox retains
events for later delivery. Enable the publisher only when an external Kafka
endpoint is available, by setting `kafka.enabled=true` and
`kafka.brokers=<host:port>` in an environment-specific Helm release.

## Run locally without Docker or Kubernetes

Requires Go 1.26.6 or newer. Without `DATABASE_URL`, the service uses in-memory
orders and carts and can be started without external dependencies. Run the
process from this repository:

```sh
go run ./cmd/server
```

Run it beside Product and Inventory as a local process by assigning unique
ports:

```sh
GRPC_ADDR=:50053 METRICS_ADDR=:8083 \
PRODUCT_SERVICE_ADDRESS=localhost:50051 \
INVENTORY_SERVICE_ADDRESS=localhost:50052 \
go run ./cmd/server
```

Coordinated persistent orders require `DATABASE_URL`, reachable Product and
Inventory gRPC addresses, and the applicable migrations. The Kafka publisher
is not started by `go run ./cmd/server`; run `go run ./cmd/outbox-publisher`
only when an external broker is intentionally available.
