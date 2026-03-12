<script lang="ts">
  import { onMount } from "svelte";
  import { push, querystring } from "svelte-spa-router";
  import { Button, Input } from "../lib/components";
  import { auth, toast } from "../lib/stores";
  import { totpApi } from "../lib/api";

  let code = "";
  let loading = true;
  let confirming = false;
  let errors: Record<string, string> = {};
  let setupQR = "";
  let setupSecret = "";
  let recoveryCodes: string[] = [];
  let accessToken = "";
  let refreshToken = "";
  let pendingToken = "";

  onMount(async () => {
    const params = new URLSearchParams($querystring);
    pendingToken =
      sessionStorage.getItem("pending2FAToken") || params.get("token") || "";

    if (!pendingToken) {
      loading = false;
      push("/login");
      return;
    }

    try {
      const response = await totpApi.beginSetup(pendingToken);
      setupQR = response.qr_code;
      setupSecret = response.secret;
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to start 2FA setup";
      toast.error(message);
      push("/login");
    } finally {
      loading = false;
    }
  });

  async function handleSubmit(e: Event) {
    e.preventDefault();
    errors = {};

    if (!code.trim()) {
      errors = { code: "Code is required" };
      return;
    }

    confirming = true;
    try {
      const response = await totpApi.confirmSetup(code.trim(), pendingToken);
      if (!response.access_token || !response.refresh_token) {
        throw new Error(
          "Mandatory 2FA setup completed, but the server did not return session tokens",
        );
      }

      recoveryCodes = response.recovery_codes;
      accessToken = response.access_token;
      refreshToken = response.refresh_token;
      toast.success("Two-factor authentication enabled");
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to enable 2FA";
      toast.error(message);
      code = "";
    } finally {
      confirming = false;
    }
  }

  function handleContinue() {
    if (!accessToken || !refreshToken) {
      return;
    }
    auth.setTokens(accessToken, refreshToken);
    sessionStorage.removeItem("pending2FAToken");
    push("/");
  }
</script>

<div
  class="min-h-screen bg-surface-subtle flex items-center justify-center px-4"
>
  <div class="w-full max-w-md">
    <div class="text-center mb-8">
      <h1 class="text-2xl font-semibold text-text">enlace</h1>
      <p class="text-sm text-muted mt-1">Set up two-factor authentication</p>
    </div>

    <div class="bg-surface rounded-xl border border-border shadow-sm p-8">
      {#if loading}
        <p class="text-sm text-muted">Preparing your authenticator setup…</p>
      {:else if recoveryCodes.length > 0}
        <p class="text-sm text-muted mb-4">
          Save these recovery codes before continuing.
        </p>

        <div
          class="rounded-lg border border-border bg-surface-subtle p-4 mb-6 space-y-2"
        >
          {#each recoveryCodes as recoveryCode}
            <div class="font-mono text-sm text-text">{recoveryCode}</div>
          {/each}
        </div>

        <Button type="button" on:click={handleContinue}>
          Continue to enlace
        </Button>
      {:else}
        <p class="text-sm text-muted mb-6">
          Your administrator requires two-factor authentication before you can
          continue.
        </p>

        {#if setupQR}
          <div class="flex justify-center mb-4">
            <img
              src={`data:image/png;base64,${setupQR}`}
              alt="Two-factor QR code"
              class="w-48 h-48 rounded-lg border border-border bg-white p-3"
            />
          </div>
        {/if}

        <div class="rounded-lg border border-border bg-surface-subtle p-4 mb-6">
          <p class="text-xs uppercase tracking-wide text-subtle mb-2">Secret</p>
          <p class="font-mono text-sm text-text break-all">{setupSecret}</p>
        </div>

        <form on:submit={handleSubmit} class="space-y-5">
          <Input
            type="text"
            label="Authentication code"
            bind:value={code}
            placeholder="000000"
            error={errors.code}
            autocomplete="one-time-code"
            required
          />

          <Button type="submit" loading={confirming} disabled={confirming}>
            {confirming ? "Verifying..." : "Enable 2FA"}
          </Button>
        </form>
      {/if}
    </div>
  </div>
</div>
