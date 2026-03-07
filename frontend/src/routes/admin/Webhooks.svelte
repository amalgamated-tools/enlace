<script lang="ts">
  import { onMount } from "svelte";
  import { push } from "svelte-spa-router";
  import { Button, Input, Modal, AdminNav } from "../../lib/components";
  import { auth, isAuthenticated, isAdmin, toast } from "../../lib/stores";
  import {
    webhooksApi,
    ApiError,
    type Webhook,
    type CreateWebhookResponse,
    type WebhookDelivery,
  } from "../../lib/api";

  const ALL_EVENTS = [
    "file.upload.completed",
    "share.viewed",
    "share.downloaded",
    "share.created",
  ];

  let webhooks: Webhook[] = [];
  let loading = true;

  // Create modal
  let createModal = false;
  let creating = false;
  let newName = "";
  let newUrl = "";
  let newEvents: string[] = [];
  let createErrors: Record<string, string> = {};

  // Secret modal (shown once after creation)
  let secretModal = false;
  let createdSecret = "";

  // Edit modal
  let editModal = false;
  let editing = false;
  let editWebhook: Webhook | null = null;
  let editName = "";
  let editUrl = "";
  let editEvents: string[] = [];
  let editErrors: Record<string, string> = {};

  // Delete modal
  let deleteModal = false;
  let deleting = false;
  let webhookToDelete: Webhook | null = null;

  // Deliveries
  let deliveriesWebhook: Webhook | null = null;
  let deliveries: WebhookDelivery[] = [];
  let loadingDeliveries = false;

  $: if ($auth.initialized && !$isAuthenticated) {
    push("/login");
  }

  $: if ($auth.initialized && $isAuthenticated && !$isAdmin) {
    toast.error("Access denied");
    push("/");
  }

  onMount(async () => {
    await loadWebhooks();
  });

  async function loadWebhooks() {
    if (!$isAdmin) return;

    loading = true;
    try {
      webhooks = await webhooksApi.list();
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to load webhooks";
      toast.error(message);
    } finally {
      loading = false;
    }
  }

  // Create
  function openCreateModal() {
    newName = "";
    newUrl = "";
    newEvents = [];
    createErrors = {};
    createModal = true;
  }

  function toggleCreateEvent(event: string) {
    if (newEvents.includes(event)) {
      newEvents = newEvents.filter((e) => e !== event);
    } else {
      newEvents = [...newEvents, event];
    }
  }

  async function handleCreate(e: Event) {
    e.preventDefault();
    createErrors = {};

    if (!newName.trim()) {
      createErrors = { ...createErrors, name: "Name is required" };
    }
    if (!newUrl.trim()) {
      createErrors = { ...createErrors, url: "URL is required" };
    }
    if (newEvents.length === 0) {
      createErrors = {
        ...createErrors,
        events: "At least one event is required",
      };
    }

    if (Object.keys(createErrors).length > 0) {
      return;
    }

    creating = true;
    try {
      const result: CreateWebhookResponse = await webhooksApi.create({
        name: newName.trim(),
        url: newUrl.trim(),
        events: newEvents,
      });
      // Separate secret from webhook data
      createdSecret = result.secret;
      const { secret: _, ...webhook } = result;
      webhooks = [...webhooks, webhook];
      createModal = false;
      secretModal = true;
      toast.success("Webhook created");
    } catch (err) {
      if (err instanceof ApiError && err.fields) {
        createErrors = err.fields;
      } else {
        const message =
          err instanceof Error ? err.message : "Failed to create webhook";
        toast.error(message);
      }
    } finally {
      creating = false;
    }
  }

  async function copySecret() {
    try {
      await navigator.clipboard.writeText(createdSecret);
      toast.success("Secret copied to clipboard");
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to copy secret to clipboard";
      toast.error(message);
    }
  }

  // Edit
  function openEditModal(webhook: Webhook) {
    editWebhook = webhook;
    editName = webhook.name;
    editUrl = webhook.url;
    editEvents = [...webhook.events];
    editErrors = {};
    editModal = true;
  }

  function toggleEditEvent(event: string) {
    if (editEvents.includes(event)) {
      editEvents = editEvents.filter((e) => e !== event);
    } else {
      editEvents = [...editEvents, event];
    }
  }

  async function handleEdit(e: Event) {
    e.preventDefault();
    if (!editWebhook) return;
    editErrors = {};

    if (!editName.trim()) {
      editErrors = { ...editErrors, name: "Name is required" };
    }
    if (!editUrl.trim()) {
      editErrors = { ...editErrors, url: "URL is required" };
    }
    if (editEvents.length === 0) {
      editErrors = {
        ...editErrors,
        events: "At least one event is required",
      };
    }

    if (Object.keys(editErrors).length > 0) {
      return;
    }

    editing = true;
    try {
      const updated = await webhooksApi.update(editWebhook.id, {
        name: editName.trim(),
        url: editUrl.trim(),
        events: editEvents,
      });
      webhooks = webhooks.map((w) => (w.id === updated.id ? updated : w));
      editModal = false;
      editWebhook = null;
      toast.success("Webhook updated");
    } catch (err) {
      if (err instanceof ApiError && err.fields) {
        editErrors = err.fields;
      } else {
        const message =
          err instanceof Error ? err.message : "Failed to update webhook";
        toast.error(message);
      }
    } finally {
      editing = false;
    }
  }

  // Delete
  function confirmDelete(webhook: Webhook) {
    webhookToDelete = webhook;
    deleteModal = true;
  }

  async function handleDelete() {
    if (!webhookToDelete) return;

    deleting = true;
    try {
      await webhooksApi.delete(webhookToDelete.id);
      webhooks = webhooks.filter((w) => w.id !== webhookToDelete!.id);
      if (deliveriesWebhook?.id === webhookToDelete.id) {
        deliveriesWebhook = null;
        deliveries = [];
      }
      deleteModal = false;
      webhookToDelete = null;
      toast.success("Webhook deleted");
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to delete webhook";
      toast.error(message);
    } finally {
      deleting = false;
    }
  }

  // Toggle enabled
  async function toggleEnabled(webhook: Webhook) {
    try {
      const updated = await webhooksApi.update(webhook.id, {
        enabled: !webhook.enabled,
      });
      webhooks = webhooks.map((w) => (w.id === updated.id ? updated : w));
      toast.success(
        `${webhook.name} ${updated.enabled ? "enabled" : "disabled"}`,
      );
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to update webhook";
      toast.error(message);
    }
  }

  // Deliveries
  async function showDeliveries(webhook: Webhook) {
    if (deliveriesWebhook?.id === webhook.id) {
      deliveriesWebhook = null;
      deliveries = [];
      return;
    }

    deliveriesWebhook = webhook;
    loadingDeliveries = true;
    try {
      deliveries = await webhooksApi.listDeliveries({
        subscription_id: webhook.id,
        limit: 50,
      });
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to load deliveries";
      toast.error(message);
    } finally {
      loadingDeliveries = false;
    }
  }

  function truncateUrl(url: string, max = 40): string {
    return url.length > max ? url.slice(0, max) + "..." : url;
  }

  function formatDate(dateStr: string): string {
    return new Date(dateStr).toLocaleDateString();
  }

  function formatTime(dateStr: string): string {
    return new Date(dateStr).toLocaleString();
  }

  function deliveryStatusClass(status: string): string {
    switch (status) {
      case "delivered":
        return "bg-green-100 text-green-800";
      case "failed":
        return "bg-red-100 text-red-800";
      default:
        return "bg-yellow-100 text-yellow-800";
    }
  }
</script>

<AdminNav />

<div class="flex items-center justify-between mb-6">
  <h2 class="text-lg font-semibold text-text">Webhooks</h2>
  <Button on:click={openCreateModal}>Create Webhook</Button>
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
            URL
          </th>
          <th
            class="px-6 py-3 text-left text-xs font-medium text-subtle uppercase tracking-wider"
          >
            Events
          </th>
          <th
            class="px-6 py-3 text-left text-xs font-medium text-subtle uppercase tracking-wider"
          >
            Status
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
        {#each webhooks as webhook (webhook.id)}
          <tr class="hover:bg-surface-subtle transition-colors">
            <td class="px-6 py-4 whitespace-nowrap">
              <span class="text-sm font-medium text-text">{webhook.name}</span>
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-sm text-muted">
              <span title={webhook.url}>{truncateUrl(webhook.url)}</span>
            </td>
            <td class="px-6 py-4">
              <div class="flex flex-wrap gap-1">
                {#each webhook.events as event}
                  <span
                    class="inline-flex items-center px-2 py-0.5 rounded-md text-xs font-medium bg-surface-muted text-muted"
                  >
                    {event}
                  </span>
                {/each}
              </div>
            </td>
            <td class="px-6 py-4 whitespace-nowrap">
              <button
                class="inline-flex items-center px-2 py-0.5 rounded-md text-xs font-medium cursor-pointer transition-colors {webhook.enabled
                  ? 'bg-green-100 text-green-800 hover:bg-green-200'
                  : 'bg-surface-muted text-muted hover:bg-surface-subtle'}"
                on:click={() => toggleEnabled(webhook)}
              >
                {webhook.enabled ? "Enabled" : "Disabled"}
              </button>
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-sm text-muted">
              {formatDate(webhook.created_at)}
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-right text-xs">
              <button
                class="text-muted hover:text-text transition-colors mr-3"
                on:click={() => openEditModal(webhook)}
              >
                Edit
              </button>
              <button
                class="text-muted hover:text-text transition-colors mr-3"
                on:click={() => showDeliveries(webhook)}
              >
                Deliveries
              </button>
              <button
                class="text-red-500 hover:text-red-700 transition-colors"
                on:click={() => confirmDelete(webhook)}
              >
                Delete
              </button>
            </td>
          </tr>
        {:else}
          <tr>
            <td colspan="6" class="px-6 py-8 text-center text-sm text-subtle">
              No webhooks configured
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>

  <!-- Deliveries section -->
  {#if deliveriesWebhook}
    <div
      class="mt-6 bg-surface rounded-xl border border-border overflow-hidden"
    >
      <div class="px-6 py-4 border-b border-border">
        <h3 class="text-sm font-semibold text-text">
          Deliveries for {deliveriesWebhook.name}
        </h3>
      </div>
      {#if loadingDeliveries}
        <div class="text-center py-8">
          <p class="text-sm text-subtle">Loading deliveries...</p>
        </div>
      {:else}
        <table class="min-w-full divide-y divide-border">
          <thead>
            <tr class="bg-surface-subtle">
              <th
                class="px-6 py-3 text-left text-xs font-medium text-subtle uppercase tracking-wider"
              >
                Event Type
              </th>
              <th
                class="px-6 py-3 text-left text-xs font-medium text-subtle uppercase tracking-wider"
              >
                Status
              </th>
              <th
                class="px-6 py-3 text-left text-xs font-medium text-subtle uppercase tracking-wider"
              >
                Status Code
              </th>
              <th
                class="px-6 py-3 text-left text-xs font-medium text-subtle uppercase tracking-wider"
              >
                Attempt
              </th>
              <th
                class="px-6 py-3 text-left text-xs font-medium text-subtle uppercase tracking-wider"
              >
                Duration
              </th>
              <th
                class="px-6 py-3 text-left text-xs font-medium text-subtle uppercase tracking-wider"
              >
                Time
              </th>
              <th
                class="px-6 py-3 text-left text-xs font-medium text-subtle uppercase tracking-wider"
              >
                Error
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-border">
            {#each deliveries as delivery (delivery.id)}
              <tr class="hover:bg-surface-subtle transition-colors">
                <td class="px-6 py-4 whitespace-nowrap text-sm text-text">
                  {delivery.event_type}
                </td>
                <td class="px-6 py-4 whitespace-nowrap">
                  <span
                    class="inline-flex items-center px-2 py-0.5 rounded-md text-xs font-medium {deliveryStatusClass(
                      delivery.status,
                    )}"
                  >
                    {delivery.status}
                  </span>
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-sm text-muted">
                  {delivery.status_code ?? "—"}
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-sm text-muted">
                  {delivery.attempt}
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-sm text-muted">
                  {delivery.duration_ms}ms
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-sm text-muted">
                  {formatTime(delivery.created_at)}
                </td>
                <td class="px-6 py-4 text-sm text-red-500">
                  {delivery.error || ""}
                </td>
              </tr>
            {:else}
              <tr>
                <td
                  colspan="7"
                  class="px-6 py-8 text-center text-sm text-subtle"
                >
                  No deliveries yet
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </div>
  {/if}
{/if}

<!-- Create Modal -->
<Modal
  open={createModal}
  title="Create Webhook"
  on:close={() => (createModal = false)}
>
  <form on:submit={handleCreate} class="space-y-4">
    <Input
      label="Name"
      bind:value={newName}
      error={createErrors.name}
      autocomplete="off"
      required
    />
    <Input
      label="URL"
      bind:value={newUrl}
      placeholder="https://example.com/webhook"
      error={createErrors.url}
      autocomplete="off"
      required
    />
    <fieldset>
      <legend class="block text-sm font-medium text-text mb-2">Events</legend>
      {#if createErrors.events}
        <p class="text-sm text-red-500 mb-2">{createErrors.events}</p>
      {/if}
      <div class="space-y-2">
        {#each ALL_EVENTS as event}
          <div class="flex items-center gap-2.5">
            <input
              type="checkbox"
              id="create-{event}"
              checked={newEvents.includes(event)}
              on:change={() => toggleCreateEvent(event)}
              class="w-4 h-4 text-text border-border rounded focus:ring-accent/20"
            />
            <label for="create-{event}" class="text-sm text-muted"
              >{event}</label
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

<!-- Secret Modal -->
<Modal
  open={secretModal}
  title="Webhook Secret"
  on:close={() => (secretModal = false)}
>
  <div class="space-y-4">
    <div
      class="p-3 bg-yellow-50 border border-yellow-200 rounded-lg text-sm text-yellow-800"
    >
      Copy this signing secret now. It will not be shown again.
    </div>
    <div class="flex items-center gap-2">
      <code
        class="flex-1 p-3 bg-surface-muted rounded-lg text-sm text-text break-all font-mono"
      >
        {createdSecret}
      </code>
      <Button variant="secondary" on:click={copySecret}>Copy</Button>
    </div>
    <div class="flex justify-end pt-2">
      <Button on:click={() => (secretModal = false)}>Done</Button>
    </div>
  </div>
</Modal>

<!-- Edit Modal -->
<Modal
  open={editModal}
  title="Edit Webhook"
  on:close={() => (editModal = false)}
>
  <form on:submit={handleEdit} class="space-y-4">
    <Input
      label="Name"
      bind:value={editName}
      error={editErrors.name}
      autocomplete="off"
      required
    />
    <Input
      label="URL"
      bind:value={editUrl}
      placeholder="https://example.com/webhook"
      error={editErrors.url}
      autocomplete="off"
      required
    />
    <fieldset>
      <legend class="block text-sm font-medium text-text mb-2">Events</legend>
      {#if editErrors.events}
        <p class="text-sm text-red-500 mb-2">{editErrors.events}</p>
      {/if}
      <div class="space-y-2">
        {#each ALL_EVENTS as event}
          <div class="flex items-center gap-2.5">
            <input
              type="checkbox"
              id="edit-{event}"
              checked={editEvents.includes(event)}
              on:change={() => toggleEditEvent(event)}
              class="w-4 h-4 text-text border-border rounded focus:ring-accent/20"
            />
            <label for="edit-{event}" class="text-sm text-muted">{event}</label>
          </div>
        {/each}
      </div>
    </fieldset>
    <div class="flex gap-2 justify-end pt-2">
      <Button variant="secondary" on:click={() => (editModal = false)}
        >Cancel</Button
      >
      <Button type="submit" loading={editing}>Save</Button>
    </div>
  </form>
</Modal>

<!-- Delete Modal -->
<Modal
  open={deleteModal}
  title="Delete Webhook"
  on:close={() => (deleteModal = false)}
>
  <p class="text-sm text-muted mb-5">
    Are you sure you want to delete "{webhookToDelete?.name}"? This action
    cannot be undone.
  </p>
  <div class="flex gap-2 justify-end">
    <Button variant="secondary" on:click={() => (deleteModal = false)}
      >Cancel</Button
    >
    <Button variant="danger" loading={deleting} on:click={handleDelete}
      >Delete</Button
    >
  </div>
</Modal>
