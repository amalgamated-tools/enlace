import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { get } from "svelte/store";
import {
  applyInitialTheme,
  destroyTheme,
  initTheme,
  setThemePreference,
  themeEffective,
} from "../theme";

type MatchMediaMock = MediaQueryList & {
  setMatches: (value: boolean) => void;
  addEventListener: ReturnType<typeof vi.fn>;
  removeEventListener: ReturnType<typeof vi.fn>;
};

function createMatchMedia(initialMatches: boolean): MatchMediaMock {
  let listener: ((event: MediaQueryListEvent) => void) | null = null;

  const mql = {
    matches: initialMatches,
    media: "(prefers-color-scheme: dark)",
    onchange: null as MediaQueryList["onchange"],
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(
      (_type: string, cb: (e: MediaQueryListEvent) => void) => {
        listener = cb;
      },
    ),
    removeEventListener: vi.fn(
      (_type: string, cb: (e: MediaQueryListEvent) => void) => {
        if (listener === cb) listener = null;
      },
    ),
    dispatchEvent: vi.fn(),
    setMatches(value: boolean) {
      mql.matches = value;
      if (listener) listener({ matches: value } as MediaQueryListEvent);
    },
  };

  return mql as unknown as MatchMediaMock;
}

describe("theme store", () => {
  beforeEach(() => {
    localStorage.clear();
    document.documentElement.dataset.theme = "";
  });

  afterEach(() => {
    destroyTheme();
    vi.restoreAllMocks();
  });

  it("applies system preference changes when set to system", () => {
    localStorage.setItem("enlace.theme", "system");
    const media = createMatchMedia(false);
    vi.stubGlobal("matchMedia", vi.fn().mockReturnValue(media));

    initTheme();
    expect(document.documentElement.dataset.theme).toBe("light");

    media.setMatches(true);
    expect(document.documentElement.dataset.theme).toBe("dark");
  });

  it("ignores system changes when preference is explicit", () => {
    localStorage.setItem("enlace.theme", "dark");
    const media = createMatchMedia(false);
    vi.stubGlobal("matchMedia", vi.fn().mockReturnValue(media));

    initTheme();
    expect(document.documentElement.dataset.theme).toBe("dark");

    media.setMatches(true);
    expect(document.documentElement.dataset.theme).toBe("dark");
  });

  it("cleans up matchMedia listener on destroy", () => {
    localStorage.setItem("enlace.theme", "system");
    const media = createMatchMedia(false);
    vi.stubGlobal("matchMedia", vi.fn().mockReturnValue(media));

    initTheme();
    const removeSpy = media.removeEventListener;

    destroyTheme();
    expect(removeSpy).toHaveBeenCalled();
  });

  it("applyInitialTheme sets the initial dataset without initializing the store", () => {
    localStorage.setItem("enlace.theme", "light");
    const media = createMatchMedia(true);
    vi.stubGlobal("matchMedia", vi.fn().mockReturnValue(media));

    applyInitialTheme();
    expect(document.documentElement.dataset.theme).toBe("light");
    expect(get(themeEffective)).toBe("light");

    setThemePreference("dark");
    expect(document.documentElement.dataset.theme).toBe("dark");
  });
});
