# Protocol and landing roadmap

These are requested capabilities under development, not a declaration that the current preview has passed all of them.

1. SOCKS5
2. VLESS + REALITY
3. Hysteria2
4. Cloudflare Tunnel
5. Shadowsocks, including SS2022
6. VLESS Encryption + REALITY + Vision: X25519 or ML-KEM-768 authentication, each with native/xorpub/random appearance
7. VLESS Encryption
8. VMess + WebSocket
9. VLESS + WebSocket
10. AnyTLS + REALITY (sing-box)
11. AnyTLS + TLS (sing-box)
12. MTProto/MTProxy (mtg)
13. VLESS + XHTTP + REALITY (Xray, isolated path)
14. VLESS + XHTTP + REALITY + Encryption / applicable Vision
15. VLESS + XHTTP + TLS + H3 (QUIC/UDP, separate listener)

Landing: paste one or more proxy links, parse locally without fetching remote URLs, review/edit, save encrypted proxy credentials and bind an outbound/default route or explicit domain/CIDR route to a node. Unsupported kernel/protocol combinations must be rejected, not silently downgraded.

Core validation is required at the pinned fork versions, not merely against current website documentation. Cloudflare Tunnel is a separate ingress/connector capability, not arbitrary transparent UDP. MTProxy has different authentication and accounting primitives; do not present shared metrics as per-user bytes without an explicit attributable design.
