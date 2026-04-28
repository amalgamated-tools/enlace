<script lang="ts">
  import Router, { router } from "svelte-spa-router";
  import routes from "./routes";
  import { Toast } from "./lib/components";
  import {
    auth,
    destroyTheme,
    initTheme,
    isAuthenticated,
    loadFeatures,
  } from "./lib/stores";
  import { push } from "svelte-spa-router";
  import { onDestroy, onMount } from "svelte";
  import { writable } from "svelte/store";

  // Writable store for the current path — updated on hashchange so that
  // reactive `$:` declarations in this legacy-mode component can subscribe
  // to location changes via the `$` prefix (guaranteed in Svelte 5 legacy mode).
  const currentPath = writable(router.location);

  onMount(() => {
    initTheme();
    auth.init();
    loadFeatures();

    const onHashChange = () => currentPath.set(router.location);
    window.addEventListener("hashchange", onHashChange);
    return () => window.removeEventListener("hashchange", onHashChange);
  });

  onDestroy(() => {
    destroyTheme();
  });

  function handleLogout() {
    auth.logout();
    push("/login");
  }

  // Pages that should NOT show the authenticated layout
  $: isPublicPage =
    $currentPath === "/login" ||
    $currentPath === "/register" ||
    $currentPath === "/auth/callback" ||
    $currentPath === "/auth/2fa" ||
    $currentPath.startsWith("/s/");
  $: showLayout = $auth.initialized && $isAuthenticated && !isPublicPage;

  // Active nav link helpers
  $: dashboardActive = $currentPath === "/";
  $: sharesActive = $currentPath.startsWith("/shares");
  $: settingsActive = $currentPath.startsWith("/settings");
  $: adminActive = $currentPath.startsWith("/admin");
</script>

{#if !$auth.initialized}
  <div class="min-h-screen flex items-center justify-center">
    <div class="flex items-center gap-3 text-subtle">
      <svg
        class="animate-spin h-5 w-5"
        xmlns="http://www.w3.org/2000/svg"
        fill="none"
        viewBox="0 0 24 24"
      >
        <circle
          class="opacity-25"
          cx="12"
          cy="12"
          r="10"
          stroke="currentColor"
          stroke-width="4"
        ></circle>
        <path
          class="opacity-75"
          fill="currentColor"
          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
        ></path>
      </svg>
      <span class="text-sm">Loading...</span>
    </div>
  </div>
{:else if showLayout}
  <div class="min-h-screen bg-surface-subtle">
    <header class="bg-surface border-b border-border">
      <div
        class="max-w-6xl mx-auto px-6 h-14 flex items-center justify-between"
      >
        <div class="flex items-center gap-8">
          <a href="#/" class="text-base font-semibold text-text tracking-tight"
            >enlace</a
          >
          <nav class="flex items-center gap-1">
            <a
              href="#/"
              class="px-3 py-1.5 text-sm rounded-md transition-colors {dashboardActive
                ? 'text-text bg-surface-muted font-medium'
                : 'text-muted hover:text-text hover:bg-surface-subtle'}"
            >
              Dashboard
            </a>
            <a
              href="#/shares"
              class="px-3 py-1.5 text-sm rounded-md transition-colors {sharesActive
                ? 'text-text bg-surface-muted font-medium'
                : 'text-muted hover:text-text hover:bg-surface-subtle'}"
            >
              Shares
            </a>
            <a
              href="#/settings/profile"
              class="px-3 py-1.5 text-sm rounded-md transition-colors {settingsActive
                ? 'text-text bg-surface-muted font-medium'
                : 'text-muted hover:text-text hover:bg-surface-subtle'}"
            >
              Settings
            </a>
            {#if $auth.user?.is_admin}
              <a
                href="#/admin/users"
                class="px-3 py-1.5 text-sm rounded-md transition-colors {adminActive
                  ? 'text-text bg-surface-muted font-medium'
                  : 'text-muted hover:text-text hover:bg-surface-subtle'}"
              >
                Admin
              </a>
            {/if}
          </nav>
        </div>
        <div class="flex items-center gap-3">
          <span class="text-sm text-muted">{$auth.user?.display_name}</span>
          <button
            on:click={handleLogout}
            class="text-muted hover:text-text transition-colors"
            aria-label="Sign out"
            title="Sign out"
          >
            <svg
              class="w-4 h-4"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="1.6"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <path d="M9 21H5a2 2 0 01-2-2V5a2 2 0 012-2h4"></path>
              <polyline points="16 17 21 12 16 7"></polyline>
              <line x1="21" y1="12" x2="9" y2="12"></line>
            </svg>
          </button>
        </div>
      </div>
    </header>

    <main class="max-w-6xl mx-auto px-6 py-8">
      <Router {routes} />
    </main>
  </div>
{:else}
  <Router {routes} />
{/if}
<Toast />
