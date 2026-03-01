<script lang="ts">
  import type { Share } from "../api";
  import { push } from "svelte-spa-router";

  export let share: Share;

  function formatDate(date: string): string {
    return new Date(date).toLocaleDateString("en-US", {
      month: "short",
      day: "numeric",
      year: "numeric",
    });
  }

  function handleClick() {
    push(`/shares/${share.id}`);
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      handleClick();
    }
  }
</script>

<div
  class="bg-white border border-slate-200 rounded-xl p-5 hover:border-slate-300 hover:shadow-sm transition-all duration-150 cursor-pointer"
  on:click={handleClick}
  on:keydown={handleKeydown}
  role="button"
  tabindex="0"
  aria-label="View share: {share.name}"
>
  <div class="flex justify-between items-start">
    <div class="min-w-0 flex-1">
      <h3 class="font-semibold text-slate-900">{share.name}</h3>
      {#if share.description}
        <p class="text-sm text-slate-500 mt-1 line-clamp-2">
          {share.description}
        </p>
      {/if}
    </div>
    <div class="flex items-center gap-1.5 ml-3 flex-shrink-0">
      {#if share.has_password}
        <span
          class="inline-flex items-center px-2 py-0.5 rounded-md text-xs font-medium bg-slate-100 text-slate-600"
          title="Password protected"
        >
          <svg
            class="w-3 h-3 mr-1"
            fill="none"
            viewBox="0 0 24 24"
            stroke-width="1.5"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              d="M16.5 10.5V6.75a4.5 4.5 0 10-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 002.25-2.25v-6.75a2.25 2.25 0 00-2.25-2.25H6.75a2.25 2.25 0 00-2.25 2.25v6.75a2.25 2.25 0 002.25 2.25z"
            />
          </svg>
          Protected
        </span>
      {/if}
      {#if share.is_reverse_share}
        <span
          class="inline-flex items-center px-2 py-0.5 rounded-md text-xs font-medium bg-blue-50 text-blue-600"
        >
          Upload
        </span>
      {/if}
    </div>
  </div>
  <div class="flex items-center gap-4 mt-3 text-xs text-slate-400">
    <span class="font-mono">/{share.slug}</span>
    <span>{share.view_count} view{share.view_count !== 1 ? "s" : ""}</span>
    <span
      >{share.download_count} download{share.download_count !== 1
        ? "s"
        : ""}</span
    >
    <span class="ml-auto">{formatDate(share.created_at)}</span>
  </div>
</div>
