<script lang="ts">
  import { push } from "svelte-spa-router";
  import { Button, Input, SettingsNav } from "../../lib/components";
  import {
    auth,
    isAuthenticated,
    setThemePreference,
    themePreference,
    toast,
  } from "../../lib/stores";
  import { api } from "../../lib/api";

  let displayName = "";
  let savingProfile = false;
  let profileErrors: Record<string, string> = {};

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
</script>

<div>
  <h2 class="text-lg font-semibold text-text mb-6">Settings</h2>
  <SettingsNav />

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

  <div class="bg-surface rounded-xl border border-border">
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
</div>
