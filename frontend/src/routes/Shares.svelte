<script lang="ts">
  import { onMount } from "svelte";
  import { push } from "svelte-spa-router";
  import { Button, ShareCard, Modal } from "../lib/components";
  import { auth, isAuthenticated, toast } from "../lib/stores";
  import { sharesApi, type Share } from "../lib/api";

  let shares: Share[] = [];
  let loading = true;
  let deleteModal = false;
  let shareToDelete: Share | null = null;
  let deleting = false;

  $: if ($auth.initialized && !$isAuthenticated) {
    push("/login");
  }

  onMount(async () => {
    await loadShares();
  });

  async function loadShares() {
    if (!$isAuthenticated) return;

    loading = true;
    try {
      shares = await sharesApi.list();
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to load shares";
      toast.error(message);
    } finally {
      loading = false;
    }
  }

  function confirmDelete(share: Share) {
    shareToDelete = share;
    deleteModal = true;
  }

  async function handleDelete() {
    if (!shareToDelete) return;

    deleting = true;
    try {
      await sharesApi.delete(shareToDelete.id);
      shares = shares.filter((s) => s.id !== shareToDelete!.id);
      toast.success("Share deleted");
      deleteModal = false;
      shareToDelete = null;
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to delete share";
      toast.error(message);
    } finally {
      deleting = false;
    }
  }
</script>

<div class="flex items-center justify-between mb-6">
  <h2 class="text-lg font-semibold text-text">Your Shares</h2>
  <Button on:click={() => push("/shares/new")}>New Share</Button>
</div>

{#if loading}
  <div class="text-center py-16">
    <p class="text-sm text-subtle">Loading...</p>
  </div>
{:else if shares.length === 0}
  <div class="bg-surface rounded-xl border border-border p-12 text-center">
    <div class="max-w-xs mx-auto">
      <p class="text-sm text-muted mb-4">
        You haven't created any shares yet
      </p>
      <Button on:click={() => push("/shares/new")}
        >Create your first share</Button
      >
    </div>
  </div>
{:else}
  <div class="space-y-3">
    {#each shares as share (share.id)}
      <div class="relative group">
        <ShareCard {share} />
        <button
          class="absolute top-4 right-4 text-xs text-subtle hover:text-red-500 opacity-0 group-hover:opacity-100 transition-all"
          on:click|stopPropagation={() => confirmDelete(share)}
          aria-label="Delete share"
        >
          Delete
        </button>
      </div>
    {/each}
  </div>
{/if}

<Modal
  open={deleteModal}
  title="Delete Share"
  on:close={() => (deleteModal = false)}
>
  <p class="text-sm text-muted mb-5">
    Are you sure you want to delete "{shareToDelete?.name}"? This action cannot
    be undone.
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
