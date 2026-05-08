# 🛡️ Go Auth Service

A high-performance, production-ready authentication service is designed as a gRPC-based Identity Provider. It manages user registration, authentication, and token issuance (JWT) with refresh token rotation. It follows a clean architecture pattern, separating the transport layer (gRPC/HTTP) from the core business logic (Services) and data persistence (Storage).

### 🛠️ Key Technical Components (including SRE features)

1. **gRPC Interface**: The primary API is gRPC, featuring automated validation (via `protovalidate`) and standardized logging/recovery interceptors.
2. **Security-First Design**:
    - **Password Hashing**: Uses **Argon2**, the winner of the Password Hashing Competition, which is resistant to GPU-based cracking.
    - **Encryption**: Employs **AES-GCM** (Galois/Counter Mode) to encrypt sensitive data (like app-specific secrets) before storing them in the database.
    - **JWT EdDSA**: Uses asymmetric signing (Ed25519) for tokens, providing better security and performance with smaller keys compared to RSA.
3. **High Availability Storage**:
    - **PostgreSQL**: Configured with a strategy for Master/Replica separation (via HAProxy/PgBouncer comments in the code) to avoid single points of failure.
    - **Redis Cluster**: Uses a 6-node Redis cluster for high-performance caching of application metadata and session tokens.
4. **Observability**:
    - **Prometheus**: Exports metrics via an HTTP `/metrics` endpoint.
    - **Health Checks**: Implements both gRPC Health Checking and a standard HTTP `/health` endpoint that pings the underlying database.
    - **Graceful Shutdown**: The service handles `SIGTERM` and `SIGINT` signals to ensure that all database connections and servers close properly without losing data.

---

## 🚀 Features

- **gRPC API**: Fast and type-safe communication.
- **Argon2id Hashing**: Industry-standard secure password hashing.
- **JWT with EdDSA**: Asymmetric token signing using Ed25519 for secure verification.
- **Layered Caching**: Integration with **Redis Cluster** for low-latency lookups.
- **Database Encryption**: Sensitive application data is encrypted at rest using **AES-GCM**.
- **High Availability**: Designed to work with PostgreSQL replicas and Redis clusters.
- **Observability**: Built-in Prometheus metrics and comprehensive health checks.

---

## 🛠️ Tech Stack

- **Language**: Go 1.24+
- **Database**: PostgreSQL 14
- **Cache**: Redis 7 (Cluster Mode)
- **Communication**: gRPC & Protocol Buffers
- **Logging**: `slog` (Structured Logging)
- **Metrics**: Prometheus

---

## 🏗️ Architecture

The service follows a modular architecture:
- `cmd/server`: Entry point and dependency injection.
- `internal/app`: gRPC and HTTP server initializers.
- `internal/service`: Core business logic (Auth, Token management).
- `internal/storage`: Persistence layer implementations (Postgres, Redis).
- `internal/lib`: Shared utilities (Crypto, JWT, Hashers).

---

## 🚦 Getting Started

### Prerequisites
- Docker and Docker Compose
- Go 1.22+ (for local development)

### Running the environment
The service and all its dependencies (Postgres, 6-node Redis Cluster, Migrator) can be started with a single command:

```bash
docker-compose up -d
```

The gRPC server will be available at `localhost:50001` and the HTTP metrics/health at `localhost:8081`.

---

### Gen a private key for an app

To generate a secure Ed25519 key pair and prepare the private key for registration via the API:

```bash
# 1. Generate the private key
openssl genpkey -algorithm Ed25519 -out private.pem

# 2. Extract the public key (for client services)
openssl pkey -in private.pem -pubout -out public.pem

# 3. Prepare for API registration (Base64 encode the whole PEM file)
base64 -w 0 private.pem
```

Use public key to verify JWT tokens in gateway service

---

## 🌐 Handling Client IP Addresses

To ensure security and accurate logging, the service captures the user's real IP address. When deploying behind a reverse proxy (like **Nginx**, **HAProxy**, or a **Cloud Load Balancer**), the direct connection IP will be that of the proxy. 

To get the actual client IP, the service inspects the `X-Forwarded-For` header.

### Go Implementation Detail

The following helper function extracts the correct IP by checking for proxy headers before falling back to the remote address:

```go
import (
    "net"
    "net/http"
)

// GetIP extracts the real client IP address from the request.
func GetIP(r *http.Request) string {
    // 1. Check if the app is behind a proxy (e.g., Nginx, LB)
    ip := r.Header.Get("X-Forwarded-For")
    
    if ip == "" {
        // 2. Fallback to the direct connection IP if no proxy header exists
        ip, _, _ = net.SplitHostPort(r.RemoteAddr)
    }

    return ip
}
```

### Kafka topic creation:

```sh
kafka-configs --bootstrap-server localhost:9092 \
  --entity-type topics --entity-name auth-user-activity.v1 \
  --add-config min.insync.replicas=1,retention.ms=1200000,segment.ms=1200000
```

list of topics:

``` sh
sudo docker compose exec kafka-1 kafka-topics --bootstrap-server kafka-1:29092 --list
```

describe topic:

``` sh
sudo docker compose exec kafka-1 kafka-topics --bootstrap-server kafka-1:29092 --describe --topic auth-user-activity-v1
```

consume topic:

``` sh
sudo docker compose exec kafka-1 kafka-console-consumer --bootstrap-server kafka-1:29092 --topic auth-user-activity-v1 --from-beginning
```

---

### Auto delete of refresh tokens expired more than N months ago

enabled in your PostgreSQL configuration (postgresql.conf):
```
shared_preload_libraries = 'pg_cron'
cron.database_name = 'auth_db' # OR replace with your actual DB name
```

add in postgres cli

``` sql
CREATE INDEX CONCURRENTLY idx_refresh_tokens_expires_at ON refresh_tokens(expires_at);
-- Enable the extension
CREATE EXTENSION IF NOT EXISTS pg_cron;
```

Schedule the deletion of tokens that expired more than a month ago. running daily at midnight.

``` sql
SELECT cron.schedule('cleanup-old-refresh-tokens', '0 0 * * *', $$
    DELETE FROM refresh_tokens 
    WHERE expires_at < NOW() - INTERVAL '1 month'
$$);
```