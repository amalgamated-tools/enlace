import { readonly, writable } from "svelte/store";

export type ThemePreference = "system" | "light" | "dark";
export type Theme = "light" | "dark";

const STORAGE_KEY = "enlace.theme";

const _themePreference = writable<ThemePreference>("system");
const _themeEffective = writable<Theme>("light");

export const themePreference = readonly(_themePreference);
export const themeEffective = readonly(_themeEffective);

let initialized = false;
let currentPreference: ThemePreference = "system";
let mediaQuery: MediaQueryList | null = null;
let mediaListener: ((event: MediaQueryListEvent) => void) | null = null;

function getSystemTheme() {
  return window.matchMedia("(prefers-color-scheme: dark)").matches
    ? "dark"
    : "light";
}

function applyTheme(theme: Theme) {
  document.documentElement.dataset.theme = theme;
  _themeEffective.set(theme);
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
  _themePreference.set(currentPreference);
  const effective =
    currentPreference === "system" ? getSystemTheme() : currentPreference;
  if (document.documentElement.dataset.theme !== effective) {
    applyTheme(effective);
  } else {
    _themeEffective.set(effective);
  }

  mediaQuery = window.matchMedia("(prefers-color-scheme: dark)");
  mediaListener = (event) => {
    if (currentPreference !== "system") return;
    applyTheme(event.matches ? "dark" : "light");
  };
  mediaQuery.addEventListener("change", mediaListener);
}

export function setThemePreference(preference: ThemePreference) {
  currentPreference = preference;
  _themePreference.set(preference);
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

export function destroyTheme() {
  if (mediaQuery && mediaListener) {
    mediaQuery.removeEventListener("change", mediaListener);
  }
  mediaListener = null;
  mediaQuery = null;
  initialized = false;
  currentPreference = "system";
  _themePreference.set("system");
  _themeEffective.set("light");
}
