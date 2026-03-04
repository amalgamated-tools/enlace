<script lang="ts">
  import { toast } from "../stores";
  import { fly } from "svelte/transition";

  const icons: Record<string, string> = {
    success: "M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z",
    error:
      "M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z",
    info: "M11.25 11.25l.041-.02a.75.75 0 011.063.852l-.708 2.836a.75.75 0 001.063.853l.041-.021M21 12a9 9 0 11-18 0 9 9 0 0118 0zm-9-3.75h.008v.008H12V8.25z",
    warning:
      "M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z",
  };

  const colors: Record<string, string> = {
    success: "text-emerald-600",
    error: "text-red-600",
    info: "text-muted",
    warning: "text-amber-600",
  };

  const bgColors: Record<string, string> = {
    success: "bg-emerald-50 border-emerald-100",
    error: "bg-red-50 border-red-100",
    info: "bg-surface-subtle border-border",
    warning: "bg-amber-50 border-amber-100",
  };
</script>

<div class="fixed bottom-5 right-5 z-50 flex flex-col gap-2">
  {#each $toast as t (t.id)}
    <div
      class="flex items-start gap-3 pl-4 pr-3 py-3 rounded-xl border shadow-lg bg-surface max-w-sm {bgColors[
        t.type
      ]}"
      transition:fly={{ x: 100, duration: 200 }}
    >
      <svg
        class="w-5 h-5 flex-shrink-0 mt-0.5 {colors[t.type]}"
        fill="none"
        viewBox="0 0 24 24"
        stroke-width="1.5"
        stroke="currentColor"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          d={icons[t.type]}
        />
      </svg>
      <span class="text-sm text-text flex-1">{t.message}</span>
      <button
        on:click={() => toast.dismiss(t.id)}
        class="text-subtle hover:text-muted transition-colors p-0.5 rounded hover:bg-surface-muted flex-shrink-0"
        aria-label="Dismiss"
      >
        <svg
          class="w-3.5 h-3.5"
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
  {/each}
</div>
