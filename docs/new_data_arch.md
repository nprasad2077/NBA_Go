# Database Enhancement

Currently, the entire applicaction operates from a single DB/connection for both reads and writes.

I am developing a new solution, decoupled from the current project, that aims to convert the single DB setup into a HA multi read-replica server architecture.

The only changes that need to be made to this Go Fiber backend is which DB/connection to use for write operations and which oness are for read.

The write operations should point to the write-ingress (port 5437). Read operations should use the Read Ingress (port 5438).

How can this be seamlessly integrated into this application?

---

The current method  being used to collect metrics/stats/observability needs a revamp. Examine the current setup and how we can collect metrics/logs/etc... using this new architecture.

---

Feel free to make use of any agents, tools, skills, etc to complete this initial research and planning task.

```mermaid
`flowchart TD
    subgraph Client_Tier["Client application / microservices"]
        App["Application code<br/>Dual connection pool"]
    end

    subgraph Routing_Tier["HAProxy 2.9 — routing and proxy"]
        WriteIngress["Write ingress<br/>:5437 · TCP pass-through"]
        ReadIngress["Read ingress<br/>:5438 · Round-robin load balancer"]
        Stats["HAProxy Stats UI<br/>:7080 · Live connection metrics"]
    end

    subgraph Cluster_Tier["PostgreSQL 17 replicated cluster"]
        Primary[("pg_primary · :5434<br/>Read/write primary · WAL source<br/>Replication slots: replica1_slot, replica2_slot")]
        Replica1[("pg_replica1 · :5435<br/>Read-only hot standby<br/>Uses replica1_slot · feedback on")]
        Replica2[("pg_replica2 · :5436<br/>Read-only hot standby<br/>Uses replica2_slot · feedback on")]
    end

    App -->|Writes / transactions| WriteIngress
    App -->|Read queries| ReadIngress

    WriteIngress -->|TCP forward| Primary
    ReadIngress -->|Round robin| Replica1
    ReadIngress -->|Round robin| Replica2

    Primary -.->|Physical WAL streaming| Replica1
    Primary -.->|Physical WAL streaming| Replica2
```
