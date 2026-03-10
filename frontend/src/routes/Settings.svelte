<script lang="ts">
  import { onMount } from "svelte";
  import { push, querystring } from "svelte-spa-router";
  import { Button, Input, Modal } from "../lib/components";
  import {
    auth,
    isAuthenticated,
    setThemePreference,
    themePreference,
    toast,
  } from "../lib/stores";
  import {
    api,
    apiKeysApi,
    ApiError,
    ALL_SCOPES,
    getOIDCConfig,
    getOIDCLinkURL,
    totpApi,
  } from "../lib/api";
  import type { ApiKey, CreateApiKeyResponse } from "../lib/api";
  import type { TOTPStatus } from "../lib/api/totp";

  let displayName = "";
  let currentPassword = "";
  let newPassword = "";
  let confirmPassword = "";

  let savingProfile = false;
  let savingPassword = false;
  let profileErrors: Record<string, string> = {};
  let passwordErrors: Record<string, string> = {};

  let oidcEnabled = false;
  let unlinkingOIDC = false;

  // 2FA state
  let totpStatus: TOTPStatus | null = null;
  let showSetupModal = false;
  let setupStep: "qr" | "verify" | "recovery" = "qr";
  let setupQR = "";
  let setupSecret = "";
  let setupCode = "";
  let setupRecoveryCodes: string[] = [];
  let settingUp2FA = false;
  let confirming2FA = false;
  let disabling2FA = false;
  let disablePassword = "";
  let regeneratingCodes = false;
  let regeneratePassword = "";
  let showRegenerateModal = false;
  let setupErrors: Record<string, string> = {};
  let codesCopied = false;

  // API key state
  const allScopes = ALL_SCOPES;
  let apiKeys: ApiKey[] = [];
  let loadingApiKeys = true;
  let createApiKeyModal = false;
  let creatingApiKey = false;
  let newApiKeyName = "";
  let newApiKeyScopes: string[] = [];
  let apiKeyCreateErrors: Record<string, string> = {};
  let apiKeyModal = false;
  let createdApiKey = "";
  let apiKeyCopied = false;
  let revokeApiKeyModal = false;
  let revokingApiKey = false;
  let apiKeyToRevoke: ApiKey | null = null;

  async function copyRecoveryCodes() {
    try {
      await navigator.clipboard.writeText(setupRecoveryCodes.join("\n"));
      toast.success("Recovery codes copied to clipboard");
      codesCopied = true;
      setTimeout(() => (codesCopied = false), 2000);
    } catch (error) {
      console.error("Failed to copy recovery codes", error);
      toast.error("Failed to copy recovery codes. Please copy them manually.");
    }
  }

  function downloadRecoveryCodes() {
    const content = setupRecoveryCodes.join("\n");
    const blob = new Blob([content], { type: "text/plain" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "enlace-2fa.txt";
    a.click();
    setTimeout(() => URL.revokeObjectURL(url), 100);
  }

  // API key functions
  async function loadApiKeys() {
    loadingApiKeys = true;
    try {
      apiKeys = await apiKeysApi.list();
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to load API keys";
      toast.error(message);
    } finally {
      loadingApiKeys = false;
    }
  }

  function openCreateApiKeyModal() {
    newApiKeyName = "";
    newApiKeyScopes = [];
    apiKeyCreateErrors = {};
    createApiKeyModal = true;
  }

  function toggleApiKeyScope(scope: string) {
    if (newApiKeyScopes.includes(scope)) {
      newApiKeyScopes = newApiKeyScopes.filter((s) => s !== scope);
    } else {
      newApiKeyScopes = [...newApiKeyScopes, scope];
    }
  }

  async function handleCreateApiKey(e: Event) {
    e.preventDefault();
    apiKeyCreateErrors = {};

    if (!newApiKeyName.trim()) {
      apiKeyCreateErrors = {
        ...apiKeyCreateErrors,
        name: "Name is required",
      };
    }
    if (newApiKeyScopes.length === 0) {
      apiKeyCreateErrors = {
        ...apiKeyCreateErrors,
        scopes: "At least one scope is required",
      };
    }

    if (Object.keys(apiKeyCreateErrors).length > 0) {
      return;
    }

    creatingApiKey = true;
    try {
      const result: CreateApiKeyResponse = await apiKeysApi.create({
        name: newApiKeyName.trim(),
        scopes: newApiKeyScopes,
      });
      const { key, ...apiKey } = result;
      createdApiKey = key;
      apiKeyCopied = false;
      apiKeys = [...apiKeys, apiKey];
      createApiKeyModal = false;
      apiKeyModal = true;
      toast.success("API key created");
    } catch (err) {
      if (err instanceof ApiError && err.fields) {
        apiKeyCreateErrors = err.fields;
      } else {
        const message =
          err instanceof Error ? err.message : "Failed to create API key";
        toast.error(message);
      }
    } finally {
      creatingApiKey = false;
    }
  }

  async function copyApiKey() {
    try {
      await navigator.clipboard.writeText(createdApiKey);
      apiKeyCopied = true;
      toast.success("API key copied to clipboard");
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Failed to copy API key to clipboard";
      toast.error(message);
    }
  }

  function confirmRevokeApiKey(apiKey: ApiKey) {
    apiKeyToRevoke = apiKey;
    revokeApiKeyModal = true;
  }

  async function handleRevokeApiKey() {
    if (!apiKeyToRevoke) return;

    const revokeId = apiKeyToRevoke.id;
    revokingApiKey = true;
    try {
      await apiKeysApi.revoke(revokeId);
      apiKeys = apiKeys.map((k) =>
        k.id === revokeId ? { ...k, revoked_at: new Date().toISOString() } : k,
      );
      revokeApiKeyModal = false;
      apiKeyToRevoke = null;
      toast.success("API key revoked");
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to revoke API key";
      toast.error(message);
    } finally {
      revokingApiKey = false;
    }
  }

  function formatApiKeyDate(dateStr: string): string {
    return new Date(dateStr).toLocaleDateString();
  }

  onMount(async () => {
    // Check for oidc=linked query param (from successful OIDC linking)
    const params = new URLSearchParams($querystring);
    if (params.get("oidc") === "linked") {
      toast.success("SSO account linked successfully");
      // Remove query param from URL
      push("/settings");
    }

    // Check if admin requires 2FA setup
    if (params.get("setup2fa") === "true") {
      push("/settings");
      // Will trigger setup after status loads
    }

    // Fetch OIDC config
    try {
      const config = await getOIDCConfig();
      oidcEnabled = config.enabled;
    } catch {
      // OIDC not available
      oidcEnabled = false;
    }

    // Fetch 2FA status
    try {
      totpStatus = await totpApi.getStatus();
      // Auto-open setup if redirected from login with setup2fa flag
      if (
        params.get("setup2fa") === "true" &&
        !totpStatus.enabled &&
        !$auth.user?.oidc_linked
      ) {
        handleBeginSetup();
      }
    } catch {
      // 2FA status not available
    }

    // Load API keys
    loadApiKeys();
  });

  function handleLinkOIDC() {
    window.location.href = getOIDCLinkURL();
  }

  async function handleUnlinkOIDC() {
    unlinkingOIDC = true;
    try {
      await api.delete("/me/oidc");
      toast.success("OIDC account unlinked");
      auth.init(); // Refresh user data
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to unlink OIDC";
      toast.error(message);
    } finally {
      unlinkingOIDC = false;
    }
  }

  $: if ($auth.initialized && !$isAuthenticated) {
    push("/login");
  }

  $: if ($auth.user) {
    displayName = $auth.user.display_name;
  }

  async function handleUpdateProfile(e: Event) {
    e.preventDefault();
    profileErrors = {};

    if (!displayName.trim()) {
      profileErrors = {
        ...profileErrors,
        displayName: "Display name is required",
      };
      return;
    }

    savingProfile = true;
    try {
      const updated = await api.patch<{
        display_name: string;
        email: string;
        id: string;
        is_admin: boolean;
      }>("/me", {
        display_name: displayName.trim(),
      });
      auth.setUser({ ...updated, display_name: updated.display_name });
      toast.success("Profile updated");
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to update profile";
      toast.error(message);
    } finally {
      savingProfile = false;
    }
  }

  async function handleChangePassword(e: Event) {
    e.preventDefault();
    passwordErrors = {};

    if (!currentPassword) {
      passwordErrors = {
        ...passwordErrors,
        currentPassword: "Current password is required",
      };
    }
    if (!newPassword) {
      passwordErrors = {
        ...passwordErrors,
        newPassword: "New password is required",
      };
    } else if (newPassword.length < 8) {
      passwordErrors = {
        ...passwordErrors,
        newPassword: "Password must be at least 8 characters",
      };
    }
    if (newPassword !== confirmPassword) {
      passwordErrors = {
        ...passwordErrors,
        confirmPassword: "Passwords do not match",
      };
    }

    if (Object.keys(passwordErrors).length > 0) {
      return;
    }

    savingPassword = true;
    try {
      await api.put<void>("/me/password", {
        current_password: currentPassword,
        new_password: newPassword,
      });
      currentPassword = "";
      newPassword = "";
      confirmPassword = "";
      toast.success("Password changed");
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to change password";
      toast.error(message);
    } finally {
      savingPassword = false;
    }
  }

  async function handleBeginSetup() {
    settingUp2FA = true;
    setupErrors = {};
    try {
      const response = await totpApi.beginSetup();
      setupQR = response.qr_code;
      setupSecret = response.secret;
      setupStep = "qr";
      showSetupModal = true;
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to start 2FA setup";
      toast.error(message);
    } finally {
      settingUp2FA = false;
    }
  }

  async function handleConfirmSetup() {
    setupErrors = {};
    if (!setupCode.trim()) {
      setupErrors = { code: "Code is required" };
      return;
    }

    confirming2FA = true;
    try {
      const response = await totpApi.confirmSetup(setupCode.trim());
      setupRecoveryCodes = response.recovery_codes;
      setupStep = "recovery";
      totpStatus = {
        enabled: true,
        require_2fa: totpStatus?.require_2fa || false,
      };
      toast.success("Two-factor authentication enabled");
    } catch (err) {
      const message = err instanceof Error ? err.message : "Invalid code";
      toast.error(message);
      setupCode = "";
    } finally {
      confirming2FA = false;
    }
  }

  function handleCloseSetup() {
    showSetupModal = false;
    setupQR = "";
    setupSecret = "";
    setupCode = "";
    setupRecoveryCodes = [];
    setupStep = "qr";
    setupErrors = {};
  }

  async function handleDisable2FA() {
    setupErrors = {};
    if (!disablePassword) {
      setupErrors = { disablePassword: "Password is required" };
      return;
    }

    disabling2FA = true;
    try {
      await totpApi.disable(disablePassword);
      totpStatus = {
        enabled: false,
        require_2fa: totpStatus?.require_2fa || false,
      };
      disablePassword = "";
      toast.success("Two-factor authentication disabled");
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to disable 2FA";
      toast.error(message);
    } finally {
      disabling2FA = false;
    }
  }

  async function handleRegenerateCodes() {
    setupErrors = {};
    if (!regeneratePassword) {
      setupErrors = { regeneratePassword: "Password is required" };
      return;
    }

    regeneratingCodes = true;
    try {
      const response =
        await totpApi.regenerateRecoveryCodes(regeneratePassword);
      setupRecoveryCodes = response.recovery_codes;
      regeneratePassword = "";
      showRegenerateModal = false;
      setupStep = "recovery";
      showSetupModal = true;
      toast.success("Recovery codes regenerated");
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to regenerate codes";
      toast.error(message);
    } finally {
      regeneratingCodes = false;
    }
  }
</script>

<div>
  <h2 class="text-lg font-semibold text-text mb-6">Settings</h2>

  <div class="bg-surface rounded-xl border border-border mb-6">
    <div class="px-6 py-4 border-b border-border">
      <h3 class="text-sm font-semibold text-text">Appearance</h3>
    </div>
    <div class="p-6">
      <p class="text-sm text-muted mb-4">Choose your preferred color theme.</p>
      <div class="flex gap-2">
        <button
          on:click={() => setThemePreference("system")}
          class="flex items-center gap-2 px-3 py-2 text-sm rounded-lg border transition-colors {$themePreference ===
          'system'
            ? 'border-accent bg-accent/10 text-text font-medium'
            : 'border-border text-muted hover:text-text hover:border-border-strong'}"
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
            <rect x="3" y="4" width="18" height="12" rx="2"></rect>
            <path d="M8 20h8"></path>
            <path d="M12 16v4"></path>
          </svg>
          System
        </button>
        <button
          on:click={() => setThemePreference("light")}
          class="flex items-center gap-2 px-3 py-2 text-sm rounded-lg border transition-colors {$themePreference ===
          'light'
            ? 'border-accent bg-accent/10 text-text font-medium'
            : 'border-border text-muted hover:text-text hover:border-border-strong'}"
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
            <circle cx="12" cy="12" r="4"></circle>
            <path d="M12 2v2"></path>
            <path d="M12 20v2"></path>
            <path d="M4.93 4.93l1.41 1.41"></path>
            <path d="M17.66 17.66l1.41 1.41"></path>
            <path d="M2 12h2"></path>
            <path d="M20 12h2"></path>
            <path d="M6.34 17.66l-1.41 1.41"></path>
            <path d="M19.07 4.93l-1.41 1.41"></path>
          </svg>
          Light
        </button>
        <button
          on:click={() => setThemePreference("dark")}
          class="flex items-center gap-2 px-3 py-2 text-sm rounded-lg border transition-colors {$themePreference ===
          'dark'
            ? 'border-accent bg-accent/10 text-text font-medium'
            : 'border-border text-muted hover:text-text hover:border-border-strong'}"
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
            <path d="M21 12.79A9 9 0 1111.21 3a7 7 0 009.79 9.79z"></path>
          </svg>
          Dark
        </button>
      </div>
    </div>
  </div>

  <div class="bg-surface rounded-xl border border-border mb-6">
    <div class="px-6 py-4 border-b border-border">
      <h3 class="text-sm font-semibold text-text">Profile</h3>
    </div>
    <div class="p-6">
      <form on:submit={handleUpdateProfile} class="space-y-5">
        <Input
          type="email"
          label="Email"
          value={$auth.user?.email || ""}
          autocomplete="email"
          disabled
        />
        <Input
          label="Display Name"
          bind:value={displayName}
          error={profileErrors.displayName}
          autocomplete="name"
          required
        />
        <Button type="submit" loading={savingProfile}>
          {savingProfile ? "Saving..." : "Update Profile"}
        </Button>
      </form>
    </div>
  </div>

  <div class="bg-surface rounded-xl border border-border">
    <div class="px-6 py-4 border-b border-border">
      <h3 class="text-sm font-semibold text-text">Change Password</h3>
    </div>
    <div class="p-6">
      <form on:submit={handleChangePassword} class="space-y-5">
        <Input
          type="password"
          label="Current Password"
          bind:value={currentPassword}
          error={passwordErrors.currentPassword}
          autocomplete="current-password"
          required
        />
        <Input
          type="password"
          label="New Password"
          bind:value={newPassword}
          placeholder="At least 8 characters"
          error={passwordErrors.newPassword}
          autocomplete="new-password"
          required
        />
        <Input
          type="password"
          label="Confirm New Password"
          bind:value={confirmPassword}
          error={passwordErrors.confirmPassword}
          autocomplete="new-password"
          required
        />
        <Button type="submit" loading={savingPassword}>
          {savingPassword ? "Changing..." : "Change Password"}
        </Button>
      </form>
    </div>
  </div>

  {#if totpStatus !== null && !$auth.user?.oidc_linked}
    <div class="bg-surface rounded-xl border border-border mt-6">
      <div class="px-6 py-4 border-b border-border">
        <h3 class="text-sm font-semibold text-text">
          Two-Factor Authentication
        </h3>
      </div>
      <div class="p-6">
        {#if !totpStatus.enabled}
          <p class="text-sm text-muted mb-4">
            Add an extra layer of security to your account by enabling
            two-factor authentication with an authenticator app.
          </p>
          {#if totpStatus.require_2fa}
            <p class="text-sm text-amber-600 mb-4">
              Your administrator requires two-factor authentication. Please set
              it up to continue using your account.
            </p>
          {/if}
          <Button
            variant="secondary"
            on:click={handleBeginSetup}
            loading={settingUp2FA}
          >
            {settingUp2FA
              ? "Setting up..."
              : "Enable Two-Factor Authentication"}
          </Button>
        {:else}
          <div class="flex items-center gap-2 mb-4">
            <span
              class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800"
            >
              Enabled
            </span>
            <span class="text-sm text-muted"
              >Two-factor authentication is active.</span
            >
          </div>
          <div class="flex flex-col sm:flex-row gap-3">
            <Button
              variant="secondary"
              on:click={() => (showRegenerateModal = true)}
            >
              Regenerate Recovery Codes
            </Button>
          </div>

          <div class="mt-6 pt-6 border-t border-border">
            <p class="text-sm text-muted mb-3">
              Enter your password to disable two-factor authentication.
            </p>
            <form
              on:submit|preventDefault={handleDisable2FA}
              class="flex flex-col sm:flex-row gap-3 items-start"
            >
              <div class="w-full sm:w-64">
                <Input
                  type="password"
                  bind:value={disablePassword}
                  placeholder="Your password"
                  error={setupErrors.disablePassword}
                  autocomplete="current-password"
                />
              </div>
              <Button type="submit" variant="secondary" loading={disabling2FA}>
                {disabling2FA ? "Disabling..." : "Disable 2FA"}
              </Button>
            </form>
          </div>
        {/if}
      </div>
    </div>
  {/if}

  {#if showSetupModal}
    <div
      class="fixed inset-0 bg-overlay/50 flex items-center justify-center z-50 px-4"
    >
      <div
        class="bg-surface rounded-xl border border-border shadow-lg w-full max-w-md p-6"
      >
        {#if setupStep === "qr"}
          <h3 class="text-base font-semibold text-text mb-4">
            Set up authenticator
          </h3>
          <p class="text-sm text-muted mb-4">
            Scan this QR code with your authenticator app (Google Authenticator,
            Authy, 1Password, etc.)
          </p>
          <div class="flex justify-center mb-4">
            <img
              src="data:image/png;base64,{setupQR}"
              alt="TOTP QR Code"
              class="w-48 h-48"
            />
          </div>
          <details class="mb-4">
            <summary class="text-xs text-muted cursor-pointer hover:text-text">
              Can't scan? Enter this key manually
            </summary>
            <code
              class="block mt-2 text-xs bg-surface-subtle p-2 rounded break-all select-all"
              >{setupSecret}</code
            >
          </details>
          <div class="flex justify-end gap-3">
            <Button variant="secondary" on:click={handleCloseSetup}
              >Cancel</Button
            >
            <Button on:click={() => (setupStep = "verify")}>Next</Button>
          </div>
        {:else if setupStep === "verify"}
          <h3 class="text-base font-semibold text-text mb-4">Verify setup</h3>
          <p class="text-sm text-muted mb-4">
            Enter the 6-digit code from your authenticator app to confirm setup.
          </p>
          <form on:submit|preventDefault={handleConfirmSetup} class="space-y-4">
            <Input
              type="text"
              label="Authentication code"
              bind:value={setupCode}
              placeholder="000000"
              error={setupErrors.code}
              autocomplete="one-time-code"
              required
            />
            <div class="flex justify-end gap-3">
              <Button variant="secondary" on:click={() => (setupStep = "qr")}
                >Back</Button
              >
              <Button type="submit" loading={confirming2FA}>
                {confirming2FA ? "Verifying..." : "Verify & Enable"}
              </Button>
            </div>
          </form>
        {:else if setupStep === "recovery"}
          <h3 class="text-base font-semibold text-text mb-4">
            Save your recovery codes
          </h3>
          <p class="text-sm text-muted mb-4">
            Store these codes in a safe place. Each code can only be used once.
            You'll need them if you lose access to your authenticator app.
          </p>
          <div class="bg-surface-subtle rounded-lg p-4 mb-4">
            <div class="grid grid-cols-2 gap-2">
              {#each setupRecoveryCodes as code}
                <code class="text-sm font-mono text-text">{code}</code>
              {/each}
            </div>
          </div>
          <div class="flex justify-end gap-3">
            <Button variant="secondary" size="sm" on:click={copyRecoveryCodes}>
              {codesCopied ? "Copied!" : "Copy Codes"}
            </Button>
            <Button
              variant="secondary"
              size="sm"
              on:click={downloadRecoveryCodes}
            >
              Download
            </Button>
            <Button on:click={handleCloseSetup}>Done</Button>
          </div>
        {/if}
      </div>
    </div>
  {/if}

  {#if showRegenerateModal}
    <div
      class="fixed inset-0 bg-overlay/50 flex items-center justify-center z-50 px-4"
    >
      <div
        class="bg-surface rounded-xl border border-border shadow-lg w-full max-w-md p-6"
      >
        <h3 class="text-base font-semibold text-text mb-4">
          Regenerate recovery codes
        </h3>
        <p class="text-sm text-muted mb-4">
          This will invalidate your existing recovery codes and generate new
          ones. Enter your password to confirm.
        </p>
        <form
          on:submit|preventDefault={handleRegenerateCodes}
          class="space-y-4"
        >
          <Input
            type="password"
            label="Password"
            bind:value={regeneratePassword}
            placeholder="Your password"
            error={setupErrors.regeneratePassword}
            autocomplete="current-password"
            required
          />
          <div class="flex justify-end gap-3">
            <Button
              variant="secondary"
              on:click={() => {
                showRegenerateModal = false;
                regeneratePassword = "";
              }}
            >
              Cancel
            </Button>
            <Button type="submit" loading={regeneratingCodes}>
              {regeneratingCodes ? "Regenerating..." : "Regenerate Codes"}
            </Button>
          </div>
        </form>
      </div>
    </div>
  {/if}

  {#if oidcEnabled}
    <div class="bg-surface rounded-xl border border-border mt-6">
      <div class="px-6 py-4 border-b border-border">
        <h3 class="text-sm font-semibold text-text">Single Sign-On</h3>
      </div>
      <div class="p-6">
        {#if $auth.user?.oidc_linked}
          <p class="text-sm text-muted mb-4">
            Your account is linked to an external identity provider.
          </p>
          {#if $auth.user?.has_password}
            <Button
              variant="secondary"
              on:click={handleUnlinkOIDC}
              loading={unlinkingOIDC}
            >
              {unlinkingOIDC ? "Unlinking..." : "Unlink SSO Account"}
            </Button>
          {:else}
            <p class="text-xs text-subtle">
              Set a password before unlinking SSO to avoid being locked out.
            </p>
          {/if}
        {:else}
          <p class="text-sm text-muted mb-4">
            Link your account to an external identity provider for easier login.
          </p>
          <Button variant="secondary" on:click={handleLinkOIDC}>
            Link SSO Account
          </Button>
        {/if}
      </div>
    </div>
  {/if}

  <div class="bg-surface rounded-xl border border-border mt-6">
    <div
      class="px-6 py-4 border-b border-border flex items-center justify-between"
    >
      <h3 class="text-sm font-semibold text-text">API Keys</h3>
      <Button size="sm" on:click={openCreateApiKeyModal}>Create API Key</Button>
    </div>
    <div class="p-6">
      {#if loadingApiKeys}
        <p class="text-sm text-subtle text-center py-4">Loading...</p>
      {:else}
        <div class="overflow-hidden">
          <table class="min-w-full divide-y divide-border">
            <thead>
              <tr class="bg-surface-subtle">
                <th
                  class="px-4 py-2 text-left text-xs font-medium text-subtle uppercase tracking-wider"
                  >Name</th
                >
                <th
                  class="px-4 py-2 text-left text-xs font-medium text-subtle uppercase tracking-wider"
                  >Key Prefix</th
                >
                <th
                  class="px-4 py-2 text-left text-xs font-medium text-subtle uppercase tracking-wider"
                  >Scopes</th
                >
                <th
                  class="px-4 py-2 text-left text-xs font-medium text-subtle uppercase tracking-wider"
                  >Last Used</th
                >
                <th
                  class="px-4 py-2 text-left text-xs font-medium text-subtle uppercase tracking-wider"
                  >Created</th
                >
                <th
                  class="px-4 py-2 text-right text-xs font-medium text-subtle uppercase tracking-wider"
                  >Actions</th
                >
              </tr>
            </thead>
            <tbody class="divide-y divide-border">
              {#each apiKeys as apiKey (apiKey.id)}
                <tr class="hover:bg-surface-subtle transition-colors">
                  <td class="px-4 py-3 whitespace-nowrap">
                    <span
                      class="text-sm font-medium {apiKey.revoked_at
                        ? 'text-muted line-through'
                        : 'text-text'}"
                    >
                      {apiKey.name}
                    </span>
                  </td>
                  <td class="px-4 py-3 whitespace-nowrap">
                    <code
                      class="text-sm font-mono {apiKey.revoked_at
                        ? 'text-muted'
                        : 'text-text'}"
                    >
                      {apiKey.key_prefix}...
                    </code>
                  </td>
                  <td class="px-4 py-3">
                    <div class="flex flex-wrap gap-1">
                      {#each apiKey.scopes as scope}
                        <span
                          class="inline-flex items-center px-2 py-0.5 rounded-md text-xs font-medium bg-surface-muted text-muted"
                        >
                          {scope}
                        </span>
                      {/each}
                    </div>
                  </td>
                  <td class="px-4 py-3 whitespace-nowrap text-sm text-muted">
                    {apiKey.last_used_at
                      ? formatApiKeyDate(apiKey.last_used_at)
                      : "Never"}
                  </td>
                  <td class="px-4 py-3 whitespace-nowrap text-sm text-muted">
                    {formatApiKeyDate(apiKey.created_at)}
                  </td>
                  <td class="px-4 py-3 whitespace-nowrap text-right text-xs">
                    {#if apiKey.revoked_at}
                      <span
                        class="inline-flex items-center px-2 py-0.5 rounded-md text-xs font-medium bg-surface-muted text-muted"
                      >
                        Revoked
                      </span>
                    {:else}
                      <button
                        class="text-red-500 hover:text-red-700 transition-colors"
                        on:click={() => confirmRevokeApiKey(apiKey)}
                      >
                        Revoke
                      </button>
                    {/if}
                  </td>
                </tr>
              {:else}
                <tr>
                  <td
                    colspan="6"
                    class="px-4 py-8 text-center text-sm text-subtle"
                  >
                    No API keys configured
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </div>
  </div>

  <!-- Create API Key Modal -->
  <Modal
    open={createApiKeyModal}
    title="Create API Key"
    on:close={() => {
      if (!creatingApiKey) createApiKeyModal = false;
    }}
  >
    <form on:submit={handleCreateApiKey} class="space-y-4">
      <Input
        label="Name"
        bind:value={newApiKeyName}
        error={apiKeyCreateErrors.name}
        autocomplete="off"
        required
      />
      <fieldset>
        <legend class="block text-sm font-medium text-text mb-2">Scopes</legend>
        {#if apiKeyCreateErrors.scopes}
          <p class="text-sm text-red-500 mb-2">{apiKeyCreateErrors.scopes}</p>
        {/if}
        <div class="space-y-2">
          {#each allScopes as scope}
            <div class="flex items-center gap-2.5">
              <input
                type="checkbox"
                id="create-apikey-{scope}"
                checked={newApiKeyScopes.includes(scope)}
                on:change={() => toggleApiKeyScope(scope)}
                class="w-4 h-4 text-text border-border rounded focus:ring-accent/20"
              />
              <label for="create-apikey-{scope}" class="text-sm text-muted"
                >{scope}</label
              >
            </div>
          {/each}
        </div>
      </fieldset>
      <div class="flex gap-2 justify-end pt-2">
        <Button
          variant="secondary"
          on:click={() => {
            if (!creatingApiKey) createApiKeyModal = false;
          }}
          disabled={creatingApiKey}>Cancel</Button
        >
        <Button type="submit" loading={creatingApiKey}>Create</Button>
      </div>
    </form>
  </Modal>

  <!-- API Key Display Modal -->
  <Modal
    open={apiKeyModal}
    title="API Key"
    on:close={() => {
      if (
        apiKeyCopied ||
        confirm(
          "Are you sure? The API key will not be shown again after closing.",
        )
      ) {
        apiKeyModal = false;
      }
    }}
  >
    <div class="space-y-4">
      <div
        class="p-3 bg-yellow-50 border border-yellow-200 rounded-lg text-sm text-yellow-800"
      >
        Copy this API key now. It will not be shown again.
      </div>
      <div class="flex items-center gap-2">
        <code
          class="flex-1 p-3 bg-surface-muted rounded-lg text-sm text-text break-all font-mono"
        >
          {createdApiKey}
        </code>
        <Button variant="secondary" on:click={copyApiKey}>
          {apiKeyCopied ? "Copied" : "Copy"}
        </Button>
      </div>
      <div class="flex justify-end pt-2">
        <Button on:click={() => (apiKeyModal = false)}>Done</Button>
      </div>
    </div>
  </Modal>

  <!-- Revoke API Key Modal -->
  <Modal
    open={revokeApiKeyModal}
    title="Revoke API Key"
    on:close={() => {
      revokeApiKeyModal = false;
      apiKeyToRevoke = null;
    }}
  >
    <p class="text-sm text-muted mb-5">
      Are you sure you want to revoke "{apiKeyToRevoke?.name}"? This action
      cannot be undone. Any integrations using this key will immediately stop
      working.
    </p>
    <div class="flex gap-2 justify-end">
      <Button
        variant="secondary"
        on:click={() => {
          revokeApiKeyModal = false;
          apiKeyToRevoke = null;
        }}>Cancel</Button
      >
      <Button
        variant="danger"
        loading={revokingApiKey}
        on:click={handleRevokeApiKey}>Revoke</Button
      >
    </div>
  </Modal>
</div>
