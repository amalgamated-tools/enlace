import { writable } from "svelte/store";

export const emailConfigured = writable(false);

export async function loadFeatures() {
  try {
    const res = await fetch("/health");
    const json = await res.json();
    if (json.success && json.data) {
      emailConfigured.set(!!json.data.email_configured);
    }
  } catch {
    // Health endpoint unavailable; default to disabled
  }
}
