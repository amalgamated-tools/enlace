import { describe, it, expect } from "vitest";
import {
  generateShareKey,
  exportKey,
  importKey,
  encryptFile,
  decryptFile,
} from "../e2e";

describe("E2E Encryption", () => {
  describe("key management", () => {
    it("generates a valid AES-GCM-256 key", async () => {
      const key = await generateShareKey();
      expect(key).toBeDefined();
      expect(key.algorithm).toEqual({ name: "AES-GCM", length: 256 });
      expect(key.extractable).toBe(true);
      expect(key.usages).toContain("encrypt");
      expect(key.usages).toContain("decrypt");
    });

    it("exports and imports a key roundtrip", async () => {
      const original = await generateShareKey();
      const encoded = await exportKey(original);

      expect(typeof encoded).toBe("string");
      expect(encoded.length).toBeGreaterThan(0);
      // base64url should not contain +, /, or =
      expect(encoded).not.toMatch(/[+/=]/);

      const imported = await importKey(encoded);
      expect(imported).toBeDefined();
      expect(imported.algorithm).toEqual({ name: "AES-GCM", length: 256 });
    });

    it("produces different encoded keys for different generated keys", async () => {
      const key1 = await generateShareKey();
      const key2 = await generateShareKey();
      const encoded1 = await exportKey(key1);
      const encoded2 = await exportKey(key2);
      expect(encoded1).not.toEqual(encoded2);
    });
  });

  describe("file encryption/decryption", () => {
    function createTestFile(
      content: string,
      name: string,
      type: string,
    ): File {
      return new File([content], name, { type });
    }

    it("encrypts and decrypts a file roundtrip", async () => {
      const key = await generateShareKey();
      const originalContent = "Hello, World! This is a test file.";
      const file = createTestFile(
        originalContent,
        "test.txt",
        "text/plain",
      );

      const encrypted = await encryptFile(key, file);
      expect(encrypted.blob).toBeInstanceOf(Blob);
      expect(encrypted.iv).toBeTruthy();
      expect(encrypted.encryptedMeta).toBeTruthy();

      // Encrypted blob should differ from original
      const encryptedData = await encrypted.blob.arrayBuffer();
      expect(encryptedData.byteLength).toBeGreaterThan(originalContent.length);

      const decrypted = await decryptFile(
        key,
        encrypted.iv,
        encrypted.encryptedMeta,
        encryptedData,
      );

      expect(decrypted.filename).toBe("test.txt");
      expect(decrypted.mimeType).toBe("text/plain");
      const decryptedText = await decrypted.blob.text();
      expect(decryptedText).toBe(originalContent);
    });

    it("preserves binary file content", async () => {
      const key = await generateShareKey();
      const bytes = new Uint8Array([0, 1, 2, 255, 128, 64, 32]);
      const file = new File([bytes], "binary.bin", {
        type: "application/octet-stream",
      });

      const encrypted = await encryptFile(key, file);
      const encryptedData = await encrypted.blob.arrayBuffer();

      const decrypted = await decryptFile(
        key,
        encrypted.iv,
        encrypted.encryptedMeta,
        encryptedData,
      );

      expect(decrypted.filename).toBe("binary.bin");
      expect(decrypted.mimeType).toBe("application/octet-stream");
      const result = new Uint8Array(await decrypted.blob.arrayBuffer());
      expect(Array.from(result)).toEqual(Array.from(bytes));
    });

    it("produces unique IVs for the same key and file", async () => {
      const key = await generateShareKey();
      const file = createTestFile("same content", "test.txt", "text/plain");

      const encrypted1 = await encryptFile(key, file);
      const encrypted2 = await encryptFile(key, file);

      expect(encrypted1.iv).not.toEqual(encrypted2.iv);
    });

    it("fails to decrypt with a wrong key", async () => {
      const key1 = await generateShareKey();
      const key2 = await generateShareKey();
      const file = createTestFile("secret data", "secret.txt", "text/plain");

      const encrypted = await encryptFile(key1, file);
      const encryptedData = await encrypted.blob.arrayBuffer();

      await expect(
        decryptFile(key2, encrypted.iv, encrypted.encryptedMeta, encryptedData),
      ).rejects.toThrow();
    });

    it("preserves original filename and MIME type through encryption", async () => {
      const key = await generateShareKey();
      const file = createTestFile(
        "<html></html>",
        "page.html",
        "text/html",
      );

      const encrypted = await encryptFile(key, file);
      const encryptedData = await encrypted.blob.arrayBuffer();

      const decrypted = await decryptFile(
        key,
        encrypted.iv,
        encrypted.encryptedMeta,
        encryptedData,
      );

      expect(decrypted.filename).toBe("page.html");
      expect(decrypted.mimeType).toBe("text/html");
    });

    it("handles empty files", async () => {
      const key = await generateShareKey();
      const file = createTestFile("", "empty.txt", "text/plain");

      const encrypted = await encryptFile(key, file);
      const encryptedData = await encrypted.blob.arrayBuffer();

      const decrypted = await decryptFile(
        key,
        encrypted.iv,
        encrypted.encryptedMeta,
        encryptedData,
      );

      expect(decrypted.filename).toBe("empty.txt");
      const content = await decrypted.blob.text();
      expect(content).toBe("");
    });
  });
});
