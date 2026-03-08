# End-to-End Encryption — Threat Model

## Overview

Enlace's optional E2E encryption mode provides zero-knowledge guarantees: when enabled on a share, all files are encrypted client-side before upload. The server stores only ciphertext and never has access to plaintext or encryption keys.

## Cryptographic Design

| Property | Value |
|----------|-------|
| Algorithm | AES-GCM (Galois/Counter Mode) |
| Key length | 256 bits |
| IV length | 96 bits (12 bytes), unique per file |
| Key derivation | None — keys are randomly generated via `crypto.subtle.generateKey()` |
| Key transport | Base64url-encoded in URL fragment (`#key=...`) |

## Assets

| Asset | Sensitivity | Location |
|-------|-------------|----------|
| Plaintext files | High | Client browser memory (transient) |
| Encryption key | Critical | URL fragment (never sent to server) |
| Ciphertext | Low | Server storage (local/S3) |
| Encrypted metadata | Low | Server database (filename + MIME type) |
| IV (nonce) | Public | Server database |

## Threat Actors

1. **Compromised server / server operator** — Has access to database and storage.
2. **Network attacker (MitM)** — Can intercept network traffic (mitigated by TLS).
3. **Malicious recipient** — Has the share URL including key fragment.
4. **Compromised client** — Attacker controls the user's browser or device.

## Threat Analysis

### What E2E encryption protects against

| Threat | Mitigation |
|--------|------------|
| Server data breach | Server only stores ciphertext; no keys present |
| Server operator viewing files | Operator cannot decrypt without the URL fragment |
| Storage backend compromise | S3/local storage contains only encrypted blobs |
| Database leak | File metadata (names, types) is encrypted; only IVs are plaintext |
| Network eavesdropping on storage | Files are encrypted before leaving the browser |

### What E2E encryption does NOT protect against

| Threat | Reason |
|--------|--------|
| Compromised client device | If the browser is compromised, the attacker can read plaintext before encryption or after decryption |
| Key leakage via URL sharing | If the full URL (including `#key=...`) is shared insecurely, the key is exposed |
| Browser extension access | Extensions with page access can read the key from the URL fragment |
| Shoulder surfing | Physical observation of the share URL reveals the key |
| Future quantum attacks | AES-256 has post-quantum resilience for symmetric encryption, but this should be reassessed |
| Malicious JavaScript injection | If the server is compromised and serves malicious frontend code, it could exfiltrate keys (supply-chain attack) |

### Trust Boundaries

```
┌─────────────────────────────────────────────────────┐
│                    CLIENT BROWSER                    │
│                                                     │
│  ┌───────────┐    ┌──────────┐    ┌──────────────┐ │
│  │ Plaintext │───>│ Crypto   │───>│ Ciphertext   │ │
│  │ Files     │    │ Module   │    │ (ready to    │ │
│  └───────────┘    │ (AES-GCM)│    │  upload)     │ │
│                   └──────────┘    └──────┬───────┘ │
│  ┌───────────┐         ▲                 │         │
│  │ URL #key  │─────────┘                 │         │
│  │ (never    │                           │         │
│  │  leaves)  │                           │         │
│  └───────────┘                           │         │
└──────────────────────────────────────────┼─────────┘
                    TRUST BOUNDARY         │
═══════════════════════════════════════════╪═════════
                                           │ HTTPS
┌──────────────────────────────────────────┼─────────┐
│                 ENLACE SERVER            │         │
│                                          ▼         │
│  ┌──────────────┐    ┌────────────────────────┐   │
│  │ Database     │    │ Storage (Local / S3)    │   │
│  │ - IVs        │    │ - Encrypted blobs only  │   │
│  │ - Enc. meta  │    │ - No plaintext          │   │
│  │ - No keys    │    │ - No keys               │   │
│  └──────────────┘    └────────────────────────┘   │
└────────────────────────────────────────────────────┘
```

## URL Fragment Security

The encryption key is transported in the URL fragment (the part after `#`). Per RFC 3986, browsers do not include fragments in HTTP requests. This means:

- ✅ The key is never sent to the server
- ✅ The key is not included in HTTP Referrer headers
- ✅ The key is not logged in server access logs
- ⚠️ The key IS visible in the browser address bar
- ⚠️ The key IS stored in browser history
- ⚠️ The key CAN be read by JavaScript on the page (including extensions)

## Recommendations for Users

1. Share E2E-encrypted share URLs only through secure channels (encrypted messaging, password managers).
2. Do not share the URL via unencrypted email if the threat model includes email compromise.
3. Save the share URL (including key) in a password manager — if lost, files cannot be recovered.
4. Clear browser history if the device is shared or potentially compromised.

## Security Review Checklist

- [ ] AES-GCM implementation uses Web Crypto API (no custom crypto)
- [ ] IVs are generated using `crypto.getRandomValues()` (CSPRNG)
- [ ] Each file gets a unique IV (IV reuse with same key breaks AES-GCM)
- [ ] Keys are generated with `crypto.subtle.generateKey()` (CSPRNG)
- [ ] No encryption keys are transmitted to or stored on the server
- [ ] Encrypted metadata prevents server from learning filenames/types
- [ ] Feature is opt-in (disabled by default via `E2E_ENCRYPTION_ENABLED`)
- [ ] UI clearly warns users about irreversible key loss
