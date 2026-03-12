<script lang="ts">
  import { onMount } from "svelte";
  import { push, querystring } from "svelte-spa-router";
  import { Button, Input } from "../lib/components";
  import { auth, isAuthenticated, toast } from "../lib/stores";
  import { getOIDCConfig, getOIDCLoginURL } from "../lib/api";

  let email = "";
  let password = "";
  let loading = false;
  let errors: Record<string, string> = {};
  let oidcEnabled = false;

  $: if ($isAuthenticated) {
    push("/");
  }

  onMount(async () => {
    // Check for error query param (e.g., from failed OIDC login)
    const params = new URLSearchParams($querystring);
    const error = params.get("error");
    if (error) {
      toast.error(decodeURIComponent(error));
    }

    // Check if OIDC is enabled
    try {
      const config = await getOIDCConfig();
      oidcEnabled = config.enabled;
    } catch {
      // OIDC not available, keep disabled
    }
  });

  function handleOIDCLogin(): void {
    window.location.href = getOIDCLoginURL();
  }

  async function handleSubmit(e: Event) {
    e.preventDefault();
    errors = {};

    if (!email.trim()) {
      errors = { ...errors, email: "Email is required" };
    }
    if (!password) {
      errors = { ...errors, password: "Password is required" };
    }

    if (Object.keys(errors).length > 0) {
      return;
    }

    loading = true;
    try {
      const result = await auth.login(email, password);

      if (result.requires2FA && result.pendingToken) {
        // Store the pending 2FA token in sessionStorage instead of passing it via URL
        try {
          sessionStorage.setItem("pending2FAToken", result.pendingToken);
        } catch {
          // If storage is unavailable, proceed without persisting the token here.
        }
        push("/auth/2fa");
        return;
      }

      if (result.requires2FASetup) {
        if (result.pendingToken) {
          try {
            sessionStorage.setItem("pending2FAToken", result.pendingToken);
          } catch {
            // Ignore storage failures and rely on in-memory navigation only.
          }
          push("/auth/2fa/setup");
          return;
        }
        toast.info(
          "Your administrator requires two-factor authentication. Please set it up now.",
        );
        push("/settings?setup2fa=true");
        return;
      }

      toast.success("Logged in successfully");
      push("/");
    } catch (err) {
      const message = err instanceof Error ? err.message : "Login failed";
      toast.error(message);
    } finally {
      loading = false;
    }
  }
</script>

<div
  class="min-h-screen bg-surface-subtle flex items-center justify-center px-4"
>
  <div class="w-full max-w-sm">
    <div class="text-center mb-8">
      <h1 class="text-2xl font-semibold text-text">enlace</h1>
      <p class="text-sm text-muted mt-1">Sign in to your account</p>
    </div>

    <div class="bg-surface rounded-xl border border-border shadow-sm p-8">
      {#if oidcEnabled}
        <button
          type="button"
          on:click={handleOIDCLogin}
          class="w-full flex items-center justify-center gap-2 px-4 py-2.5 border border-border rounded-lg text-sm font-medium text-text hover:bg-surface-subtle transition-colors mb-6"
        >
          <svg
            class="w-5 h-5"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
          >
            <path
              d="M15 3h4a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-4M10 17l5-5-5-5M13.8 12H3"
            />
          </svg>
          Sign in with SSO
        </button>

        <div class="relative mb-6">
          <div class="absolute inset-0 flex items-center">
            <div class="w-full border-t border-border"></div>
          </div>
          <div class="relative flex justify-center text-xs uppercase">
            <span class="bg-surface px-2 text-subtle"
              >Or continue with email</span
            >
          </div>
        </div>
      {/if}

      <form on:submit={handleSubmit} class="space-y-5">
        <Input
          type="email"
          label="Email"
          bind:value={email}
          placeholder="you@example.com"
          error={errors.email}
          autocomplete="email"
          required
        />

        <Input
          type="password"
          label="Password"
          bind:value={password}
          placeholder="Enter your password"
          error={errors.password}
          autocomplete="current-password"
          required
        />

        <Button type="submit" {loading} disabled={loading}>
          {loading ? "Signing in..." : "Sign in"}
        </Button>
      </form>
    </div>

    <p class="mt-6 text-center text-sm text-muted">
      Don't have an account?
      <a href="#/register" class="text-text font-medium hover:underline"
        >Register</a
      >
    </p>
  </div>
</div>
