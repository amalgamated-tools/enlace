/**
 * End-to-end encryption module using AES-GCM-256 via Web Crypto API.
 *
 * Encryption key lifecycle:
 *   1. generateShareKey() → CryptoKey (one per share)
 *   2. exportKey(key) → base64url string (embedded in URL fragment)
 *   3. importKey(encoded) → CryptoKey (reconstructed from URL fragment)
 *
 * File encryption envelope (binary):
 *   [4 bytes: metadata length (big-endian uint32)]
 *   [metadata length bytes: encrypted metadata JSON (IV + ciphertext)]
 *   [remaining bytes: encrypted file content (12-byte IV prepended)]
 *
 * Metadata JSON (before encryption): { "name": "file.pdf", "type": "application/pdf" }
 */

const ALGORITHM = "AES-GCM";
const KEY_LENGTH = 256;
const IV_LENGTH = 12; // 96 bits recommended for AES-GCM

/** Generate a new AES-GCM-256 key for a share. */
export async function generateShareKey(): Promise<CryptoKey> {
  return crypto.subtle.generateKey(
    { name: ALGORITHM, length: KEY_LENGTH },
    true, // extractable so we can export it
    ["encrypt", "decrypt"],
  );
}

/** Export a CryptoKey to a base64url-encoded string for embedding in URL fragments. */
export async function exportKey(key: CryptoKey): Promise<string> {
  const raw = await crypto.subtle.exportKey("raw", key);
  return base64urlEncode(new Uint8Array(raw));
}

/** Import a CryptoKey from a base64url-encoded string (from URL fragment). */
export async function importKey(encoded: string): Promise<CryptoKey> {
  const raw = base64urlDecode(encoded);
  return crypto.subtle.importKey(
    "raw",
    raw,
    { name: ALGORITHM, length: KEY_LENGTH },
    false, // no need to re-export
    ["encrypt", "decrypt"],
  );
}

/** Metadata about the original file, encrypted alongside the content. */
interface FileMeta {
  name: string;
  type: string;
}

/** Result of encrypting a file. */
export interface EncryptedFile {
  /** The encrypted blob (envelope: metadata length + encrypted metadata + encrypted content). */
  blob: Blob;
  /** Base64-encoded IV used for the file content encryption. */
  iv: string;
  /** Base64-encoded encrypted metadata (original filename + MIME type). */
  encryptedMeta: string;
}

/** Result of decrypting a file. */
export interface DecryptedFile {
  blob: Blob;
  filename: string;
  mimeType: string;
}

/**
 * Encrypt a file for upload.
 * Returns the encrypted blob plus the IV and encrypted metadata as separate
 * strings for storage in the database alongside the file record.
 */
export async function encryptFile(
  key: CryptoKey,
  file: File,
): Promise<EncryptedFile> {
  // Read file content
  const plaintext = new Uint8Array(await file.arrayBuffer());

  // Generate unique IV for this file
  const iv = crypto.getRandomValues(new Uint8Array(IV_LENGTH));

  // Encrypt file content
  const ciphertext = new Uint8Array(
    await crypto.subtle.encrypt({ name: ALGORITHM, iv }, key, plaintext),
  );

  // Encrypt metadata (filename + MIME type)
  const meta: FileMeta = { name: file.name, type: file.type || "application/octet-stream" };
  const metaBytes = new TextEncoder().encode(JSON.stringify(meta));
  const metaIv = crypto.getRandomValues(new Uint8Array(IV_LENGTH));
  const encryptedMetaBytes = new Uint8Array(
    await crypto.subtle.encrypt({ name: ALGORITHM, iv: metaIv }, key, metaBytes),
  );

  // Build encrypted metadata string: base64(metaIv + encryptedMetaBytes)
  const metaPayload = new Uint8Array(metaIv.length + encryptedMetaBytes.length);
  metaPayload.set(metaIv, 0);
  metaPayload.set(encryptedMetaBytes, metaIv.length);
  const encryptedMeta = btoa(String.fromCharCode(...metaPayload));

  // Build content blob: IV prepended to ciphertext
  const contentPayload = new Uint8Array(iv.length + ciphertext.length);
  contentPayload.set(iv, 0);
  contentPayload.set(ciphertext, iv.length);

  return {
    blob: new Blob([contentPayload], { type: "application/octet-stream" }),
    iv: btoa(String.fromCharCode(...iv)),
    encryptedMeta,
  };
}

/**
 * Decrypt a downloaded file.
 * @param key — the share's CryptoKey
 * @param iv — base64-encoded IV (from file record)
 * @param encryptedMeta — base64-encoded encrypted metadata (from file record)
 * @param data — the raw encrypted file content (ArrayBuffer from fetch)
 */
export async function decryptFile(
  key: CryptoKey,
  iv: string,
  encryptedMeta: string,
  data: ArrayBuffer,
): Promise<DecryptedFile> {
  // Decode IV
  const ivBytes = Uint8Array.from(atob(iv), (c) => c.charCodeAt(0));

  // Decrypt file content (IV was prepended during encryption, but we also
  // store it separately; the data blob has IV + ciphertext)
  const contentBytes = new Uint8Array(data);
  const contentIv = contentBytes.slice(0, IV_LENGTH);
  const ciphertext = contentBytes.slice(IV_LENGTH);

  const plaintext = await crypto.subtle.decrypt(
    { name: ALGORITHM, iv: contentIv },
    key,
    ciphertext,
  );

  // Decrypt metadata
  const metaPayload = Uint8Array.from(atob(encryptedMeta), (c) => c.charCodeAt(0));
  const metaIv = metaPayload.slice(0, IV_LENGTH);
  const metaCiphertext = metaPayload.slice(IV_LENGTH);

  const metaPlaintext = await crypto.subtle.decrypt(
    { name: ALGORITHM, iv: metaIv },
    key,
    metaCiphertext,
  );

  const meta: FileMeta = JSON.parse(new TextDecoder().decode(metaPlaintext));

  return {
    blob: new Blob([plaintext], { type: meta.type }),
    filename: meta.name,
    mimeType: meta.type,
  };
}

// --- Base64url helpers (RFC 4648 §5) ---

function base64urlEncode(bytes: Uint8Array): string {
  const binStr = String.fromCharCode(...bytes);
  return btoa(binStr).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

function base64urlDecode(str: string): Uint8Array {
  const padded = str.replace(/-/g, "+").replace(/_/g, "/");
  const binStr = atob(padded);
  return Uint8Array.from(binStr, (c) => c.charCodeAt(0));
}
