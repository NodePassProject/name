# Name Package

[![Go Reference](https://pkg.go.dev/badge/github.com/NodePassProject/name.svg)](https://pkg.go.dev/github.com/NodePassProject/name)
[![License](https://img.shields.io/badge/License-BSD_3--Clause-blue.svg)](https://opensource.org/licenses/BSD-3-Clause)

A high-performance DNS resolver with intelligent caching and background refresh capabilities for Go applications.

## Table of Contents

- [Features](#features)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Usage](#usage)
  - [Creating a Resolver](#creating-a-resolver)
  - [Resolving Addresses](#resolving-addresses)
  - [Cache Management](#cache-management)
  - [TTL Configuration](#ttl-configuration)
- [Caching Strategy](#caching-strategy)
  - [Background Refresh](#background-refresh)
  - [Cache Expiration](#cache-expiration)
- [DNS Server Configuration](#dns-server-configuration)
- [Advanced Usage](#advanced-usage)
- [Performance Considerations](#performance-considerations)
- [Troubleshooting](#troubleshooting)
- [Best Practices](#best-practices)
  - [Resolver Configuration](#1-resolver-configuration)
  - [Cache Management](#2-cache-management)
  - [Error Handling and Monitoring](#3-error-handling-and-monitoring)
  - [Production Deployment](#4-production-deployment)
  - [Performance Optimization](#5-performance-optimization)
  - [Testing and Development](#6-testing-and-development)
- [License](#license)

## Features

- **Intelligent caching** with configurable TTL (Time To Live)
- **Background refresh** to maintain cache freshness without blocking queries
- **Custom DNS server support** with automatic failover and round-robin
- **Thread-safe operations** with concurrent cache access
- **IPv4/IPv6 support** with network-specific address resolution
- **Direct IP handling** for optimal performance when IP addresses are provided
- **Automatic cache expiration** and cleanup
- **Dynamic TTL adjustment** for flexible cache management
- **DNS server round-robin** for load distribution and reliability
- **Comprehensive cache inspection** and management APIs

## Installation

```bash
go get github.com/NodePassProject/name
```

## Quick Start

```go
package main

import (
    "fmt"
    "log"
    "time"
    "github.com/NodePassProject/name"
)

func main() {
    resolver := name.NewResolver(5*time.Minute, nil)
    addr, err := resolver.ResolveTCPAddr("tcp", "example.com:80")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Resolved address: %s\n", addr)
}
```

## Usage

### Creating a Resolver

```go
// Using system default DNS servers
resolver := name.NewResolver(5*time.Minute, nil)

// Using custom DNS servers
customDNS := []string{"8.8.8.8", "8.8.4.4", "1.1.1.1"}
resolver = name.NewResolver(5*time.Minute, customDNS)
```

**Parameters:**
- `ttl`: Cache duration (default 5 minutes if ≤ 0)
- `dnsServers`: DNS server IPs (port :53 added automatically)

### Resolving Addresses

```go
resolver := name.NewResolver(5*time.Minute, nil)

// TCP address
tcpAddr, err := resolver.ResolveTCPAddr("tcp", "example.com:443")

// UDP address
udpAddr, err := resolver.ResolveUDPAddr("udp", "dns.google:53")

// IPv4-only
tcp4Addr, err := resolver.ResolveTCPAddr("tcp4", "example.com:80")
```

**Network Types:** `tcp`, `tcp4`, `tcp6`, `udp`, `udp4`, `udp6`
- `tcp4`/`udp4`: IPv4 only
- `tcp6`/`udp6`: IPv6 only

### Cache Management

```go
resolver := name.NewResolver(5*time.Minute, nil)
resolver.ResolveTCPAddr("tcp", "example.com:80")

// Get cached IPs
ips, found := resolver.GetCachedIPs("example.com")

// Cache count
count := resolver.CacheCount()

// Clear specific host or all cache
resolver.ClearHost("example.com")
resolver.ClearCache()

// Refresh all entries
resolver.RefreshCache()
```

### TTL Configuration

```go
resolver := name.NewResolver(5*time.Minute, nil)

// Get current TTL
currentTTL := resolver.GetTTL()

// Change TTL (affects new entries only)
resolver.SetTTL(10 * time.Minute)
```

## Caching Strategy

The resolver implements a sophisticated two-tier caching strategy for optimal performance and freshness:

### Background Refresh

- **Stale Threshold**: 80% of TTL
- **Mechanism**: At 80% TTL, background goroutine refreshes without blocking
- **Timeline**: New data cached before old entries expire

### Cache Expiration

Cache entries are automatically refreshed at 80% TTL. Expired entries trigger fresh DNS lookups.

## DNS Server Configuration

### Round-Robin Load Balancing

Queries are automatically distributed across configured DNS servers in round-robin fashion.

### Automatic Failover & Timeout

- Automatically tries next server if current fails
- **Default timeout**: 5 seconds per server
- **Protocol**: UDP

## Advanced Usage

### Custom Error Handling

```go
addr, err := resolver.ResolveTCPAddr("tcp", "example.com:80")
if err != nil {
    if strings.Contains(err.Error(), "lookup failed") {
        // Handle lookup failure
    } else if strings.Contains(err.Error(), "no IP addresses found") {
        // Handle no IPs
    }
}
```

### Periodic Cache Refresh

```go
go func() {
    ticker := time.NewTicker(3 * time.Minute)
    defer ticker.Stop()
    for range ticker.C {
        resolver.RefreshCache()
    }
}()
```

### Multiple Resolvers

```go
fastResolver := name.NewResolver(1*time.Minute, []string{"8.8.8.8"})
standardResolver := name.NewResolver(5*time.Minute, nil)
stableResolver := name.NewResolver(30*time.Minute, []string{"1.1.1.1"})
```

## Performance Considerations

### Cache Hit Rates

| TTL Duration | Cache Hit Rate | Best For |
|--------------|---|---|
| 1 minute | ~60-70% | Dynamic hosts |
| 5 minutes | ~80-90% | General purpose |
| 15 minutes | ~90-95% | Stable infrastructure |
| 30+ minutes | ~95-99% | Static hosts |

### Memory Usage

~500 bytes per cached hostname. Monitor with `CacheCount()` to prevent growth.

### Background Refresh Impact

- Minimal CPU overhead
- One DNS query per entry at 80% TTL
- Eliminates lookup delays for cached hosts

## Troubleshooting

### Common Issues

| Issue | Solution |
|-------|----------|
| DNS lookup fails | Check network connectivity; test with public DNS (8.8.8.8) |
| No IP addresses found | Verify hostname has A/AAAA records |
| IPv4/IPv6 not found | Use generic `tcp`/`udp` instead of specific versions |
| Stale cache data | Reduce TTL or use `ClearHost()` |
| High memory usage | Reduce TTL or call `ClearCache()` periodically |

### Quick Debug

```go
// Monitor cache activity
count := resolver.CacheCount()
ttl := resolver.GetTTL()
log.Printf("Cache: %d entries, TTL: %s", count, ttl)

// Clear cache if needed
resolver.ClearHost("problematic-host.com")
```

## Best Practices

### 1. Resolver Configuration

```go
// Choose TTL based on hostname stability
dynamicResolver := name.NewResolver(2*time.Minute, nil)     // Dynamic hosts
standardResolver := name.NewResolver(5*time.Minute, nil)    // General use
stableResolver := name.NewResolver(15*time.Minute, nil)     // Stable infra

// Use multiple DNS servers in production
productionDNS := []string{"8.8.8.8", "8.8.4.4", "1.1.1.1"}
resolver := name.NewResolver(5*time.Minute, productionDNS)
```

### 2. Cache Management

```go
// GOOD: Reuse resolver instance
type App struct {
    resolver *name.Resolver
}

func NewApp() *App {
    return &App{resolver: name.NewResolver(5*time.Minute, nil)}
}

// BAD: Don't create resolver for each operation
func badExample() {
    resolver := name.NewResolver(5*time.Minute, nil) // Defeats caching!
}

// Monitor and maintain cache
go func() {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()
    for range ticker.C {
        app.resolver.RefreshCache()
        if app.resolver.CacheCount() > maxSize {
            app.resolver.ClearCache()
        }
    }
}()
```

### 3. Error Handling

```go
addr, err := resolver.ResolveTCPAddr("tcp", "example.com:80")
if err != nil {
    log.Printf("Resolve failed: %v", err)
    // Implement retry or fallback
}
```

### 4. Production Setup

```go
resolver := name.NewResolver(5*time.Minute, 
    []string{"8.8.8.8", "8.8.4.4", "1.1.1.1"})

// Background maintenance
go func() {
    ticker := time.NewTicker(3 * time.Minute)
    for range ticker.C {
        resolver.RefreshCache()
    }
}()
```

### 5. Testing

```go
func TestResolve(t *testing.T) {
    resolver := name.NewResolver(5*time.Minute, nil)
    addr, err := resolver.ResolveTCPAddr("tcp", "google.com:80")
    if err != nil {
        t.Fatal(err)
    }
    if addr == nil {
        t.Fatal("Expected address")
    }
}
```

## License

Copyright (c) 2025, NodePassProject. Licensed under the BSD 3-Clause License.
See the [LICENSE](LICENSE) file for details.
