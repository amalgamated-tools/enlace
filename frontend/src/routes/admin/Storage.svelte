<script lang="ts">
  import { onMount } from "svelte";
  import { push } from "svelte-spa-router";
  import { Button, Input, Modal, AdminNav } from "../../lib/components";
  import { auth, isAuthenticated, isAdmin, toast } from "../../lib/stores";
  import { api, ApiError } from "../../lib/api";

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

  // Form state — empty string means no DB override is set
  let storageType = "";
  let storageLocalPath = "";
  let s3Endpoint = "";
  let s3Bucket = "";
  let s3AccessKey = "";
  let s3SecretKey = "";
  let s3SecretKeySet = false;
  let s3AccessKeySet = false;
  let s3Region = "";
  let s3PathPrefix = "";

  // Reset modal
  let resetModal = false;
  let resetting = false;

  // Test connection
  let testing = false;

  $: hasOverrides = storageType !== "";

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

  function applyConfig(config: StorageConfig) {
    storageType = config.storage_type || "";
    storageLocalPath = config.storage_local_path || "";
    s3Endpoint = config.s3_endpoint || "";
    s3Bucket = config.s3_bucket || "";
    s3AccessKey = "";
    s3AccessKeySet = (config.s3_access_key || "").length > 0;
    s3SecretKeySet = config.s3_secret_key_set;
    s3Region = config.s3_region || "";
    s3PathPrefix = config.s3_path_prefix || "";
    s3SecretKey = "";
  }

  async function loadConfig() {
    if (!$isAdmin) return;

    loading = true;
    try {
      const config = await api.get<StorageConfig>("/admin/storage");
      applyConfig(config);
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

    if (!storageType) {
      errors = { storage_type: "Select a storage type to configure" };
      return;
    }

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
      if (!s3AccessKey.trim() && !s3AccessKeySet) {
        errors = { ...errors, s3_access_key: "Access key is required" };
      }
      if (!s3SecretKey.trim() && !s3SecretKeySet) {
        errors = { ...errors, s3_secret_key: "Secret key is required" };
      }

      payload.s3_bucket = s3Bucket.trim();
      payload.s3_region = s3Region.trim();
      payload.s3_path_prefix = s3PathPrefix.trim();
      payload.s3_endpoint = s3Endpoint.trim();
      if (s3AccessKey.trim()) {
        payload.s3_access_key = s3AccessKey.trim();
      }
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
      applyConfig(config);
      toast.success("Storage configuration saved");
    } catch (err) {
      if (err instanceof ApiError && err.fields) {
        errors = err.fields;
      } else {
        const message =
          err instanceof Error
            ? err.message
            : "Failed to save storage configuration";
        toast.error(message);
      }
    } finally {
      saving = false;
    }
  }

  async function handleReset() {
    resetting = true;
    try {
      await api.delete<void>("/admin/storage");
      storageType = "";
      storageLocalPath = "";
      s3Endpoint = "";
      s3Bucket = "";
      s3AccessKey = "";
      s3SecretKey = "";
      s3SecretKeySet = false;
      s3AccessKeySet = false;
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

  async function handleTestConnection() {
    testing = true;
    try {
      const payload: Record<string, string> = {};
      payload.s3_bucket = s3Bucket.trim();
      payload.s3_region = s3Region.trim();
      payload.s3_path_prefix = s3PathPrefix.trim();
      payload.s3_endpoint = s3Endpoint.trim();
      if (s3AccessKey.trim()) {
        payload.s3_access_key = s3AccessKey.trim();
      }
      if (s3SecretKey.trim()) {
        payload.s3_secret_key = s3SecretKey.trim();
      }
      await api.post<void>("/admin/storage/test", payload);
      toast.success("S3 connection successful");
    } catch (err) {
      if (err instanceof ApiError) {
        // Surface validation field errors similarly to handleSave
        const fields = err.fields as Record<string, unknown> | undefined;
        if (fields && Object.keys(fields).length > 0) {
          const details = Object.entries(fields)
            .map(([field, value]) => {
              if (Array.isArray(value)) {
                return `${field}: ${value.join(", ")}`;
              }
              return `${field}: ${String(value)}`;
            })
            .join("; ");
          toast.error(`Validation failed: ${details}`);
        } else {
          toast.error(err.message);
        }
      } else {
        const message =
          err instanceof Error ? err.message : "S3 connection test failed";
        toast.error(message);
      }
    } finally {
      testing = false;
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
      <h3 class="text-sm font-semibold text-text">Storage Configuration</h3>
    </div>
    <div class="p-6">
      {#if !hasOverrides}
        <div class="mb-5 rounded-lg bg-surface-subtle px-4 py-3">
          <p class="text-sm text-muted">
            No database overrides are configured. Storage is using environment
            variable defaults.
          </p>
        </div>
      {/if}

      <form on:submit={handleSave} class="space-y-5">
        <fieldset>
          <legend class="block text-sm font-medium text-text mb-2"
            >Storage Type</legend
          >
          {#if errors.storage_type}
            <p class="text-xs text-danger mb-2">{errors.storage_type}</p>
          {/if}
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
              type="password"
              label="Access Key"
              bind:value={s3AccessKey}
              placeholder={s3AccessKeySet ? "Leave blank to keep current" : ""}
              error={errors.s3_access_key}
              autocomplete="off"
              required={!s3AccessKeySet}
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

        {#if storageType}
          <div class="flex items-center gap-3">
            <Button type="submit" loading={saving}>
              {saving ? "Saving..." : "Save Configuration"}
            </Button>
            {#if storageType === "s3"}
              <Button
                type="button"
                variant="secondary"
                loading={testing}
                on:click={handleTestConnection}
              >
                {testing ? "Testing..." : "Test Connection"}
              </Button>
            {/if}
          </div>
        {/if}
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
