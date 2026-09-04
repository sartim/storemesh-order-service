# StoreMesh Order Service

The Order Service owns order lifecycle, customer association, line-item
snapshots, totals, and cancellation state. It coordinates Product and
Inventory services but does not own catalog or stock data.

The protobuf contract establishes order creation, retrieval, paginated listing,
and cancellation. `ListOrders` supports optional customer/status filters and
page-token pagination. Customer history should supply `customer_id`; an empty
filter is intended for an authorized administrative caller. Authorization
remains an edge/interceptor responsibility until the shared service
authorization interceptor is added.

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
Do not treat a cart as an order: carts are excluded from order metrics until
checkout creates an order.

Order creation also writes an `OrderCreated` record to `event_outbox` in the
same transaction as the order and its lines. A publisher/worker will later
deliver pending outbox records to Kafka and mark them published; no service
should publish directly before that worker exists.

The publisher is `cmd/outbox-publisher`. It requires `DATABASE_URL` and a
comma-separated `KAFKA_BROKERS` value, publishes order events to
`storemesh.order.events`, and marks an event published only after Kafka
acknowledges it. Run multiple workers only after adding leasing/claiming; the
current local worker is intentionally single-instance.

The publisher is optional. The standard Order Service image includes the
publisher binary, but the default Helm and Argo CD configuration does not run
it and does not install Kafka or Confluent for Kubernetes. This keeps order
creation fully functional with PostgreSQL alone while the outbox retains
events for later delivery. Enable the publisher only when an external Kafka
endpoint is available, by setting `kafka.enabled=true` and
`kafka.brokers=<host:port>` in an environment-specific Helm release.

## Run locally without Docker or Kubernetes

Requires Go 1.26.6 or newer. Without `DATABASE_URL`, the service uses in-memory
orders and carts and can be started without external dependencies:

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
