<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import type { FileInfo } from "../api";

  export let files: FileInfo[] = [];
  export let canDelete = false;

  const dispatch = createEventDispatcher<{ delete: string }>();

  function formatSize(bytes: number): string {
    if (bytes < 1024) return bytes + " B";
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + " KB";
    if (bytes < 1024 * 1024 * 1024)
      return (bytes / (1024 * 1024)).toFixed(1) + " MB";
    return (bytes / (1024 * 1024 * 1024)).toFixed(1) + " GB";
  }
</script>

<ul
  class="divide-y divide-border border border-border rounded-xl overflow-hidden"
>
  {#each files as file (file.id)}
    <li
      class="flex items-center justify-between px-4 py-3 hover:bg-surface-subtle transition-colors"
    >
      <div class="flex items-center gap-3 min-w-0">
        <div
          class="w-8 h-8 rounded-lg bg-surface-muted flex items-center justify-center flex-shrink-0"
        >
          <svg
            class="w-4 h-4 text-subtle"
            fill="none"
            viewBox="0 0 24 24"
            stroke-width="1.5"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              d="M19.5 14.25v-2.625a3.375 3.375 0 00-3.375-3.375h-1.5A1.125 1.125 0 0113.5 7.125v-1.5a3.375 3.375 0 00-3.375-3.375H8.25m2.25 0H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 00-9-9z"
            />
          </svg>
        </div>
        <div class="min-w-0">
          <p class="text-sm font-medium text-text truncate">{file.name}</p>
          <p class="text-xs text-subtle">{formatSize(file.size)}</p>
        </div>
      </div>
      {#if canDelete}
        <button
          class="text-xs text-subtle hover:text-red-500 transition-colors ml-3 flex-shrink-0"
          on:click={() => dispatch("delete", file.id)}
        >
          Remove
        </button>
      {/if}
    </li>
  {:else}
    <li class="px-4 py-8 text-center text-sm text-subtle">No files</li>
  {/each}
</ul>
