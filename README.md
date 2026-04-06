# auth

Go Implementation Detail
When you insert the IP from your Go code, ensure you handle the X-Forwarded-For header if your app is behind a proxy (like Nginx or a Load Balancer), otherwise, you will only see the proxy's IP.

func GetIP(r *http.Request) string {
    // Check if behind a proxy
    ip := r.Header.Get("X-Forwarded-For")
    if ip == "" {
        // Fallback to direct connection IP
        ip, _, _ = net.SplitHostPort(r.RemoteAddr)
    }

    return ip
}