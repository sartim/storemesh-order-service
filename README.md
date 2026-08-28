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
