<script lang="ts">
  import Router, { location } from "svelte-spa-router";
  import routes from "./routes";
  import { Toast } from "./lib/components";
  import { auth, isAuthenticated } from "./lib/stores";
  import { push } from "svelte-spa-router";
  import { onMount } from "svelte";

  onMount(() => {
    auth.init();
  });

  function handleLogout() {
    auth.logout();
    push("/login");
  }

  // Pages that should NOT show the authenticated layout
  $: isPublicPage =
    $location === "/login" ||
    $location === "/register" ||
    $location === "/auth/callback" ||
    $location.startsWith("/s/");
  $: showLayout = $auth.initialized && $isAuthenticated && !isPublicPage;

  // Active nav link helper
  function isActive(path: string): boolean {
    if (path === "/") return $location === "/";
    return $location.startsWith(path);
  }
</script>

{#if !$auth.initialized}
  <div class="min-h-screen flex items-center justify-center">
    <div class="flex items-center gap-3 text-slate-400">
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
  <div class="min-h-screen bg-slate-50">
    <header class="bg-white border-b border-slate-200">
      <div
        class="max-w-6xl mx-auto px-6 h-14 flex items-center justify-between"
      >
        <div class="flex items-center gap-8">
          <a
            href="#/"
            class="text-base font-semibold text-slate-900 tracking-tight"
            >enlace</a
          >
          <nav class="flex items-center gap-1">
            <a
              href="#/"
              class="px-3 py-1.5 text-sm rounded-md transition-colors {isActive(
                '/',
              )
                ? 'text-slate-900 bg-slate-100 font-medium'
                : 'text-slate-500 hover:text-slate-700 hover:bg-slate-50'}"
            >
              Dashboard
            </a>
            <a
              href="#/shares"
              class="px-3 py-1.5 text-sm rounded-md transition-colors {isActive(
                '/shares',
              )
                ? 'text-slate-900 bg-slate-100 font-medium'
                : 'text-slate-500 hover:text-slate-700 hover:bg-slate-50'}"
            >
              Shares
            </a>
            <a
              href="#/settings"
              class="px-3 py-1.5 text-sm rounded-md transition-colors {isActive(
                '/settings',
              )
                ? 'text-slate-900 bg-slate-100 font-medium'
                : 'text-slate-500 hover:text-slate-700 hover:bg-slate-50'}"
            >
              Settings
            </a>
            {#if $auth.user?.is_admin}
              <a
                href="#/admin/users"
                class="px-3 py-1.5 text-sm rounded-md transition-colors {isActive(
                  '/admin',
                )
                  ? 'text-slate-900 bg-slate-100 font-medium'
                  : 'text-slate-500 hover:text-slate-700 hover:bg-slate-50'}"
              >
                Admin
              </a>
            {/if}
          </nav>
        </div>
        <div class="flex items-center gap-4">
          <span class="text-sm text-slate-500">{$auth.user?.display_name}</span>
          <button
            on:click={handleLogout}
            class="text-sm text-slate-500 hover:text-slate-700 transition-colors"
          >
            Sign out
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
