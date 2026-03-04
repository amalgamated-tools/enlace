import { writable } from "svelte/store";

export type ThemePreference = "system" | "light" | "dark";
export type Theme = "light" | "dark";

const STORAGE_KEY = "enlace.theme";

export const themePreference = writable<ThemePreference>("system");
export const themeEffective = writable<Theme>("light");

let initialized = false;
let currentPreference: ThemePreference = "system";
let mediaQuery: MediaQueryList | null = null;

function getSystemTheme() {
  return window.matchMedia("(prefers-color-scheme: dark)").matches
    ? "dark"
    : "light";
}

function applyTheme(theme: Theme) {
  document.documentElement.dataset.theme = theme;
  themeEffective.set(theme);
}

function readStoredPreference(): ThemePreference {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored === "light" || stored === "dark" || stored === "system") {
      return stored;
    }
  } catch {
    // Ignore storage errors and fall back to system.
  }
  return "system";
}

function persistPreference(preference: ThemePreference) {
  try {
    localStorage.setItem(STORAGE_KEY, preference);
  } catch {
    // Ignore storage errors (private mode, disabled storage).
  }
}

export function applyInitialTheme() {
  const preference = readStoredPreference();
  const effective = preference === "system" ? getSystemTheme() : preference;
  document.documentElement.dataset.theme = effective;
}

export function initTheme() {
  if (initialized) return;
  initialized = true;

  currentPreference = readStoredPreference();
  themePreference.set(currentPreference);
  applyTheme(
    currentPreference === "system" ? getSystemTheme() : currentPreference,
  );

  mediaQuery = window.matchMedia("(prefers-color-scheme: dark)");
  mediaQuery.addEventListener("change", (event) => {
    if (currentPreference !== "system") return;
    applyTheme(event.matches ? "dark" : "light");
  });
}

export function setThemePreference(preference: ThemePreference) {
  currentPreference = preference;
  themePreference.set(preference);
  persistPreference(preference);
  applyTheme(preference === "system" ? getSystemTheme() : preference);
}

export function cycleThemePreference() {
  const nextPreference: ThemePreference =
    currentPreference === "system"
      ? "light"
      : currentPreference === "light"
        ? "dark"
        : "system";
  setThemePreference(nextPreference);
}
