<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import { fade, scale } from "svelte/transition";
  export let open = false;
  export let title = "";

  const dispatch = createEventDispatcher();

  function close() {
    dispatch("close");
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") close();
  }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if open}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center p-4"
    transition:fade={{ duration: 150 }}
  >
    <div
      class="absolute inset-0 bg-overlay/40 backdrop-blur-sm"
      on:click={close}
      on:keydown={(e) => e.key === "Enter" && close()}
      role="button"
      tabindex="-1"
      aria-label="Close modal"
    ></div>
    <div
      class="relative bg-surface rounded-xl shadow-xl max-w-md w-full max-h-[90vh] overflow-auto border border-border"
      transition:scale={{ duration: 150, start: 0.95 }}
    >
      <div
        class="flex items-center justify-between px-6 py-4 border-b border-border"
      >
        <h2 class="text-base font-semibold text-text">{title}</h2>
        <button
          on:click={close}
          class="text-subtle hover:text-muted transition-colors p-1 -mr-1 rounded-md hover:bg-surface-muted"
          aria-label="Close"
        >
          <svg
            class="w-4 h-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke-width="2"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              d="M6 18L18 6M6 6l12 12"
            />
          </svg>
        </button>
      </div>
      <div class="px-6 py-5">
        <slot />
      </div>
    </div>
  </div>
{/if}
