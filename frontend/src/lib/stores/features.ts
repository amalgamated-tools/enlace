import { writable } from "svelte/store";

export const emailConfigured = writable(false);
export const e2eEncryptionEnabled = writable(false);

export async function loadFeatures() {
  try {
    const res = await fetch("/health");
    const json = await res.json();
    if (json.success && json.data) {
      emailConfigured.set(!!json.data.email_configured);
      e2eEncryptionEnabled.set(!!json.data.e2e_encryption_enabled);
    }
  } catch {
    // Health endpoint unavailable; default to disabled
  }
}
