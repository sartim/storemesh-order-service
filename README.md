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
cart-to-order checkout orchestration are the next implementation pieces. Do
not treat the presence of the generated contract as evidence that cross-device
cart synchronization is available yet.
