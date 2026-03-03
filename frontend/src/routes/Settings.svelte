<script lang="ts">
  import { onMount } from "svelte";
  import { push, querystring } from "svelte-spa-router";
  import { Button, Input } from "../lib/components";
  import { auth, isAuthenticated, toast } from "../lib/stores";
  import { api, getOIDCConfig, getOIDCLinkURL, totpApi } from "../lib/api";
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
      await api.post<void>("/me/password", {
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
  <h2 class="text-lg font-semibold text-slate-900 mb-6">Settings</h2>

  <div class="bg-white rounded-xl border border-slate-200 mb-6">
    <div class="px-6 py-4 border-b border-slate-100">
      <h3 class="text-sm font-semibold text-slate-900">Profile</h3>
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

  <div class="bg-white rounded-xl border border-slate-200">
    <div class="px-6 py-4 border-b border-slate-100">
      <h3 class="text-sm font-semibold text-slate-900">Change Password</h3>
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
    <div class="bg-white rounded-xl border border-slate-200 mt-6">
      <div class="px-6 py-4 border-b border-slate-100">
        <h3 class="text-sm font-semibold text-slate-900">
          Two-Factor Authentication
        </h3>
      </div>
      <div class="p-6">
        {#if !totpStatus.enabled}
          <p class="text-sm text-slate-600 mb-4">
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
            <span class="text-sm text-slate-600"
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

          <div class="mt-6 pt-6 border-t border-slate-100">
            <p class="text-sm text-slate-600 mb-3">
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
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50 px-4"
    >
      <div
        class="bg-white rounded-xl border border-slate-200 shadow-lg w-full max-w-md p-6"
      >
        {#if setupStep === "qr"}
          <h3 class="text-base font-semibold text-slate-900 mb-4">
            Set up authenticator
          </h3>
          <p class="text-sm text-slate-600 mb-4">
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
            <summary
              class="text-xs text-slate-500 cursor-pointer hover:text-slate-700"
            >
              Can't scan? Enter this key manually
            </summary>
            <code
              class="block mt-2 text-xs bg-slate-50 p-2 rounded break-all select-all"
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
          <h3 class="text-base font-semibold text-slate-900 mb-4">
            Verify setup
          </h3>
          <p class="text-sm text-slate-600 mb-4">
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
          <h3 class="text-base font-semibold text-slate-900 mb-4">
            Save your recovery codes
          </h3>
          <p class="text-sm text-slate-600 mb-4">
            Store these codes in a safe place. Each code can only be used once.
            You'll need them if you lose access to your authenticator app.
          </p>
          <div class="bg-slate-50 rounded-lg p-4 mb-4">
            <div class="grid grid-cols-2 gap-2">
              {#each setupRecoveryCodes as code}
                <code class="text-sm font-mono text-slate-700">{code}</code>
              {/each}
            </div>
          </div>
          <div class="flex justify-end">
            <Button on:click={handleCloseSetup}>Done</Button>
          </div>
        {/if}
      </div>
    </div>
  {/if}

  {#if showRegenerateModal}
    <div
      class="fixed inset-0 bg-black/50 flex items-center justify-center z-50 px-4"
    >
      <div
        class="bg-white rounded-xl border border-slate-200 shadow-lg w-full max-w-md p-6"
      >
        <h3 class="text-base font-semibold text-slate-900 mb-4">
          Regenerate recovery codes
        </h3>
        <p class="text-sm text-slate-600 mb-4">
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
    <div class="bg-white rounded-xl border border-slate-200 mt-6">
      <div class="px-6 py-4 border-b border-slate-100">
        <h3 class="text-sm font-semibold text-slate-900">Single Sign-On</h3>
      </div>
      <div class="p-6">
        {#if $auth.user?.oidc_linked}
          <p class="text-sm text-slate-600 mb-4">
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
            <p class="text-xs text-slate-400">
              Set a password before unlinking SSO to avoid being locked out.
            </p>
          {/if}
        {:else}
          <p class="text-sm text-slate-600 mb-4">
            Link your account to an external identity provider for easier login.
          </p>
          <Button variant="secondary" on:click={handleLinkOIDC}>
            Link SSO Account
          </Button>
        {/if}
      </div>
    </div>
  {/if}
</div>
