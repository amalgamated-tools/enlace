<script lang="ts">
  import { onMount } from "svelte";
  import { Button, Input, Modal } from ".";
  import { toast } from "../stores";
  import { apiKeysApi, ApiError, ALL_SCOPES } from "../api";
  import type { ApiKey, CreateApiKeyResponse } from "../api";

  const allScopes = ALL_SCOPES;
  let apiKeys: ApiKey[] = [];
  let loading = true;

  // Create modal
  let createModal = false;
  let creating = false;
  let newName = "";
  let newScopes: string[] = [];
  let createErrors: Record<string, string> = {};

  // Key display modal
  let keyModal = false;
  let createdKey = "";
  let keyCopied = false;

  // Revoke modal
  let revokeModal = false;
  let revoking = false;
  let keyToRevoke: ApiKey | null = null;

  onMount(() => {
    loadApiKeys();
  });

  async function loadApiKeys() {
    loading = true;
    try {
      apiKeys = await apiKeysApi.list();
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        return;
      }
      const message =
        err instanceof Error ? err.message : "Failed to load API keys";
      toast.error(message);
    } finally {
      loading = false;
    }
  }

  function openCreateModal() {
    newName = "";
    newScopes = [];
    createErrors = {};
    createModal = true;
  }

  function toggleScope(scope: string) {
    if (newScopes.includes(scope)) {
      newScopes = newScopes.filter((s) => s !== scope);
    } else {
      newScopes = [...newScopes, scope];
    }
  }

  async function handleCreate(e: Event) {
    e.preventDefault();
    createErrors = {};

    if (!newName.trim()) {
      createErrors = { ...createErrors, name: "Name is required" };
    }
    if (newScopes.length === 0) {
      createErrors = {
        ...createErrors,
        scopes: "At least one scope is required",
      };
    }

    if (Object.keys(createErrors).length > 0) {
      return;
    }

    creating = true;
    try {
      const result: CreateApiKeyResponse = await apiKeysApi.create({
        name: newName.trim(),
        scopes: newScopes,
      });
      const { key, ...apiKey } = result;
      createdKey = key;
      keyCopied = false;
      apiKeys = [...apiKeys, apiKey];
      createModal = false;
      keyModal = true;
      toast.success("API key created");
    } catch (err) {
      if (err instanceof ApiError && err.fields) {
        createErrors = err.fields;
      } else {
        const message =
          err instanceof Error ? err.message : "Failed to create API key";
        toast.error(message);
      }
    } finally {
      creating = false;
    }
  }

  async function copyKey() {
    try {
      await navigator.clipboard.writeText(createdKey);
      keyCopied = true;
      toast.success("API key copied to clipboard");
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Failed to copy API key to clipboard";
      toast.error(message);
    }
  }

  function confirmRevoke(apiKey: ApiKey) {
    keyToRevoke = apiKey;
    revokeModal = true;
  }

  async function handleRevoke() {
    if (!keyToRevoke) return;

    const revokeId = keyToRevoke.id;
    revoking = true;
    try {
      await apiKeysApi.revoke(revokeId);
      apiKeys = apiKeys.map((k) =>
        k.id === revokeId ? { ...k, revoked_at: new Date().toISOString() } : k,
      );
      revokeModal = false;
      keyToRevoke = null;
      toast.success("API key revoked");
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to revoke API key";
      toast.error(message);
    } finally {
      revoking = false;
    }
  }

  function formatDate(dateStr: string): string {
    return new Date(dateStr).toLocaleDateString();
  }
</script>

<div class="bg-surface rounded-xl border border-border mt-6">
  <div
    class="px-6 py-4 border-b border-border flex items-center justify-between"
  >
    <h3 class="text-sm font-semibold text-text">API Keys</h3>
    <Button size="sm" on:click={openCreateModal}>Create API Key</Button>
  </div>
  <div class="p-6">
    {#if loading}
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
                    ? formatDate(apiKey.last_used_at)
                    : "Never"}
                </td>
                <td class="px-4 py-3 whitespace-nowrap text-sm text-muted">
                  {formatDate(apiKey.created_at)}
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
                      on:click={() => confirmRevoke(apiKey)}
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

<!-- Create Modal -->
<Modal
  open={createModal}
  title="Create API Key"
  on:close={() => {
    if (!creating) createModal = false;
  }}
>
  <form on:submit={handleCreate} class="space-y-4">
    <Input
      label="Name"
      bind:value={newName}
      error={createErrors.name}
      autocomplete="off"
      required
    />
    <fieldset>
      <legend class="block text-sm font-medium text-text mb-2">Scopes</legend>
      {#if createErrors.scopes}
        <p class="text-sm text-red-500 mb-2">{createErrors.scopes}</p>
      {/if}
      <div class="space-y-2">
        {#each allScopes as scope}
          <div class="flex items-center gap-2.5">
            <input
              type="checkbox"
              id="create-apikey-{scope}"
              checked={newScopes.includes(scope)}
              on:change={() => toggleScope(scope)}
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
          if (!creating) createModal = false;
        }}
        disabled={creating}>Cancel</Button
      >
      <Button type="submit" loading={creating}>Create</Button>
    </div>
  </form>
</Modal>

<!-- Key Display Modal -->
<Modal
  open={keyModal}
  title="API Key"
  on:close={() => {
    if (
      keyCopied ||
      confirm(
        "Are you sure? The API key will not be shown again after closing.",
      )
    ) {
      keyModal = false;
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
        {createdKey}
      </code>
      <Button variant="secondary" on:click={copyKey}>
        {keyCopied ? "Copied" : "Copy"}
      </Button>
    </div>
    <div class="flex justify-end pt-2">
      <Button on:click={() => (keyModal = false)}>Done</Button>
    </div>
  </div>
</Modal>

<!-- Revoke Modal -->
<Modal
  open={revokeModal}
  title="Revoke API Key"
  on:close={() => {
    revokeModal = false;
    keyToRevoke = null;
  }}
>
  <p class="text-sm text-muted mb-5">
    Are you sure you want to revoke "{keyToRevoke?.name}"? This action cannot be
    undone. Any integrations using this key will immediately stop working.
  </p>
  <div class="flex gap-2 justify-end">
    <Button
      variant="secondary"
      on:click={() => {
        revokeModal = false;
        keyToRevoke = null;
      }}>Cancel</Button
    >
    <Button variant="danger" loading={revoking} on:click={handleRevoke}
      >Revoke</Button
    >
  </div>
</Modal>
