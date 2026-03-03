<script lang="ts">
  import { push, querystring } from "svelte-spa-router";
  import { Button, Input } from "../lib/components";
  import { auth, toast } from "../lib/stores";
  import { totpApi } from "../lib/api";

  let code = "";
  let recoveryCode = "";
  let loading = false;
  let useRecovery = false;
  let errors: Record<string, string> = {};

  // Get pending token from sessionStorage (primary) or query string (fallback)
  $: params = new URLSearchParams($querystring);
  $: pendingToken =
    sessionStorage.getItem("pending2FAToken") || params.get("token") || "";

  $: if (!pendingToken) {
    push("/login");
  }

  async function handleVerify(e: Event) {
    e.preventDefault();
    errors = {};

    if (!code.trim()) {
      errors = { code: "Code is required" };
      return;
    }

    loading = true;
    try {
      const response = await totpApi.verify(pendingToken, code.trim());
      if (!response.access_token || !response.refresh_token) {
        throw new Error("Verification failed: no tokens received");
      }
      auth.setTokens(response.access_token, response.refresh_token);
      sessionStorage.removeItem("pending2FAToken");
      toast.success("Logged in successfully");
      push("/");
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Verification failed";
      toast.error(message);
      code = "";
    } finally {
      loading = false;
    }
  }

  async function handleRecovery(e: Event) {
    e.preventDefault();
    errors = {};

    if (!recoveryCode.trim()) {
      errors = { recovery_code: "Recovery code is required" };
      return;
    }

    loading = true;
    try {
      const response = await totpApi.recovery(
        pendingToken,
        recoveryCode.trim(),
      );
      if (!response.access_token || !response.refresh_token) {
        throw new Error("Recovery failed: no tokens received");
      }
      auth.setTokens(response.access_token, response.refresh_token);
      sessionStorage.removeItem("pending2FAToken");
      toast.success("Logged in successfully");
      push("/");
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Recovery code verification failed";
      toast.error(message);
      recoveryCode = "";
    } finally {
      loading = false;
    }
  }
</script>

<div class="min-h-screen bg-slate-50 flex items-center justify-center px-4">
  <div class="w-full max-w-sm">
    <div class="text-center mb-8">
      <h1 class="text-2xl font-semibold text-slate-900">enlace</h1>
      <p class="text-sm text-slate-500 mt-1">Two-factor authentication</p>
    </div>

    <div class="bg-white rounded-xl border border-slate-200 shadow-sm p-8">
      {#if !useRecovery}
        <p class="text-sm text-slate-600 mb-6">
          Enter the 6-digit code from your authenticator app.
        </p>

        <form on:submit={handleVerify} class="space-y-5">
          <Input
            type="text"
            label="Authentication code"
            bind:value={code}
            placeholder="000000"
            error={errors.code}
            autocomplete="one-time-code"
            required
          />

          <Button type="submit" {loading} disabled={loading}>
            {loading ? "Verifying..." : "Verify"}
          </Button>
        </form>

        <button
          type="button"
          on:click={() => (useRecovery = true)}
          class="mt-4 w-full text-center text-sm text-slate-500 hover:text-slate-700 transition-colors"
        >
          Use a recovery code
        </button>
      {:else}
        <p class="text-sm text-slate-600 mb-6">
          Enter one of your recovery codes.
        </p>

        <form on:submit={handleRecovery} class="space-y-5">
          <Input
            type="text"
            label="Recovery code"
            bind:value={recoveryCode}
            placeholder="xxxx-xxxx"
            error={errors.recovery_code}
            autocomplete="off"
            required
          />

          <Button type="submit" {loading} disabled={loading}>
            {loading ? "Verifying..." : "Use recovery code"}
          </Button>
        </form>

        <button
          type="button"
          on:click={() => (useRecovery = false)}
          class="mt-4 w-full text-center text-sm text-slate-500 hover:text-slate-700 transition-colors"
        >
          Use authenticator code
        </button>
      {/if}
    </div>

    <p class="mt-6 text-center text-sm text-slate-500">
      <a href="#/login" class="text-slate-900 font-medium hover:underline"
        >Back to login</a
      >
    </p>
  </div>
</div>
