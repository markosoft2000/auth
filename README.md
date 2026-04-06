# 🛡️ Go Auth Service

A high-performance authentication service built with **Golang**, **gRPC**, and **PostgreSQL**. This service handles secure JWT issuance, refresh token rotation, and session tracking with IP validation.

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