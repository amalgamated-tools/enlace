<script lang="ts">
  import { createEventDispatcher } from "svelte";

  export let multiple = true;
  export let disabled = false;
  export let accept = "*";

  const dispatch = createEventDispatcher<{ files: File[] }>();

  let dragover = false;
  let fileInput: HTMLInputElement;

  function handleFiles(files: FileList | null) {
    if (files && files.length > 0) {
      dispatch("files", Array.from(files));
    }
  }

  function handleDrop(e: DragEvent) {
    e.preventDefault();
    dragover = false;
    handleFiles(e.dataTransfer?.files ?? null);
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      fileInput.click();
    }
  }
</script>

<div
  class="border-2 border-dashed rounded-xl p-10 text-center transition-all duration-150 {dragover
    ? 'border-slate-400 bg-slate-50'
    : 'border-slate-200 hover:border-slate-300'}"
  on:dragover|preventDefault={() => (dragover = true)}
  on:dragleave={() => (dragover = false)}
  on:drop={handleDrop}
  on:keydown={handleKeydown}
  role="button"
  tabindex="0"
  aria-label="File upload area"
>
  <input
    bind:this={fileInput}
    type="file"
    {multiple}
    {accept}
    {disabled}
    class="hidden"
    on:change={(e) => handleFiles(e.currentTarget.files)}
  />
  <div class="flex flex-col items-center gap-3">
    <div
      class="w-10 h-10 rounded-lg bg-slate-100 flex items-center justify-center"
    >
      <svg
        class="w-5 h-5 text-slate-400"
        fill="none"
        viewBox="0 0 24 24"
        stroke-width="1.5"
        stroke="currentColor"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5m-13.5-9L12 3m0 0l4.5 4.5M12 3v13.5"
        />
      </svg>
    </div>
    <div>
      <p class="text-sm text-slate-600">
        Drop files here or
        <button
          type="button"
          {disabled}
          class="text-slate-900 font-medium underline underline-offset-2 hover:text-slate-700 disabled:opacity-40 disabled:cursor-not-allowed"
          on:click={() => fileInput.click()}
        >
          browse
        </button>
      </p>
    </div>
  </div>
</div>
