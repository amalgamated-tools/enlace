<script lang="ts">
  import { onMount } from "svelte";
  import { push } from "svelte-spa-router";
  import { Button, ShareCard } from "../lib/components";
  import { auth, isAuthenticated, toast } from "../lib/stores";
  import { sharesApi, type Share } from "../lib/api";

  let shares: Share[] = [];
  let loading = true;

  $: if ($auth.initialized && !$isAuthenticated) {
    push("/login");
  }

  onMount(async () => {
    if (!$isAuthenticated) return;

    try {
      shares = await sharesApi.list();
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to load shares";
      toast.error(message);
    } finally {
      loading = false;
    }
  });

  $: recentShares = shares.slice(0, 5);
  $: totalViews = shares.reduce((sum, s) => sum + s.view_count, 0);
  $: totalDownloads = shares.reduce((sum, s) => sum + s.download_count, 0);
</script>

{#if loading}
  <div class="text-center py-16">
    <p class="text-sm text-slate-400">Loading...</p>
  </div>
{:else}
  <div class="grid gap-5 sm:grid-cols-3 mb-10">
    <div class="bg-white rounded-xl border border-slate-200 p-5">
      <p class="text-xs font-medium text-slate-400 uppercase tracking-wider">
        Total Shares
      </p>
      <p class="text-2xl font-semibold text-slate-900 mt-1">{shares.length}</p>
    </div>
    <div class="bg-white rounded-xl border border-slate-200 p-5">
      <p class="text-xs font-medium text-slate-400 uppercase tracking-wider">
        Total Views
      </p>
      <p class="text-2xl font-semibold text-slate-900 mt-1">{totalViews}</p>
    </div>
    <div class="bg-white rounded-xl border border-slate-200 p-5">
      <p class="text-xs font-medium text-slate-400 uppercase tracking-wider">
        Total Downloads
      </p>
      <p class="text-2xl font-semibold text-slate-900 mt-1">{totalDownloads}</p>
    </div>
  </div>

  <div class="flex items-center justify-between mb-5">
    <h2 class="text-lg font-semibold text-slate-900">Recent Shares</h2>
    <Button size="sm" on:click={() => push("/shares/new")}>New Share</Button>
  </div>

  {#if recentShares.length === 0}
    <div class="bg-white rounded-xl border border-slate-200 p-12 text-center">
      <div class="max-w-xs mx-auto">
        <p class="text-sm text-slate-500 mb-4">
          No shares yet. Create your first share to get started.
        </p>
        <Button on:click={() => push("/shares/new")}
          >Create your first share</Button
        >
      </div>
    </div>
  {:else}
    <div class="space-y-3">
      {#each recentShares as share (share.id)}
        <ShareCard {share} />
      {/each}
    </div>
    {#if shares.length > 5}
      <div class="mt-5 text-center">
        <a
          href="#/shares"
          class="text-sm text-slate-500 hover:text-slate-700 font-medium"
          >View all shares</a
        >
      </div>
    {/if}
  {/if}
{/if}
