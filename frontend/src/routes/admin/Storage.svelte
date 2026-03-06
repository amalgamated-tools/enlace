<script lang="ts">
  import { onMount } from "svelte";
  import { push, location } from "svelte-spa-router";
  import { Button, Input, Modal } from "../../lib/components";
  import { auth, isAuthenticated, isAdmin, toast } from "../../lib/stores";
  import { api } from "../../lib/api";

  interface StorageConfig {
    storage_type: string;
    storage_local_path?: string;
    s3_endpoint?: string;
    s3_bucket?: string;
    s3_access_key?: string;
    s3_secret_key_set: boolean;
    s3_region?: string;
    s3_path_prefix?: string;
  }

  let loading = true;
  let saving = false;
  let errors: Record<string, string> = {};

  // Form state
  let storageType = "local";
  let storageLocalPath = "";
  let s3Endpoint = "";
  let s3Bucket = "";
  let s3AccessKey = "";
  let s3SecretKey = "";
  let s3SecretKeySet = false;
  let s3Region = "";
  let s3PathPrefix = "";

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

  $: usersActive = $location === "/admin/users";
  $: storageActive = $location === "/admin/storage";

  onMount(async () => {
    await loadConfig();
  });

  async function loadConfig() {
    if (!$isAdmin) return;

    loading = true;
    try {
      const config = await api.get<StorageConfig>("/admin/storage");
      storageType = config.storage_type || "local";
      storageLocalPath = config.storage_local_path || "";
      s3Endpoint = config.s3_endpoint || "";
      s3Bucket = config.s3_bucket || "";
      s3AccessKey = config.s3_access_key || "";
      s3SecretKeySet = config.s3_secret_key_set;
      s3Region = config.s3_region || "";
      s3PathPrefix = config.s3_path_prefix || "";
      s3SecretKey = "";
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Failed to load storage configuration";
      toast.error(message);
    } finally {
      loading = false;
    }
  }

  async function handleSave(e: Event) {
    e.preventDefault();
    errors = {};

    const payload: Record<string, string> = {
      storage_type: storageType,
    };

    if (storageType === "local") {
      if (!storageLocalPath.trim()) {
        errors = {
          ...errors,
          storage_local_path: "Local path is required",
        };
      }
      payload.storage_local_path = storageLocalPath.trim();
    } else if (storageType === "s3") {
      if (!s3Bucket.trim()) {
        errors = { ...errors, s3_bucket: "Bucket is required" };
      }
      if (!s3AccessKey.trim()) {
        errors = { ...errors, s3_access_key: "Access key is required" };
      }
      if (!s3SecretKey.trim() && !s3SecretKeySet) {
        errors = { ...errors, s3_secret_key: "Secret key is required" };
      }

      payload.s3_bucket = s3Bucket.trim();
      payload.s3_access_key = s3AccessKey.trim();
      payload.s3_region = s3Region.trim();
      payload.s3_path_prefix = s3PathPrefix.trim();
      payload.s3_endpoint = s3Endpoint.trim();
      if (s3SecretKey.trim()) {
        payload.s3_secret_key = s3SecretKey.trim();
      }
    }

    if (Object.keys(errors).length > 0) {
      return;
    }

    saving = true;
    try {
      const config = await api.put<StorageConfig>("/admin/storage", payload);
      storageType = config.storage_type || "local";
      storageLocalPath = config.storage_local_path || "";
      s3Endpoint = config.s3_endpoint || "";
      s3Bucket = config.s3_bucket || "";
      s3AccessKey = config.s3_access_key || "";
      s3SecretKeySet = config.s3_secret_key_set;
      s3Region = config.s3_region || "";
      s3PathPrefix = config.s3_path_prefix || "";
      s3SecretKey = "";
      toast.success("Storage configuration saved");
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Failed to save storage configuration";
      toast.error(message);
    } finally {
      saving = false;
    }
  }

  async function handleReset() {
    resetting = true;
    try {
      await api.delete<void>("/admin/storage");
      storageType = "local";
      storageLocalPath = "";
      s3Endpoint = "";
      s3Bucket = "";
      s3AccessKey = "";
      s3SecretKey = "";
      s3SecretKeySet = false;
      s3Region = "";
      s3PathPrefix = "";
      resetModal = false;
      toast.success(
        "Storage configuration reset to environment variable defaults",
      );
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Failed to reset storage configuration";
      toast.error(message);
    } finally {
      resetting = false;
    }
  }
</script>

<div class="flex items-center gap-1 mb-6">
  <a
    href="#/admin/users"
    class="px-3 py-1.5 text-sm rounded-md transition-colors {usersActive
      ? 'text-text bg-surface-muted font-medium'
      : 'text-muted hover:text-text hover:bg-surface-subtle'}"
  >
    Users
  </a>
  <a
    href="#/admin/storage"
    class="px-3 py-1.5 text-sm rounded-md transition-colors {storageActive
      ? 'text-text bg-surface-muted font-medium'
      : 'text-muted hover:text-text hover:bg-surface-subtle'}"
  >
    Storage
  </a>
</div>

{#if loading}
  <div class="text-center py-16">
    <p class="text-sm text-subtle">Loading...</p>
  </div>
{:else}
  <div class="bg-surface rounded-xl border border-border mb-6">
    <div class="px-6 py-4 border-b border-border">
      <h3 class="text-sm font-semibold text-text">Storage Configuration</h3>
    </div>
    <div class="p-6">
      <form on:submit={handleSave} class="space-y-5">
        <fieldset>
          <legend class="block text-sm font-medium text-text mb-2"
            >Storage Type</legend
          >
          <div class="flex items-center gap-6">
            <label class="flex items-center gap-2 cursor-pointer">
              <input
                type="radio"
                bind:group={storageType}
                value="local"
                class="w-4 h-4 text-accent border-border focus:ring-accent/20"
              />
              <span class="text-sm text-text">Local</span>
            </label>
            <label class="flex items-center gap-2 cursor-pointer">
              <input
                type="radio"
                bind:group={storageType}
                value="s3"
                class="w-4 h-4 text-accent border-border focus:ring-accent/20"
              />
              <span class="text-sm text-text">S3</span>
            </label>
          </div>
        </fieldset>

        {#if storageType === "local"}
          <Input
            label="Local Path"
            bind:value={storageLocalPath}
            placeholder="./uploads"
            error={errors.storage_local_path}
            required
          />
        {:else if storageType === "s3"}
          <Input
            label="Endpoint"
            bind:value={s3Endpoint}
            placeholder="https://s3.amazonaws.com (optional for AWS)"
            error={errors.s3_endpoint}
          />
          <Input
            label="Bucket"
            bind:value={s3Bucket}
            placeholder="my-bucket"
            error={errors.s3_bucket}
            required
          />
          <div class="grid gap-5 sm:grid-cols-2">
            <Input
              label="Access Key"
              bind:value={s3AccessKey}
              error={errors.s3_access_key}
              autocomplete="off"
              required
            />
            <Input
              type="password"
              label="Secret Key"
              bind:value={s3SecretKey}
              placeholder={s3SecretKeySet ? "Leave blank to keep current" : ""}
              error={errors.s3_secret_key}
              autocomplete="off"
              required={!s3SecretKeySet}
            />
          </div>
          <div class="grid gap-5 sm:grid-cols-2">
            <Input
              label="Region"
              bind:value={s3Region}
              placeholder="us-east-1 (optional)"
              error={errors.s3_region}
            />
            <Input
              label="Path Prefix"
              bind:value={s3PathPrefix}
              placeholder="uploads/ (optional)"
              error={errors.s3_path_prefix}
            />
          </div>
        {/if}

        <Button type="submit" loading={saving}>
          {saving ? "Saving..." : "Save Configuration"}
        </Button>
      </form>
    </div>
  </div>

  <div class="bg-surface rounded-xl border border-border mb-6">
    <div class="px-6 py-4 border-b border-border">
      <h3 class="text-sm font-semibold text-text">Reset to Defaults</h3>
    </div>
    <div class="p-6">
      <p class="text-sm text-muted mb-4">
        Remove all database overrides and revert to environment variable
        configuration on the next restart.
      </p>
      <Button variant="danger" on:click={() => (resetModal = true)}>
        Reset to Defaults
      </Button>
    </div>
  </div>

  <p class="text-xs text-subtle">
    Changes take effect after application restart.
  </p>
{/if}

<Modal
  open={resetModal}
  title="Reset Storage Configuration"
  on:close={() => (resetModal = false)}
>
  <p class="text-sm text-muted mb-5">
    Are you sure you want to reset storage configuration? All database overrides
    will be removed and the application will use environment variable defaults
    on the next restart.
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
