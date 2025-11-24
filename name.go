// Package name 提供带缓存和后台刷新的DNS解析功能
package name

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

const (
	dnsTimeout       = 5 * time.Second
	defaultTTL       = 5 * time.Minute
	defaultDNSPort   = ":53"
	refreshThreshold = 80
)

// cacheEntry 缓存条目
type cacheEntry struct {
	ips     []net.IP  // 解析的IP地址
	expires time.Time // 缓存过期时间
	stale   time.Time // 后台刷新时间
}

// Resolver DNS解析器结构体
type Resolver struct {
	cache       sync.Map      // 缓存映射
	ttl         time.Duration // 缓存时间
	dnsServers  []string      // DNS地址组
	serverIndex uint32        // 轮询索引
	netResolver *net.Resolver // 底层解析器
}

// NewResolver 创建新的DNS解析器
func NewResolver(ttl time.Duration, dnsServers []string) *Resolver {
	if ttl <= 0 {
		ttl = defaultTTL
	}

	resolver := &Resolver{
		ttl:        ttl,
		dnsServers: dnsServers,
	}

	// 配置自定义DNS服务器
	if len(dnsServers) > 0 {
		resolver.netResolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				var lastErr error
				// 轮询DNS服务器
				for range dnsServers {
					idx := atomic.AddUint32(&resolver.serverIndex, 1) - 1
					server := dnsServers[int(idx)%len(dnsServers)] + defaultDNSPort

					dialCtx, dialCancel := context.WithTimeout(context.Background(), dnsTimeout)
					dialer := &net.Dialer{}
					conn, err := dialer.DialContext(dialCtx, "udp", server)
					dialCancel()

					if err == nil {
						return conn, nil
					}
					lastErr = err
				}
				return nil, fmt.Errorf("all DNS servers failed: %w", lastErr)
			},
		}
	} else {
		resolver.netResolver = &net.Resolver{PreferGo: true}
	}

	return resolver
}

// lookupHost 执行DNS解析，支持缓存和后台刷新
func (r *Resolver) lookupHost(host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}

	now := time.Now()

	if entry, ok := r.cache.Load(host); ok {
		ce := entry.(*cacheEntry)
		if now.Before(ce.expires) {
			// 后台刷新：超过阈值时触发
			if now.After(ce.stale) {
				go r.resolveDNS(host)
			}
			return ce.ips, nil
		}
		// 缓存过期
		r.cache.Delete(host)
	}

	return r.resolveDNS(host)
}

// resolveDNS 执行实际的DNS解析并缓存结果
func (r *Resolver) resolveDNS(host string) ([]net.IP, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dnsTimeout)
	defer cancel()

	ips, err := r.netResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("resolveDNS: lookup failed for %s: %w", host, err)
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("resolveDNS: no IP addresses found for %s", host)
	}

	// 缓存结果
	now := time.Now()
	r.cache.Store(host, &cacheEntry{
		ips:     ips,
		expires: now.Add(r.ttl),
		stale:   now.Add(r.ttl * refreshThreshold / 100),
	})

	return ips, nil
}

// resolveAddr 解析网络地址
func (r *Resolver) resolveAddr(network, address string) (net.IP, int, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, 0, fmt.Errorf("resolveAddr: invalid address %s: %w", address, err)
	}

	portNum, err := strconv.Atoi(port)
	if err != nil {
		return nil, 0, fmt.Errorf("resolveAddr: invalid port %s: %w", port, err)
	}

	ips, err := r.lookupHost(host)
	if err != nil {
		return nil, 0, err
	}

	var selectedIP net.IP
	switch network {
	case "tcp4", "udp4":
		for _, ip := range ips {
			if ip.To4() != nil {
				selectedIP = ip
				break
			}
		}
		if selectedIP == nil {
			return nil, 0, fmt.Errorf("resolveAddr: no IPv4 address found for %s", host)
		}
	case "tcp6", "udp6":
		for _, ip := range ips {
			if ip.To4() == nil && ip.To16() != nil {
				selectedIP = ip
				break
			}
		}
		if selectedIP == nil {
			return nil, 0, fmt.Errorf("resolveAddr: no IPv6 address found for %s", host)
		}
	default:
		selectedIP = ips[0]
	}

	return selectedIP, portNum, nil
}

// ResolveTCPAddr 解析TCP地址
func (r *Resolver) ResolveTCPAddr(network, address string) (*net.TCPAddr, error) {
	ip, port, err := r.resolveAddr(network, address)
	if err != nil {
		return nil, err
	}
	return &net.TCPAddr{IP: ip, Port: port}, nil
}

// ResolveUDPAddr 解析UDP地址
func (r *Resolver) ResolveUDPAddr(network, address string) (*net.UDPAddr, error) {
	ip, port, err := r.resolveAddr(network, address)
	if err != nil {
		return nil, err
	}
	return &net.UDPAddr{IP: ip, Port: port}, nil
}

// ClearCache 清除所有缓存条目
func (r *Resolver) ClearCache() {
	r.cache.Range(func(key, value interface{}) bool {
		r.cache.Delete(key)
		return true
	})
}

// ClearHost 清除指定主机的缓存
func (r *Resolver) ClearHost(host string) {
	r.cache.Delete(host)
}

// GetCachedIPs 获取指定主机的缓存IP
func (r *Resolver) GetCachedIPs(host string) ([]net.IP, bool) {
	entry, ok := r.cache.Load(host)
	if !ok {
		return nil, false
	}

	ce := entry.(*cacheEntry)
	if time.Now().After(ce.expires) {
		r.cache.Delete(host)
		return nil, false
	}

	return ce.ips, true
}

// RefreshCache 刷新所有缓存条目
func (r *Resolver) RefreshCache() {
	hosts := make([]string, 0)
	r.cache.Range(func(key, value any) bool {
		hosts = append(hosts, key.(string))
		return true
	})

	for _, host := range hosts {
		go r.resolveDNS(host)
	}
}

// CacheCount 返回当前缓存条目数量
func (r *Resolver) CacheCount() int {
	count := 0
	r.cache.Range(func(key, value any) bool {
		count++
		return true
	})
	return count
}

// SetTTL 动态调整TTL配置
func (r *Resolver) SetTTL(ttl time.Duration) {
	r.ttl = ttl
}

// GetTTL 获取当前TTL配置
func (r *Resolver) GetTTL() time.Duration {
	return r.ttl
}
