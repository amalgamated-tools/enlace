<script lang="ts">
  import { onMount } from 'svelte';
  import { push, querystring } from 'svelte-spa-router';
  import { Button, Input } from '../lib/components';
  import { auth, isAuthenticated, toast } from '../lib/stores';
  import { api, getOIDCConfig, getOIDCLinkURL } from '../lib/api';

  let displayName = '';
  let currentPassword = '';
  let newPassword = '';
  let confirmPassword = '';

  let savingProfile = false;
  let savingPassword = false;
  let profileErrors: Record<string, string> = {};
  let passwordErrors: Record<string, string> = {};

  let oidcEnabled = false;
  let unlinkingOIDC = false;

  onMount(async () => {
    // Check for oidc=linked query param (from successful OIDC linking)
    const params = new URLSearchParams($querystring);
    if (params.get('oidc') === 'linked') {
      toast.success('SSO account linked successfully');
      // Remove query param from URL
      push('/settings');
    }

    // Fetch OIDC config
    try {
      const config = await getOIDCConfig();
      oidcEnabled = config.enabled;
    } catch {
      // OIDC not available
      oidcEnabled = false;
    }
  });

  function handleLinkOIDC() {
    window.location.href = getOIDCLinkURL();
  }

  async function handleUnlinkOIDC() {
    unlinkingOIDC = true;
    try {
      await api.delete('/me/oidc');
      toast.success('OIDC account unlinked');
      auth.init(); // Refresh user data
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to unlink OIDC';
      toast.error(message);
    } finally {
      unlinkingOIDC = false;
    }
  }

  $: if ($auth.initialized && !$isAuthenticated) {
    push('/login');
  }

  $: if ($auth.user) {
    displayName = $auth.user.display_name;
  }

  async function handleUpdateProfile(e: Event) {
    e.preventDefault();
    profileErrors = {};

    if (!displayName.trim()) {
      profileErrors = { ...profileErrors, displayName: 'Display name is required' };
      return;
    }

    savingProfile = true;
    try {
      const updated = await api.patch<{ display_name: string; email: string; id: string; is_admin: boolean }>('/me', {
        display_name: displayName.trim(),
      });
      auth.setUser({ ...updated, display_name: updated.display_name });
      toast.success('Profile updated');
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to update profile';
      toast.error(message);
    } finally {
      savingProfile = false;
    }
  }

  async function handleChangePassword(e: Event) {
    e.preventDefault();
    passwordErrors = {};

    if (!currentPassword) {
      passwordErrors = { ...passwordErrors, currentPassword: 'Current password is required' };
    }
    if (!newPassword) {
      passwordErrors = { ...passwordErrors, newPassword: 'New password is required' };
    } else if (newPassword.length < 8) {
      passwordErrors = { ...passwordErrors, newPassword: 'Password must be at least 8 characters' };
    }
    if (newPassword !== confirmPassword) {
      passwordErrors = { ...passwordErrors, confirmPassword: 'Passwords do not match' };
    }

    if (Object.keys(passwordErrors).length > 0) {
      return;
    }

    savingPassword = true;
    try {
      await api.post<void>('/me/password', {
        current_password: currentPassword,
        new_password: newPassword,
      });
      currentPassword = '';
      newPassword = '';
      confirmPassword = '';
      toast.success('Password changed');
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to change password';
      toast.error(message);
    } finally {
      savingPassword = false;
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
          value={$auth.user?.email || ''}
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
          {savingProfile ? 'Saving...' : 'Update Profile'}
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
          {savingPassword ? 'Changing...' : 'Change Password'}
        </Button>
      </form>
    </div>
  </div>

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
              {unlinkingOIDC ? 'Unlinking...' : 'Unlink SSO Account'}
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
