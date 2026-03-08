# End-to-End Encryption — Key Model

## Key Lifecycle

### 1. Generation

When a user creates an E2E-encrypted share, a 256-bit AES-GCM key is generated client-side:

```javascript
const key = await crypto.subtle.generateKey(
  { name: "AES-GCM", length: 256 },
  true, // extractable for URL embedding
  ["encrypt", "decrypt"]
);
```

The key is generated using the browser's cryptographically secure random number generator (CSPRNG).

### 2. Export & Embedding

The key is exported as raw bytes, base64url-encoded, and appended to the share URL as a fragment:

```
https://enlace.example.com/s/abc123#key=dGhpcyBpcyBhIHRlc3Qga2V5
```

The URL fragment (`#key=...`) is never sent to the server — this is enforced by the HTTP protocol (RFC 3986 §3.5).

### 3. Usage

For each file uploaded to the share:

1. A unique 12-byte IV is generated via `crypto.getRandomValues()`
2. File content is encrypted: `AES-GCM(key, iv, plaintext) → ciphertext`
3. File metadata (name, MIME type) is separately encrypted with its own IV
4. The IV and encrypted metadata are sent to the server as form fields
5. The ciphertext is uploaded as the file content

### 4. Decryption

When a recipient accesses the share URL:

1. The key is extracted from `window.location.hash`
2. The key is imported via `crypto.subtle.importKey()`
3. For each file, the ciphertext is downloaded from the server
4. The file is decrypted using the stored IV: `AES-GCM(key, iv, ciphertext) → plaintext`
5. The metadata is decrypted to recover the original filename and MIME type
6. The plaintext file is offered for download/preview

## Key Sharing

The encryption key is shared implicitly through the share URL. Anyone with the full URL (including the `#key=...` fragment) can decrypt the files. This is analogous to how services like Firefox Send and Bitwarden Send operate.

### Sharing channels

| Channel | Security | Recommended |
|---------|----------|-------------|
| Encrypted messaging (Signal, WhatsApp) | High | ✅ Yes |
| Password manager shared vault | High | ✅ Yes |
| Face-to-face / QR code | High | ✅ Yes |
| Unencrypted email | Low | ⚠️ Only if email compromise is not in threat model |
| SMS | Medium | ⚠️ SMS can be intercepted |
| Public posting | None | ❌ No |

## Key Recovery

**There is no key recovery mechanism.** This is a fundamental property of zero-knowledge encryption.

If the URL fragment is lost:
- The server cannot recover the key (it was never transmitted)
- The encrypted files become permanently inaccessible
- There is no "forgot key" flow
- The encrypted data can only be deleted, not recovered

### User responsibilities

1. **Save the full share URL** (including the `#key=...` fragment) in a secure location
2. **Use a password manager** to store share URLs
3. **Do not rely on browser history** as the sole copy of the URL
4. **Understand the risk** before enabling E2E encryption

## Key Separation

E2E encryption keys are completely independent from:

- **Share passwords** — Share passwords control server-side access (authentication). The E2E key controls decryption (confidentiality). Both can be used together.
- **JWT tokens** — Authentication tokens have no relationship to encryption keys.
- **Server-side secrets** — The server's JWT secret, storage encryption keys, etc. are unrelated.

## Diagram

```
Share Creator                          Recipient
    │                                      │
    │ 1. Generate AES-256 key              │
    │ 2. Encrypt files in browser          │
    │ 3. Upload ciphertext to server       │
    │ 4. Receive share URL                 │
    │ 5. Append #key=... to URL            │
    │                                      │
    │──── Share full URL via secure ──────>│
    │     channel (e.g., Signal)           │
    │                                      │
    │                                      │ 6. Open URL
    │                                      │ 7. Extract key from #fragment
    │                                      │ 8. Download ciphertext
    │                                      │ 9. Decrypt in browser
    │                                      │ 10. View/save plaintext
```

## FAQ

**Q: Can the server admin recover encrypted files?**
A: No. The encryption key exists only in the URL fragment, which is never sent to the server. Without the key, AES-GCM ciphertext is computationally infeasible to decrypt.

**Q: What if I accidentally enable E2E on a share?**
A: E2E encryption is set at share creation time and cannot be disabled afterward. Files uploaded to an E2E share are always encrypted. You would need to create a new non-E2E share and re-upload the files.

**Q: Can I change the encryption key for an existing share?**
A: No. The key is generated once at share creation and embedded in the URL. Changing the key would make all existing files undecryptable.

**Q: Is the encryption key derived from the share password?**
A: No. The encryption key and share password are completely independent. The share password controls who can access the share page; the encryption key controls who can decrypt the files.
