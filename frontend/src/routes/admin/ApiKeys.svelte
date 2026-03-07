<script lang="ts">
  import { onMount } from "svelte";
  import { push } from "svelte-spa-router";
  import { Button, Input, Modal, AdminNav } from "../../lib/components";
  import { auth, isAuthenticated, isAdmin, toast } from "../../lib/stores";
  import {
    apiKeysApi,
    ApiError,
    type ApiKey,
    type CreateApiKeyResponse,
  } from "../../lib/api";

  const allScopes = [
    "shares:read",
    "shares:write",
    "files:read",
    "files:write",
  ];

  let apiKeys: ApiKey[] = [];
  let loading = true;

  // Create modal
  let createModal = false;
  let creating = false;
  let newName = "";
  let newScopes: string[] = [];
  let createErrors: Record<string, string> = {};

  // Key modal (shown once after creation)
  let keyModal = false;
  let createdKey = "";
  let keyCopied = false;

  // Revoke modal
  let revokeModal = false;
  let revoking = false;
  let keyToRevoke: ApiKey | null = null;

  $: if ($auth.initialized && !$isAuthenticated) {
    push("/login");
  }

  $: if ($auth.initialized && $isAuthenticated && !$isAdmin) {
    toast.error("Access denied");
    push("/");
  }

  onMount(async () => {
    await loadApiKeys();
  });

  async function loadApiKeys() {
    if (!$isAdmin) return;

    loading = true;
    try {
      apiKeys = await apiKeysApi.list();
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to load API keys";
      toast.error(message);
    } finally {
      loading = false;
    }
  }

  // Create
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
      // Separate key from api key data
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

  // Revoke
  function confirmRevoke(apiKey: ApiKey) {
    keyToRevoke = apiKey;
    revokeModal = true;
  }

  async function handleRevoke() {
    if (!keyToRevoke) return;

    revoking = true;
    try {
      await apiKeysApi.revoke(keyToRevoke.id);
      apiKeys = apiKeys.map((k) =>
        k.id === keyToRevoke!.id
          ? { ...k, revoked_at: new Date().toISOString() }
          : k,
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

<AdminNav />

<div class="flex items-center justify-between mb-6">
  <h2 class="text-lg font-semibold text-text">API Keys</h2>
  <Button on:click={openCreateModal}>Create API Key</Button>
</div>

{#if loading}
  <div class="text-center py-16">
    <p class="text-sm text-subtle">Loading...</p>
  </div>
{:else}
  <div class="bg-surface rounded-xl border border-border overflow-hidden">
    <table class="min-w-full divide-y divide-border">
      <thead>
        <tr class="bg-surface-subtle">
          <th
            class="px-6 py-3 text-left text-xs font-medium text-subtle uppercase tracking-wider"
          >
            Name
          </th>
          <th
            class="px-6 py-3 text-left text-xs font-medium text-subtle uppercase tracking-wider"
          >
            Key Prefix
          </th>
          <th
            class="px-6 py-3 text-left text-xs font-medium text-subtle uppercase tracking-wider"
          >
            Scopes
          </th>
          <th
            class="px-6 py-3 text-left text-xs font-medium text-subtle uppercase tracking-wider"
          >
            Last Used
          </th>
          <th
            class="px-6 py-3 text-left text-xs font-medium text-subtle uppercase tracking-wider"
          >
            Created
          </th>
          <th
            class="px-6 py-3 text-right text-xs font-medium text-subtle uppercase tracking-wider"
          >
            Actions
          </th>
        </tr>
      </thead>
      <tbody class="divide-y divide-border">
        {#each apiKeys as apiKey (apiKey.id)}
          <tr class="hover:bg-surface-subtle transition-colors">
            <td class="px-6 py-4 whitespace-nowrap">
              <span
                class="text-sm font-medium {apiKey.revoked_at
                  ? 'text-muted line-through'
                  : 'text-text'}"
              >
                {apiKey.name}
              </span>
            </td>
            <td class="px-6 py-4 whitespace-nowrap">
              <code
                class="text-sm font-mono {apiKey.revoked_at
                  ? 'text-muted'
                  : 'text-text'}"
              >
                {apiKey.key_prefix}...
              </code>
            </td>
            <td class="px-6 py-4">
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
            <td class="px-6 py-4 whitespace-nowrap text-sm text-muted">
              {apiKey.last_used_at ? formatDate(apiKey.last_used_at) : "Never"}
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-sm text-muted">
              {formatDate(apiKey.created_at)}
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-right text-xs">
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
            <td colspan="6" class="px-6 py-8 text-center text-sm text-subtle">
              No API keys configured
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

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
              id="create-{scope}"
              checked={newScopes.includes(scope)}
              on:change={() => toggleScope(scope)}
              class="w-4 h-4 text-text border-border rounded focus:ring-accent/20"
            />
            <label for="create-{scope}" class="text-sm text-muted"
              >{scope}</label
            >
          </div>
        {/each}
      </div>
    </fieldset>
    <div class="flex gap-2 justify-end pt-2">
      <Button variant="secondary" on:click={() => (createModal = false)}
        >Cancel</Button
      >
      <Button type="submit" loading={creating}>Create</Button>
    </div>
  </form>
</Modal>

<!-- Key Modal -->
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
  on:close={() => (revokeModal = false)}
>
  <p class="text-sm text-muted mb-5">
    Are you sure you want to revoke "{keyToRevoke?.name}"? This action cannot be
    undone. Any integrations using this key will immediately stop working.
  </p>
  <div class="flex gap-2 justify-end">
    <Button variant="secondary" on:click={() => (revokeModal = false)}
      >Cancel</Button
    >
    <Button variant="danger" loading={revoking} on:click={handleRevoke}
      >Revoke</Button
    >
  </div>
</Modal>
