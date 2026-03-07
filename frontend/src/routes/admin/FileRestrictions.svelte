<script lang="ts">
  import { onMount } from "svelte";
  import { push } from "svelte-spa-router";
  import { Button, Input, Modal, AdminNav } from "../../lib/components";
  import { auth, isAuthenticated, isAdmin, toast } from "../../lib/stores";
  import { fileRestrictionsApi, ApiError } from "../../lib/api";

  let loading = true;
  let saving = false;
  let errors: Record<string, string> = {};

  // Form state
  let maxFileSizeMB = "";
  let blockedExtensions = "";

  // Reset modal
  let resetModal = false;
  let resetting = false;

  $: if ($auth.initialized && !$isAuthenticated) {
    push("/login");
  }

  $: if ($auth.initialized && $isAuthenticated && !$isAdmin) {
    toast.error("Access denied");
    push("/");
  }

  onMount(async () => {
    await loadConfig();
  });

  async function loadConfig() {
    if (!$isAdmin) return;

    loading = true;
    try {
      const config = await fileRestrictionsApi.get();
      maxFileSizeMB =
        config.max_file_size != null
          ? String(config.max_file_size / 1048576)
          : "";
      blockedExtensions =
        config.blocked_extensions && config.blocked_extensions.length > 0
          ? config.blocked_extensions.join(", ")
          : "";
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to load file restrictions";
      toast.error(message);
    } finally {
      loading = false;
    }
  }

  async function handleSave(e: Event) {
    e.preventDefault();
    errors = {};

    const payload: Record<string, unknown> = {};

    if (maxFileSizeMB.trim() !== "") {
      const mb = parseFloat(maxFileSizeMB.trim());
      if (isNaN(mb) || mb <= 0) {
        errors = { max_file_size: "Must be a positive number" };
        return;
      }
      payload.max_file_size = Math.round(mb * 1048576);
    }

    if (blockedExtensions.trim() !== "") {
      payload.blocked_extensions = blockedExtensions.trim();
    }

    // If no fields are set, treat this as a reset rather than sending an empty update
    if (Object.keys(payload).length === 0) {
      resetModal = true;
      return;
    }
    saving = true;
    try {
      const config = await fileRestrictionsApi.update(
        payload as { max_file_size?: number; blocked_extensions?: string },
      );
      maxFileSizeMB =
        config.max_file_size != null
          ? String(config.max_file_size / 1048576)
          : "";
      blockedExtensions =
        config.blocked_extensions && config.blocked_extensions.length > 0
          ? config.blocked_extensions.join(", ")
          : "";
      toast.success("File restrictions saved");
    } catch (err) {
      if (err instanceof ApiError && err.fields) {
        errors = err.fields;
      } else {
        const message =
          err instanceof Error
            ? err.message
            : "Failed to save file restrictions";
        toast.error(message);
      }
    } finally {
      saving = false;
    }
  }

  async function handleReset() {
    resetting = true;
    try {
      await fileRestrictionsApi.reset();
      maxFileSizeMB = "";
      blockedExtensions = "";
      resetModal = false;
      toast.success("File restrictions reset to defaults");
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Failed to reset file restrictions";
      toast.error(message);
    } finally {
      resetting = false;
    }
  }
</script>

<AdminNav />

{#if loading}
  <div class="text-center py-16">
    <p class="text-sm text-subtle">Loading...</p>
  </div>
{:else}
  <div class="bg-surface rounded-xl border border-border mb-6">
    <div class="px-6 py-4 border-b border-border">
      <h3 class="text-sm font-semibold text-text">File Restrictions</h3>
    </div>
    <div class="p-6">
      <form on:submit={handleSave} class="space-y-5">
        <Input
          label="Max File Size (MB)"
          type="number"
          bind:value={maxFileSizeMB}
          placeholder="100"
          error={errors.max_file_size}
        />
        <p class="text-xs text-muted -mt-3">
          Maximum file upload size. Leave empty to use the default (100 MB).
        </p>

        <Input
          label="Blocked Extensions"
          bind:value={blockedExtensions}
          placeholder=".exe, .bat, .sh"
          error={errors.blocked_extensions}
        />
        <p class="text-xs text-muted -mt-3">
          Comma-separated list of blocked file extensions.
        </p>

        <div class="flex items-center gap-3">
          <Button type="submit" loading={saving}>
            {saving ? "Saving..." : "Save"}
          </Button>
        </div>
      </form>
    </div>
  </div>

  <div class="bg-surface rounded-xl border border-border mb-6">
    <div class="px-6 py-4 border-b border-border">
      <h3 class="text-sm font-semibold text-text">Reset to Defaults</h3>
    </div>
    <div class="p-6">
      <p class="text-sm text-muted mb-4">
        Remove all file restriction overrides and revert to defaults.
      </p>
      <Button variant="danger" on:click={() => (resetModal = true)}>
        Reset to Defaults
      </Button>
    </div>
  </div>
{/if}

<Modal
  open={resetModal}
  title="Reset File Restrictions"
  on:close={() => (resetModal = false)}
>
  <p class="text-sm text-muted mb-5">
    Are you sure you want to reset file restrictions? All overrides will be
    removed and defaults will be restored.
  </p>
  <div class="flex gap-2 justify-end">
    <Button variant="secondary" on:click={() => (resetModal = false)}
      >Cancel</Button
    >
    <Button variant="danger" loading={resetting} on:click={handleReset}
      >Reset</Button
    >
  </div>
</Modal>
