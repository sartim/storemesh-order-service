# StoreMesh Order Service

The Order Service owns order lifecycle, customer association, line-item
snapshots, totals, and cancellation state. It coordinates Product and
Inventory services but does not own catalog or stock data.

The initial protobuf contract establishes order creation, retrieval, and
cancellation. Runtime implementation follows contract validation.
