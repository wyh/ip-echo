package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"
	"time"
)

var cgnatPrefix = netip.MustParsePrefix("100.64.0.0/10")

func isPublicIP(addr netip.Addr) bool {
	if !addr.IsValid() {
		return false
	}
	addr = addr.Unmap()
	if addr.IsPrivate() ||
		addr.IsLoopback() ||
		addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() ||
		addr.IsMulticast() ||
		addr.IsUnspecified() {
		return false
	}
	// 过滤阿里云 SLB/内网常用的 100.64.0.0/10 (RFC 6598 CGNAT)
	if addr.Is4() && cgnatPrefix.Contains(addr) {
		return false
	}
	return true
}

func parseIP(raw string) (netip.Addr, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return netip.Addr{}, false
	}
	if host, _, err := net.SplitHostPort(s); err == nil {
		s = host
	}
	s = strings.Trim(s, "[]")
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Addr{}, false
	}
	return addr.Unmap(), true
}

func extractClientIP(r *http.Request) string {
	// 1. 优先从 X-Forwarded-For 从左到右取第一个公网 IP
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		for _, part := range strings.Split(xff, ",") {
			if addr, ok := parseIP(part); ok && isPublicIP(addr) {
				return addr.String()
			}
		}
	}

	// 2. 依次检查常见代理头
	for _, h := range []string{"X-Real-IP", "Ali-Cdn-Real-Ip", "CF-Connecting-IP"} {
		if addr, ok := parseIP(r.Header.Get(h)); ok && isPublicIP(addr) {
			return addr.String()
		}
	}

	// 3. 兜底使用直连 RemoteAddr（优先公网，其次任意有效 IP）
	if addr, ok := parseIP(r.RemoteAddr); ok {
		return addr.String()
	}
	return ""
}

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:18080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("/debug", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = fmt.Fprintf(
			w,
			"resolved_ip=%s\nremote_addr=%s\nx_forwarded_for=%s\nx_real_ip=%s\nali_cdn_real_ip=%s\n",
			extractClientIP(r),
			r.RemoteAddr,
			r.Header.Get("X-Forwarded-For"),
			r.Header.Get("X-Real-IP"),
			r.Header.Get("Ali-Cdn-Real-Ip"),
		)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ip := extractClientIP(r)
		if ip == "" {
			http.Error(w, "unable to determine client ip\n", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(ip + "\n"))
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      2 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("ip-echo listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
